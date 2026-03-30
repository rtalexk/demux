package cmd

import (
    "fmt"
    "os"
    "sync"
    "time"

    "github.com/mattn/go-isatty"
    "github.com/rtalex/demux/internal/format"
    "github.com/rtalex/demux/internal/git"
    "github.com/rtalex/demux/internal/proc"
    "github.com/rtalex/demux/internal/tmux"
    "github.com/spf13/cobra"
)

var (
    procsSession string
    procsWindow  string
    procsGit     bool
)

var procsCmd = &cobra.Command{
    Use:   "procs",
    Short: "List processes across sessions",
    RunE:  runProcs,
}

func init() {
    rootCmd.AddCommand(procsCmd)
    procsCmd.Flags().StringVar(&procsSession, "session", "", "Filter to session")
    procsCmd.Flags().StringVar(&procsWindow, "window", "", "Filter to window (session:index)")
    procsCmd.Flags().BoolVar(&procsGit, "git", false, "Include git column on pane headers")
}

type procRow struct {
    session, window, pane, process, pid, cpu, mem, port, up, cwd string
    gitCol                                                         string
    includeGit                                                     bool
}

func (r procRow) Fields() []string {
    base := []string{r.session, r.window, r.pane, r.process, r.pid, r.cpu, r.mem, r.port, r.up, r.cwd}
    if r.includeGit {
        return append(base, r.gitCol)
    }
    return base
}

type paneGitWork struct {
    key     string // "session:windowIndex:paneIndex"
    paneCWD string
}

// fetchGitForPanes fetches git info for each pane work item in parallel,
// capped at gitConcurrencyCap concurrent goroutines (defined in sessions.go).
// Returns a map of key -> git.Info (entry absent on error).
func fetchGitForPanes(work []paneGitWork, timeoutMs int) map[string]git.Info {
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
            info, err := git.Fetch(w.paneCWD, timeoutMs)
            if err == nil {
                mu.Lock()
                results[w.key] = info
                mu.Unlock()
            }
        }()
    }
    wg.Wait()
    return results
}

func runProcs(cmd *cobra.Command, _ []string) error {
    cfg := loadConfig()

    allPanes, err := tmux.ListPanes()
    if err != nil {
        return fmt.Errorf("tmux not available: %w", err)
    }

    allProcs, err := proc.Snapshot()
    if err != nil {
        return err
    }

    cwdByPID, _ := proc.CWDAll()

    ports, _ := proc.ListeningPorts()
    portByPID := map[int32]int{}
    for _, p := range ports {
        portByPID[p.PID] = p.Port
    }

    grouped := tmux.GroupBySessions(allPanes)

    headers := []string{"SESSION", "WINDOW", "PANE", "PROCESS", "PID", "CPU", "MEM", "PORT", "UP", "CWD"}
    if procsGit {
        headers = append(headers, "GIT")
    }

    // Pre-fetch git info in parallel for all deviant panes.
    var gitWork []paneGitWork
    if procsGit {
        for sessionName, windows := range grouped {
            if procsSession != "" && sessionName != procsSession {
                continue
            }
            if isIgnored(cfg, sessionName) {
                continue
            }
            primaryCWD := primaryCWDForSession(windows)
            for wi, wPanes := range windows {
                if procsWindow != "" && fmt.Sprintf("%s:%d", sessionName, wi) != procsWindow {
                    continue
                }
                for _, pane := range wPanes {
                    paneCWD := pane.CWD
                    if !git.IsDescendant(paneCWD, primaryCWD) && paneCWD != primaryCWD {
                        key := fmt.Sprintf("%s:%d:%d", sessionName, wi, pane.PaneIndex)
                        gitWork = append(gitWork, paneGitWork{key: key, paneCWD: paneCWD})
                    }
                }
            }
        }
    }
    gitResults := fetchGitForPanes(gitWork, cfg.Git.TimeoutMs)

    var rows []format.Row

    for sessionName, windows := range grouped {
        if procsSession != "" && sessionName != procsSession {
            continue
        }
        if isIgnored(cfg, sessionName) {
            continue
        }

        primaryCWD := primaryCWDForSession(windows)

        for wi, wPanes := range windows {
            if procsWindow != "" && fmt.Sprintf("%s:%d", sessionName, wi) != procsWindow {
                continue
            }

            for _, pane := range wPanes {
                paneCWD := pane.CWD
                gitCol := "—"
                if procsGit {
                    key := fmt.Sprintf("%s:%d:%d", sessionName, wi, pane.PaneIndex)
                    if !git.IsDescendant(paneCWD, primaryCWD) && paneCWD != primaryCWD {
                        if info, ok := gitResults[key]; ok {
                            gitCol = "↪ " + info.Branch + " " + gitIndicators(info)
                        } else {
                            gitCol = cfg.Git.ErrorDisplay
                        }
                    }
                }

                rows = append(rows, procRow{
                    session:    sessionName,
                    window:     fmt.Sprint(wi),
                    pane:       fmt.Sprint(pane.PaneIndex),
                    process:    "(pane)",
                    pid:        "—",
                    cpu:        "—",
                    mem:        "—",
                    port:       "—",
                    up:         "—",
                    cwd:        paneCWD,
                    gitCol:     gitCol,
                    includeGit: procsGit,
                })

                for _, p := range allProcs {
                    cwd, ok := cwdByPID[p.PID]
                    if !ok || cwd != paneCWD {
                        continue
                    }
                    portStr := "—"
                    if port, ok := portByPID[p.PID]; ok {
                        portStr = fmt.Sprintf(":%d", port)
                    }
                    rows = append(rows, procRow{
                        session:    sessionName,
                        window:     fmt.Sprint(wi),
                        pane:       fmt.Sprint(pane.PaneIndex),
                        process:    p.Name,
                        pid:        fmt.Sprint(p.PID),
                        cpu:        fmt.Sprintf("%.1f%%", p.CPU),
                        mem:        formatMem(p.MemRSS),
                        port:       portStr,
                        up:         formatDuration(p.Uptime),
                        cwd:        paneCWD,
                        gitCol:     "—",
                        includeGit: procsGit,
                    })
                }
            }
        }
    }

    isTTYVal := isatty.IsTerminal(os.Stdout.Fd())
    fmt.Println(format.Render(resolveFormat(cmd), headers, rows, isTTYVal))
    return nil
}

func formatMem(bytes uint64) string {
    mb := float64(bytes) / 1024 / 1024
    return fmt.Sprintf("%.1fMB", mb)
}

func formatDuration(d time.Duration) string {
    h := int(d.Hours())
    m := int(d.Minutes()) % 60
    s := int(d.Seconds()) % 60
    switch {
    case h >= 24:
        return fmt.Sprintf("%dd%dh", h/24, h%24)
    case h > 0:
        return fmt.Sprintf("%dh%dm", h, m)
    case m > 0:
        return fmt.Sprintf("%dm", m)
    default:
        return fmt.Sprintf("%ds", s)
    }
}
