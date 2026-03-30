package cmd

import (
    "fmt"
    "os"
    "strings"
    "sync"

    "github.com/mattn/go-isatty"
    "github.com/rtalex/demux/internal/config"
    "github.com/rtalex/demux/internal/db"
    "github.com/rtalex/demux/internal/format"
    "github.com/rtalex/demux/internal/git"
    "github.com/rtalex/demux/internal/proc"
    "github.com/rtalex/demux/internal/tmux"
    "github.com/spf13/cobra"
)

var (
    sessionsGit     bool
    sessionsGitOnly bool
)

var sessionsCmd = &cobra.Command{
    Use:   "sessions",
    Short: "List all tmux sessions",
    RunE:  runSessions,
}

func init() {
    rootCmd.AddCommand(sessionsCmd)
    sessionsCmd.Flags().BoolVar(&sessionsGit, "git", false, "Include git columns")
    sessionsCmd.Flags().BoolVar(&sessionsGitOnly, "git-only", false, "Show only session + git columns")
}

type sessionRow struct {
    session, windows, procs, alerts, status string
    branch, dirty, ahead, behind            string
    includeGit, gitOnly                     bool
}

func (r sessionRow) Fields() []string {
    base := []string{r.session, r.windows, r.procs, r.alerts, r.status}
    gitCols := []string{r.branch, r.dirty, r.ahead, r.behind}
    if r.gitOnly {
        return append([]string{r.session}, gitCols...)
    }
    if r.includeGit {
        return append(base, gitCols...)
    }
    return base
}

type sessionGitWork struct {
    sessionName string
    primaryCWD  string
}

const gitConcurrencyCap = 8

// fetchGitForSessions fetches git info for each work item in parallel,
// capped at gitConcurrencyCap concurrent goroutines.
// Returns a map of sessionName -> git.Info (entry absent on error).
func fetchGitForSessions(work []sessionGitWork, timeoutMs int) map[string]git.Info {
    results := make(map[string]git.Info, len(work))
    if len(work) == 0 {
        return results
    }
    var mu sync.Mutex
    var wg sync.WaitGroup
    sem := make(chan struct{}, gitConcurrencyCap)
    for _, w := range work {
        wg.Add(1)
        w := w
        go func() {
            defer wg.Done()
            sem <- struct{}{}
            defer func() { <-sem }()
            info, err := git.Fetch(w.primaryCWD, timeoutMs)
            if err == nil {
                mu.Lock()
                results[w.sessionName] = info
                mu.Unlock()
            }
        }()
    }
    wg.Wait()
    return results
}

func runSessions(cmd *cobra.Command, _ []string) error {
    cfg := loadConfig()

    panes, err := tmux.ListPanes()
    if err != nil {
        return fmt.Errorf("tmux not available: %w", err)
    }
    grouped := tmux.GroupBySessions(panes)

    database, err := openDB()
    if err != nil {
        return err
    }
    defer database.Close()
    alerts, _ := database.AlertList()

    alertsBySession := map[string][]db.Alert{}
    for _, a := range alerts {
        parts := strings.SplitN(a.Target, ":", 2)
        if len(parts) > 0 {
            alertsBySession[parts[0]] = append(alertsBySession[parts[0]], a)
        }
    }

    headers := []string{"SESSION", "WINDOWS", "PROCS", "ALERTS", "STATUS"}
    if sessionsGitOnly {
        headers = []string{"SESSION", "BRANCH", "DIRTY", "AHEAD", "BEHIND"}
    } else if sessionsGit {
        headers = append(headers, "BRANCH", "DIRTY", "AHEAD", "BEHIND")
    }

    allProcs, _ := proc.Snapshot()
    cwdByPID, _ := proc.CWDAll()

    sessionProcCount := map[string]int{}
    for sessionName, windows := range grouped {
        primaryCWD := primaryCWDForSession(windows)
        if primaryCWD == "" {
            continue
        }
        for _, p := range allProcs {
            cwd, ok := cwdByPID[p.PID]
            if !ok {
                continue
            }
            if cwd == primaryCWD || git.IsDescendant(cwd, primaryCWD) {
                sessionProcCount[sessionName]++
            }
        }
    }

    // Pre-fetch git info in parallel for all non-ignored sessions.
    var gitWork []sessionGitWork
    if sessionsGit || sessionsGitOnly {
        for sessionName, windows := range grouped {
            if isIgnored(cfg, sessionName) {
                continue
            }
            if cwd := primaryCWDForSession(windows); cwd != "" {
                gitWork = append(gitWork, sessionGitWork{sessionName, cwd})
            }
        }
    }
    gitResults := fetchGitForSessions(gitWork, cfg.Git.TimeoutMs)

    var rows []format.Row
    for sessionName, windows := range grouped {
        if isIgnored(cfg, sessionName) {
            continue
        }
        sessionAlerts := alertsBySession[sessionName]
        status := "ok"
        for _, a := range sessionAlerts {
            switch a.Level {
            case "error":
                status = "error"
            case "warn":
                if status != "error" {
                    status = "warn"
                }
            }
        }

        row := sessionRow{
            session:    sessionName,
            windows:    fmt.Sprint(len(windows)),
            procs:      fmt.Sprint(sessionProcCount[sessionName]),
            alerts:     fmt.Sprint(len(sessionAlerts)),
            status:     status,
            includeGit: sessionsGit,
            gitOnly:    sessionsGitOnly,
        }

        if sessionsGit || sessionsGitOnly {
            primaryCWD := primaryCWDForSession(windows)
            if primaryCWD == "" {
                row.branch = cfg.Git.FallbackDisplay
                row.dirty = "—"
                row.ahead = "—"
                row.behind = "—"
            } else if info, ok := gitResults[sessionName]; ok {
                row.branch = info.Branch
                if info.Dirty {
                    row.dirty = "yes"
                } else {
                    row.dirty = "no"
                }
                row.ahead = fmt.Sprint(info.Ahead)
                row.behind = fmt.Sprint(info.Behind)
            } else {
                row.branch = cfg.Git.ErrorDisplay
                row.dirty = "—"
                row.ahead = "—"
                row.behind = "—"
            }
        }

        rows = append(rows, row)
    }

    isTTYVal := isatty.IsTerminal(os.Stdout.Fd())
    fmt.Println(format.Render(resolveFormat(cmd), headers, rows, isTTYVal))
    return nil
}

func isIgnored(cfg config.Config, name string) bool {
    for _, s := range cfg.IgnoredSessions {
        if s == name {
            return true
        }
    }
    return false
}

func primaryCWDForSession(windows map[int][]tmux.Pane) string {
    panes, ok := windows[0]
    if !ok || len(panes) == 0 {
        return ""
    }
    for _, p := range panes {
        if p.PaneIndex == 0 {
            return p.CWD
        }
    }
    return panes[0].CWD
}
