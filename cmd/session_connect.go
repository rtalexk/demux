package cmd

import (
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

	sessions, err := loadMergedSessions()
	if err != nil {
		return err
	}

	if name == "" {
		name, err = pickSessionFuzzy(sessions)
		if err != nil {
			return err
		}
	}

	target, err := resolveConnectTarget(sessions, name)
	if err != nil {
		return err
	}

	return dispatchConnect(target)
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

func loadMergedSessions() ([]session.Session, error) {
	panes, err := tmux.ListPanes()
	if err != nil {
		return nil, fmt.Errorf("tmux not available: %w", err)
	}
	cfgPath, err := config.DefaultPath()
	if err != nil {
		return nil, fmt.Errorf("config dir: %w", err)
	}
	scfg, err := session.LoadConfigSessions(filepath.Dir(cfgPath))
	if err != nil {
		return nil, err
	}
	return session.Merge(panes, scfg.Entries), nil
}

func pickSessionFuzzy(sessions []session.Session) (string, error) {
	if _, err := exec.LookPath("fzf"); err != nil {
		return "", fmt.Errorf("fzf not installed")
	}
	if len(sessions) == 0 {
		return "", fmt.Errorf("no sessions to pick from")
	}

	var sb strings.Builder
	for _, s := range sessions {
		sb.WriteString(s.DisplayName)
		sb.WriteByte('\n')
	}

	cmd := exec.Command("fzf", "--prompt", "session> ", "--height", "40%")
	cmd.Stdin = strings.NewReader(sb.String())
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 130 {
			os.Exit(1)
		}
		return "", fmt.Errorf("fzf: %w", err)
	}
	pick := strings.TrimSpace(string(out))
	if pick == "" {
		return "", fmt.Errorf("no selection")
	}
	return pick, nil
}

func dispatchConnect(target *session.Session) error {
	if target.IsLive {
		return tmux.Connect(target.DisplayName)
	}
	if target.IsConfig {
		ce := target.Config
		if err := tmux.NewSessionDetached(ce.Name, ce.Path); err != nil {
			return err
		}
		cfgPath, err := config.DefaultPath()
		if err != nil {
			return fmt.Errorf("config dir: %w", err)
		}
		scfg, err := session.LoadConfigSessions(filepath.Dir(cfgPath))
		if err != nil {
			return err
		}
		specs, unknown := session.ResolveWindowSpecs(ce.Windows, scfg.WindowTemplates)
		for _, id := range unknown {
			fmt.Fprintf(os.Stderr, "demux: unknown window_template id %q (skipped)\n", id)
		}
		if len(specs) > 0 {
			if err := tmux.CreateSessionWindows(ce.Name, ce.Path, specs); err != nil {
				return fmt.Errorf("window setup: %w", err)
			}
		}
		return tmux.Connect(ce.Name)
	}
	return fmt.Errorf("session %q is neither live nor in config", target.DisplayName)
}
