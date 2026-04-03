package cmd

import (
	"fmt"
	"os"
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

	var labels []string
	if sessionAddLabels != "" {
		for _, l := range strings.Split(sessionAddLabels, ",") {
			if t := strings.TrimSpace(l); t != "" {
				labels = append(labels, t)
			}
		}
	}

	var windows []string
	if sessionAddWindows != "" {
		for _, w := range strings.Split(sessionAddWindows, ",") {
			if t := strings.TrimSpace(w); t != "" {
				windows = append(windows, t)
			}
		}
	}

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
	Short:   "Remove a session entry from sessions.toml (or private.toml with --private)",
	RunE:    runSessionRemove,
}

func init() {
	sessionRemoveCmd.Flags().StringVar(&sessionRemoveName, "name", "", "Session name (required)")
	sessionRemoveCmd.Flags().BoolVar(&sessionRemovePrivate, "private", false, "Target private.toml instead of sessions.toml")

	_ = sessionRemoveCmd.MarkFlagRequired("name")

	sessionCmd.AddCommand(sessionRemoveCmd)
}

func runSessionRemove(_ *cobra.Command, _ []string) error {
	path, err := sessionFilePath(sessionRemovePrivate)
	if err != nil {
		return err
	}

	if err := session.RemoveEntry(path, sessionRemoveName); err != nil {
		return err
	}

	fmt.Printf("Removed session %q from %s\n", sessionRemoveName, filepath.Base(path))
	return nil
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

	var ids []string
	for _, id := range strings.Split(sessionCreateWindowsIDs, ",") {
		if t := strings.TrimSpace(id); t != "" {
			ids = append(ids, t)
		}
	}

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
