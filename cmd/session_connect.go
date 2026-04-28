package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rtalexk/demux/internal/config"
	"github.com/rtalexk/demux/internal/session"
	"github.com/rtalexk/demux/internal/tmux"
	"github.com/spf13/cobra"
)

var errFzfAborted = errors.New("fzf: aborted")

// SilentExitError signals termination without printing an error.
// Used for user-initiated exits like fzf cancellation.
type SilentExitError struct {
	code int
}

func (e *SilentExitError) Error() string {
	return ""
}

var sessionConnectFuzzy bool

var sessionConnectCmd = &cobra.Command{
	Use:   "connect [name]",
	Short: "Attach or switch to a session (auto-creates from config when not live)",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runSessionConnect,
}

func init() {
	sessionConnectCmd.Flags().BoolVar(&sessionConnectFuzzy, "fuzzy", false, "Pick session interactively via fzf")
	sessionCmd.AddCommand(sessionConnectCmd)
}

func runSessionConnect(_ *cobra.Command, args []string) error {
	name := ""
	if len(args) == 1 {
		name = args[0]
	}
	if err := validateConnectArgs(name, sessionConnectFuzzy); err != nil {
		return err
	}

	sessions, scfg, err := loadMergedSessions()
	if err != nil {
		return err
	}

	if name == "" {
		name, err = pickSessionFuzzy(sessions)
		if errors.Is(err, errFzfAborted) {
			return &SilentExitError{code: 1}
		}
		if err != nil {
			return err
		}
	}

	target, err := resolveConnectTarget(sessions, name)
	if err != nil {
		return err
	}

	return dispatchConnect(target, scfg)
}

func validateConnectArgs(name string, fuzzy bool) error {
	if name == "" && !fuzzy {
		return fmt.Errorf("provide a session name or use --fuzzy")
	}
	if name != "" && fuzzy {
		return fmt.Errorf("--fuzzy and a positional name are mutually exclusive")
	}
	return nil
}

func resolveConnectTarget(sessions []session.Session, name string) (*session.Session, error) {
	for i := range sessions {
		if sessions[i].DisplayName == name {
			return &sessions[i], nil
		}
	}
	return nil, fmt.Errorf("session %q not found", name)
}

func loadMergedSessions() ([]session.Session, session.SessionsConfig, error) {
	panes, _ := tmux.ListPanes()  // Soft fallback: zero live sessions on error
	cfgPath, err := config.DefaultPath()
	if err != nil {
		return nil, session.SessionsConfig{}, fmt.Errorf("config dir: %w", err)
	}
	scfg, err := session.LoadConfigSessions(filepath.Dir(cfgPath))
	if err != nil {
		return nil, session.SessionsConfig{}, err
	}
	return session.Merge(panes, scfg.Entries), scfg, nil
}

func pickSessionFuzzy(sessions []session.Session) (string, error) {
	if _, err := exec.LookPath("fzf"); err != nil {
		return "", fmt.Errorf("fzf not installed")
	}
	if len(sessions) == 0 {
		return "", fmt.Errorf("no sessions to pick from")
	}

	names := make([]string, len(sessions))
	for i, s := range sessions {
		names[i] = s.DisplayName
	}

	cmd := exec.Command("fzf", "--prompt", "session> ", "--height", "40%")
	cmd.Stdin = strings.NewReader(strings.Join(names, "\n") + "\n")
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 130 {
			return "", errFzfAborted
		}
		return "", fmt.Errorf("fzf: %w", err)
	}
	pick := strings.TrimSpace(string(out))
	if pick == "" {
		return "", fmt.Errorf("no selection")
	}
	return pick, nil
}

func dispatchConnect(target *session.Session, scfg session.SessionsConfig) error {
	if target.IsLive {
		return tmux.Connect(target.DisplayName)
	}
	if target.IsConfig && target.Config != nil {
		ce := target.Config
		_, unknown := session.ResolveWindowSpecs(ce.Windows, scfg.WindowTemplates)
		for _, id := range unknown {
			fmt.Fprintf(os.Stderr, "demux: unknown window_template id %q (skipped)\n", id)
		}
		return session.LaunchAndConnectConfigSession(ce.Name, ce.Path, ce.Windows, scfg.WindowTemplates)
	}
	return fmt.Errorf("session %q is neither live nor in config", target.DisplayName)
}
