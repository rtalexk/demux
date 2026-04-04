package cmd

import (
    "fmt"
    "os"
    "strings"

    "github.com/mattn/go-isatty"
    "github.com/rtalexk/demux/internal/config"
    "github.com/rtalexk/demux/internal/format"
    "github.com/rtalexk/demux/internal/git"
    demuxlog "github.com/rtalexk/demux/internal/log"
    "github.com/rtalexk/demux/internal/tmux"
    "github.com/spf13/cobra"
)

var (
    windowsSession string
    windowsGit     bool
)

var windowsCmd = &cobra.Command{
    Use:   "windows",
    Short: "List windows in a session",
    RunE:  runWindows,
}

func init() {
    rootCmd.AddCommand(windowsCmd)
    windowsCmd.Flags().StringVar(&windowsSession, "session", "", "Session name (required)")
    windowsCmd.MarkFlagRequired("session")
    windowsCmd.Flags().BoolVar(&windowsGit, "git", false, "Include git column")
}

type windowRow struct {
    session, window, name, panes, procs, alerts, gitCol string
    includeGit                                          bool
}

func (r windowRow) Fields() []string {
    base := []string{r.session, r.window, r.name, r.panes, r.procs, r.alerts}
    if r.includeGit {
        return append(base, r.gitCol)
    }
    return base
}

func resolveWindowGitCol(wPanes []tmux.Pane, primaryCWD string, cfg config.Config) string {
    winCWD := tmux.PrimaryPaneCWD(wPanes)
    if winCWD == "" || git.IsDescendant(winCWD, primaryCWD) || winCWD == primaryCWD {
        return "—"
    }
    info, err := git.Fetch(winCWD, cfg.Git.TimeoutMs)
    if err != nil {
        return cfg.Git.ErrorDisplay
    }
    return "↪ " + info.Branch + " " + git.Indicators(info)
}

func buildWindowRows(windows map[int][]tmux.Pane, alertsByWindow map[string]int, primaryCWD string, cfg config.Config, session string, includeGit bool) []format.Row {
    var rows []format.Row
    for wi, wPanes := range windows {
        gitCol := "—"
        if includeGit {
            gitCol = resolveWindowGitCol(wPanes, primaryCWD, cfg)
        }
        rows = append(rows, windowRow{
            session:    session,
            window:     fmt.Sprint(wi),
            name:       wPanes[0].WindowName,
            panes:      fmt.Sprint(len(wPanes)),
            procs:      "—",
            alerts:     fmt.Sprint(alertsByWindow[fmt.Sprint(wi)]),
            gitCol:     gitCol,
            includeGit: includeGit,
        })
    }
    return rows
}

func runWindows(cmd *cobra.Command, _ []string) error {
    cfg := loadConfig()

    panes, err := tmux.ListPanes()
    if err != nil {
        return fmt.Errorf("tmux not available: %w", err)
    }
    grouped := tmux.GroupBySessions(panes)
    windows, ok := grouped[windowsSession]
    if !ok {
        return fmt.Errorf("session %q not found", windowsSession)
    }

    primaryCWD := tmux.PrimaryPaneCWD(windows[0])

    database, err := openDB()
    if err != nil {
        return err
    }
    defer database.Close()
    alerts, err := database.AlertList()
    if err != nil {
        demuxlog.Warn("failed to list alerts", "err", err)
    }
    alertsByWindow := map[string]int{}
    for _, a := range alerts {
        parts := strings.SplitN(a.Target, ":", 2)
        if len(parts) == 2 && parts[0] == windowsSession {
            alertsByWindow[parts[1]]++
        }
    }

    headers := []string{"SESSION", "WINDOW", "NAME", "PANES", "PROCS", "ALERTS"}
    if windowsGit {
        headers = append(headers, "GIT")
    }

    rows := buildWindowRows(windows, alertsByWindow, primaryCWD, cfg, windowsSession, windowsGit)

    isTTYVal := isatty.IsTerminal(os.Stdout.Fd())
    fmt.Println(format.Render(resolveFormat(cmd), headers, rows, isTTYVal))
    return nil
}
