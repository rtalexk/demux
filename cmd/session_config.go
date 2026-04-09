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

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage session config entries",
}

// --- add ---

var (
	sessionAddName     string
	sessionAddGroup    string
	sessionAddPath     string
	sessionAddWorktree bool
	sessionAddLabels   string
	sessionAddIcon     string
	sessionAddWindows  string
	sessionAddPrivate  bool
)

var sessionAddCmd = &cobra.Command{
	Use:   "config-add",
	Short: "Add a session entry to sessions.toml (or private.toml with --private)",
	RunE:  runSessionAdd,
}

func init() {
	sessionAddCmd.Flags().StringVar(&sessionAddName, "name", "", "Session name (required)")
	sessionAddCmd.Flags().StringVar(&sessionAddGroup, "group", "", "Session group")
	sessionAddCmd.Flags().StringVar(&sessionAddPath, "path", "", "Path to session directory (required)")
	sessionAddCmd.Flags().BoolVar(&sessionAddWorktree, "worktree", false, "Mark as a worktree session")
	sessionAddCmd.Flags().StringVar(&sessionAddLabels, "labels", "", "Comma-separated labels (e.g. work,rust)")
	sessionAddCmd.Flags().StringVar(&sessionAddIcon, "icon", "", "Icon glyph")
	sessionAddCmd.Flags().StringVar(&sessionAddWindows, "windows", "", "Comma-separated window template ids (e.g. editor,terminal)")
	sessionAddCmd.Flags().BoolVar(&sessionAddPrivate, "private", false, "Write to private.toml instead of sessions.toml")

	_ = sessionAddCmd.MarkFlagRequired("name")
	_ = sessionAddCmd.MarkFlagRequired("path")

	sessionCmd.AddCommand(sessionAddCmd)
}

func runSessionAdd(_ *cobra.Command, _ []string) error {
	path, err := sessionFilePath(sessionAddPrivate)
	if err != nil {
		return err
	}

	labels := splitCSV(sessionAddLabels)
	windows := splitCSV(sessionAddWindows)

	e := session.ConfigEntry{
		Name:     sessionAddName,
		Group:    sessionAddGroup,
		Path:     sessionAddPath,
		Worktree: sessionAddWorktree,
		Labels:   labels,
		Icon:     sessionAddIcon,
		Windows:  windows,
	}

	if err := session.AppendEntry(path, e); err != nil {
		return err
	}

	fmt.Printf("Added session %q to %s\n", e.DisplayName(), filepath.Base(path))
	return nil
}

// --- get-config ---

var (
	sessionGetConfigName string
)

var sessionGetConfigCmd = &cobra.Command{
	Use:   "config-get",
	Short: "Print the config block for a session",
	RunE:  runSessionGetConfig,
}

func init() {
	sessionGetConfigCmd.Flags().StringVar(&sessionGetConfigName, "name", "", "Session name (required)")

	_ = sessionGetConfigCmd.MarkFlagRequired("name")

	sessionCmd.AddCommand(sessionGetConfigCmd)
}

func runSessionGetConfig(_ *cobra.Command, _ []string) error {
	cfgPath, err := config.DefaultPath()
	if err != nil {
		return fmt.Errorf("config dir: %w", err)
	}
	cfg, err := session.LoadConfigSessions(filepath.Dir(cfgPath))
	if err != nil {
		return err
	}

	for _, e := range cfg.Entries {
		if e.DisplayName() == sessionGetConfigName {
			fmt.Print(session.FormatBlock(e))
			return nil
		}
	}
	return fmt.Errorf("session %q not found", sessionGetConfigName)
}

// --- remove ---

var (
	sessionRemoveName    string
	sessionRemovePrivate bool
)

var sessionRemoveCmd = &cobra.Command{
	Use:     "config-remove",
	Aliases: []string{"config-rm"},
	Short:   "Remove a session entry (searches both files; --private targets only private.toml)",
	RunE:    runSessionRemove,
}

func init() {
	sessionRemoveCmd.Flags().StringVar(&sessionRemoveName, "name", "", "Session name (required)")
	sessionRemoveCmd.Flags().BoolVar(&sessionRemovePrivate, "private", false, "Target private.toml only")

	_ = sessionRemoveCmd.MarkFlagRequired("name")

	sessionCmd.AddCommand(sessionRemoveCmd)
}

func runSessionRemove(_ *cobra.Command, _ []string) error {
	candidates := sessionFileCandidates(sessionRemovePrivate)
	for _, private := range candidates {
		path, err := sessionFilePath(private)
		if err != nil {
			return err
		}
		err = session.RemoveEntry(path, sessionRemoveName)
		if err == nil {
			fmt.Printf("Removed session %q from %s\n", sessionRemoveName, filepath.Base(path))
			return nil
		}
		if !session.IsNotFound(err) || sessionRemovePrivate {
			return err
		}
	}
	return fmt.Errorf("session %q not found in sessions.toml or private.toml", sessionRemoveName)
}

// --- remove (full: tmux + config + db) ---

var (
	sessionRemoveFullName    string
	sessionRemoveFullPrivate bool
)

var sessionRemoveFullCmd = &cobra.Command{
	Use:     "remove",
	Aliases: []string{"rm"},
	Short:   "Kill tmux session, remove config entry, and clear DB states",
	RunE:    runSessionRemoveFull,
}

func init() {
	sessionRemoveFullCmd.Flags().StringVar(&sessionRemoveFullName, "name", "", "Session name (required)")
	sessionRemoveFullCmd.Flags().BoolVar(&sessionRemoveFullPrivate, "private", false, "Target private.toml only for config step")

	_ = sessionRemoveFullCmd.MarkFlagRequired("name")

	sessionCmd.AddCommand(sessionRemoveFullCmd)
}

func runSessionRemoveFull(_ *cobra.Command, _ []string) error {
	name := sessionRemoveFullName

	tmuxOut := killTmuxSession(name)
	configOut := removeSessionConfig(name, sessionRemoveFullPrivate)
	dbOut := clearSessionStates(name)
	todosOut := clearSessionTodos(name)

	fmt.Printf("tmux:   %s\n", tmuxOut)
	fmt.Printf("config: %s\n", configOut)
	fmt.Printf("db:     %s\n", dbOut)
	fmt.Printf("todos:  %s\n", todosOut)
	return nil
}

func clearSessionTodos(name string) string {
	database, err := openDB()
	if err != nil {
		return fmt.Sprintf("error opening db: %v", err)
	}
	defer database.Close()

	if err := database.TodoDeleteSession(name); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "cleared"
}

func killTmuxSession(name string) string {
	err := exec.Command("tmux", "kill-session", "-t", name).Run()
	if err != nil {
		return "not found (skipped)"
	}
	return "killed"
}

func removeSessionConfig(name string, private bool) string {
	candidates := sessionFileCandidates(private)
	for _, isPrivate := range candidates {
		path, err := sessionFilePath(isPrivate)
		if err != nil {
			continue
		}
		err = session.RemoveEntry(path, name)
		if err == nil {
			return fmt.Sprintf("removed from %s", filepath.Base(path))
		}
		if !session.IsNotFound(err) || private {
			return fmt.Sprintf("error: %v", err)
		}
	}
	return "not found in sessions.toml or private.toml (skipped)"
}

func clearSessionStates(name string) string {
	database, err := openDB()
	if err != nil {
		return fmt.Sprintf("error opening db: %v", err)
	}
	defer database.Close()

	n, err := database.StateDeleteBySession(name)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return fmt.Sprintf("%d state(s) cleared", n)
}

// --- create-windows ---

var (
	sessionCreateWindowsSession string
	sessionCreateWindowsIDs     string
)

var sessionCreateWindowsCmd = &cobra.Command{
	Use:   "create-windows",
	Short: "Create tmux windows for a session using window template ids",
	RunE:  runSessionCreateWindows,
}

func init() {
	sessionCreateWindowsCmd.Flags().StringVar(&sessionCreateWindowsSession, "session", "", "Tmux session name (required)")
	sessionCreateWindowsCmd.Flags().StringVar(&sessionCreateWindowsIDs, "windows", "", "Comma-separated window template ids (required)")

	_ = sessionCreateWindowsCmd.MarkFlagRequired("session")
	_ = sessionCreateWindowsCmd.MarkFlagRequired("windows")

	sessionCmd.AddCommand(sessionCreateWindowsCmd)
}

func runSessionCreateWindows(_ *cobra.Command, _ []string) error {
	cfgPath, err := config.DefaultPath()
	if err != nil {
		return fmt.Errorf("config dir: %w", err)
	}
	cfg, err := session.LoadConfigSessions(filepath.Dir(cfgPath))
	if err != nil {
		return err
	}

	var sessionPath string
	for _, e := range cfg.Entries {
		if e.DisplayName() == sessionCreateWindowsSession {
			sessionPath = e.Path
			break
		}
	}

	ids := splitCSV(sessionCreateWindowsIDs)

	specs, unknown := session.ResolveWindowSpecs(ids, cfg.WindowTemplates)
	for _, id := range unknown {
		fmt.Fprintf(os.Stderr, "demux: unknown window_template id %q (skipped)\n", id)
	}
	if len(specs) == 0 {
		return fmt.Errorf("no valid window templates resolved from %q", sessionCreateWindowsIDs)
	}

	return tmux.CreateSessionWindows(sessionCreateWindowsSession, sessionPath, specs)
}

// --- helpers ---

// splitCSV splits a comma-separated string, trimming whitespace and dropping empty tokens.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, item := range strings.Split(s, ",") {
		if t := strings.TrimSpace(item); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// sessionFileCandidates returns the ordered list of private flags to search when
// looking up or removing a session config entry. With privateOnly=true only
// private.toml is searched; otherwise sessions.toml is tried first.
func sessionFileCandidates(privateOnly bool) []bool {
	if privateOnly {
		return []bool{true}
	}
	return []bool{false, true}
}

func sessionFilePath(private bool) (string, error) {
	cfgPath, err := config.DefaultPath()
	if err != nil {
		return "", fmt.Errorf("config dir: %w", err)
	}
	name := "sessions.toml"
	if private {
		name = "private.toml"
	}
	return filepath.Join(filepath.Dir(cfgPath), name), nil
}

func init() {
	rootCmd.AddCommand(sessionCmd)
}
