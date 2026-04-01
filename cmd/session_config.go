package cmd

import (
    "fmt"
    "path/filepath"
    "strings"

    "github.com/rtalex/demux/internal/config"
    "github.com/rtalex/demux/internal/session"
    "github.com/spf13/cobra"
)

var sessionCmd = &cobra.Command{
    Use:   "session",
    Short: "Manage session config entries",
}

// --- add ---

var (
    sessionAddName     string
    sessionAddAlias    string
    sessionAddPath     string
    sessionAddWorktree bool
    sessionAddLabels   string
    sessionAddIcon     string
    sessionAddWindows  string
    sessionAddPrivate  bool
)

var sessionAddCmd = &cobra.Command{
    Use:   "add",
    Short: "Add a session entry to sessions.toml (or private.toml with --private)",
    RunE:  runSessionAdd,
}

func init() {
    sessionAddCmd.Flags().StringVar(&sessionAddName, "name", "", "Session name (required)")
    sessionAddCmd.Flags().StringVar(&sessionAddAlias, "alias", "", "Session alias (required)")
    sessionAddCmd.Flags().StringVar(&sessionAddPath, "path", "", "Path to session directory (required)")
    sessionAddCmd.Flags().BoolVar(&sessionAddWorktree, "worktree", false, "Mark as a worktree session")
    sessionAddCmd.Flags().StringVar(&sessionAddLabels, "labels", "", "Comma-separated labels (e.g. work,rust)")
    sessionAddCmd.Flags().StringVar(&sessionAddIcon, "icon", "", "Icon glyph")
    sessionAddCmd.Flags().StringVar(&sessionAddWindows, "windows", "", "Comma-separated window template ids (e.g. editor,terminal)")
    sessionAddCmd.Flags().BoolVar(&sessionAddPrivate, "private", false, "Write to private.toml instead of sessions.toml")

    _ = sessionAddCmd.MarkFlagRequired("name")
    _ = sessionAddCmd.MarkFlagRequired("alias")
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
        Alias:    sessionAddAlias,
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

// --- remove ---

var (
    sessionRemoveName    string
    sessionRemoveAlias   string
    sessionRemovePrivate bool
)

var sessionRemoveCmd = &cobra.Command{
    Use:   "remove",
    Short: "Remove a session entry from sessions.toml (or private.toml with --private)",
    RunE:  runSessionRemove,
}

func init() {
    sessionRemoveCmd.Flags().StringVar(&sessionRemoveName, "name", "", "Session name (required)")
    sessionRemoveCmd.Flags().StringVar(&sessionRemoveAlias, "alias", "", "Session alias (required)")
    sessionRemoveCmd.Flags().BoolVar(&sessionRemovePrivate, "private", false, "Target private.toml instead of sessions.toml")

    _ = sessionRemoveCmd.MarkFlagRequired("name")
    _ = sessionRemoveCmd.MarkFlagRequired("alias")

    sessionCmd.AddCommand(sessionRemoveCmd)
}

func runSessionRemove(_ *cobra.Command, _ []string) error {
    path, err := sessionFilePath(sessionRemovePrivate)
    if err != nil {
        return err
    }

    dn := session.ConfigEntry{Name: sessionRemoveName, Alias: sessionRemoveAlias}.DisplayName()

    if err := session.RemoveEntry(path, sessionRemoveName, sessionRemoveAlias); err != nil {
        return err
    }

    fmt.Printf("Removed session %q from %s\n", dn, filepath.Base(path))
    return nil
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
