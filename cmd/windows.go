package cmd

import (
    "fmt"
    "os"
    "strings"

    "github.com/mattn/go-isatty"
    "github.com/rtalex/demux/internal/format"
    "github.com/rtalex/demux/internal/git"
    demuxlog "github.com/rtalex/demux/internal/log"
    "github.com/rtalex/demux/internal/tmux"
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
    includeGit                                           bool
}

func (r windowRow) Fields() []string {
    base := []string{r.session, r.window, r.name, r.panes, r.procs, r.alerts}
    if r.includeGit {
        return append(base, r.gitCol)
    }
    return base
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

    primaryCWD := primaryCWDForSession(windows)

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

    var rows []format.Row
    for wi, wPanes := range windows {
        gitCol := "—"
        if windowsGit {
            winCWD := windowCWD(wPanes)
            if winCWD != "" && !git.IsDescendant(winCWD, primaryCWD) && winCWD != primaryCWD {
                info, err := git.Fetch(winCWD, cfg.Git.TimeoutMs)
                if err != nil {
                    gitCol = cfg.Git.ErrorDisplay
                } else {
                    gitCol = "↪ " + info.Branch + " " + gitIndicators(info)
                }
            }
        }

        rows = append(rows, windowRow{
            session:    windowsSession,
            window:     fmt.Sprint(wi),
            name:       wPanes[0].WindowName,
            panes:      fmt.Sprint(len(wPanes)),
            procs:      "—",
            alerts:     fmt.Sprint(alertsByWindow[fmt.Sprint(wi)]),
            gitCol:     gitCol,
            includeGit: windowsGit,
        })
    }

    isTTYVal := isatty.IsTerminal(os.Stdout.Fd())
    fmt.Println(format.Render(resolveFormat(cmd), headers, rows, isTTYVal))
    return nil
}

func windowCWD(panes []tmux.Pane) string {
    for _, p := range panes {
        if p.PaneIndex == 0 {
            return p.CWD
        }
    }
    if len(panes) > 0 {
        return panes[0].CWD
    }
    return ""
}

func gitIndicators(info git.Info) string {
    var parts []string
    if info.Ahead > 0 {
        parts = append(parts, fmt.Sprintf("↑%d", info.Ahead))
    }
    if info.Behind > 0 {
        parts = append(parts, fmt.Sprintf("↓%d", info.Behind))
    }
    if info.Dirty {
        parts = append(parts, "*")
    }
    return strings.Join(parts, " ")
}
