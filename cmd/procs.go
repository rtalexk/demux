package cmd

import (
	"fmt"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/rtalexk/demux/internal/config"
	"github.com/rtalexk/demux/internal/format"
	"github.com/rtalexk/demux/internal/git"
	demuxlog "github.com/rtalexk/demux/internal/log"
	"github.com/rtalexk/demux/internal/proc"
	"github.com/rtalexk/demux/internal/tmux"
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
	gitCol                                                       string
	includeGit                                                   bool
}

func (r procRow) Fields() []string {
	base := []string{r.session, r.window, r.pane, r.process, r.pid, r.cpu, r.mem, r.port, r.up, r.cwd}
	if r.includeGit {
		return append(base, r.gitCol)
	}
	return base
}

func resolvePortMap(ports []proc.PortInfo) map[int32]int {
	m := map[int32]int{}
	for _, p := range ports {
		m[p.PID] = p.Port
	}
	return m
}

// paneGitWork returns a git work item for a pane whose CWD diverges from the
// session primary CWD, or nil if no extra fetch is needed.
func paneGitWork(pane tmux.Pane, wi int, sessionName, primaryCWD string) *git.ConcurrentWork {
	if !git.IsDescendant(pane.CWD, primaryCWD) && pane.CWD != primaryCWD {
		k := fmt.Sprintf("%s:%d:%d", sessionName, wi, pane.PaneIndex)
		return &git.ConcurrentWork{Key: k, Dir: pane.CWD}
	}
	return nil
}

// collectProcsGitWork builds the list of git fetch work items for panes whose
// CWD diverges from the session primary CWD.
func collectProcsGitWork(grouped map[string]map[int][]tmux.Pane, cfg config.Config, sessionFilter, windowFilter string) []git.ConcurrentWork {
	var work []git.ConcurrentWork
	for sessionName, windows := range grouped {
		if sessionFilter != "" && sessionName != sessionFilter {
			continue
		}
		if isIgnored(cfg, sessionName) {
			continue
		}
		primaryCWD := tmux.PrimaryPaneCWD(windows[0])
		for wi, wPanes := range windows {
			if windowFilter != "" && fmt.Sprintf("%s:%d", sessionName, wi) != windowFilter {
				continue
			}
			for _, pane := range wPanes {
				if w := paneGitWork(pane, wi, sessionName, primaryCWD); w != nil {
					work = append(work, *w)
				}
			}
		}
	}
	return work
}

// resolvePaneGitCol returns the git column value for a single pane.
func resolvePaneGitCol(pane tmux.Pane, wi int, sessionName, primaryCWD string, gitResults map[string]git.Info, cfg config.Config) string {
	paneCWD := pane.CWD
	if !git.IsDescendant(paneCWD, primaryCWD) && paneCWD != primaryCWD {
		k := fmt.Sprintf("%s:%d:%d", sessionName, wi, pane.PaneIndex)
		if info, ok := gitResults[k]; ok {
			return "↪ " + info.Branch + " " + git.Indicators(info)
		}
		return cfg.Git.ErrorDisplay
	}
	return "—"
}

// buildPaneRows builds the pane header row plus one row per matching process.
func buildPaneRows(pane tmux.Pane, wi int, sessionName string, allProcs []proc.Process, cwdByPID map[int32]string, portByPID map[int32]int, gitCol string, includeGit bool) []format.Row {
	paneCWD := pane.CWD
	rows := []format.Row{procRow{
		session: sessionName, window: fmt.Sprint(wi),
		pane: fmt.Sprint(pane.PaneIndex), process: "(pane)",
		pid: "—", cpu: "—", mem: "—", port: "—", up: "—",
		cwd: paneCWD, gitCol: gitCol, includeGit: includeGit,
	}}
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
			session: sessionName, window: fmt.Sprint(wi),
			pane: fmt.Sprint(pane.PaneIndex), process: p.Name,
			pid: fmt.Sprint(p.PID), cpu: fmt.Sprintf("%.1f%%", p.CPU),
			mem: format.Mem(p.MemRSS), port: portStr,
			up: format.Duration(p.Uptime), cwd: paneCWD,
			gitCol: "—", includeGit: includeGit,
		})
	}
	return rows
}

// buildProcRows builds the display rows for all panes and their processes.
func buildProcRows(grouped map[string]map[int][]tmux.Pane, allProcs []proc.Process, cwdByPID map[int32]string, portByPID map[int32]int, gitResults map[string]git.Info, cfg config.Config, sessionFilter, windowFilter string, includeGit bool) []format.Row {
	var rows []format.Row
	for sessionName, windows := range grouped {
		if sessionFilter != "" && sessionName != sessionFilter {
			continue
		}
		if isIgnored(cfg, sessionName) {
			continue
		}
		primaryCWD := tmux.PrimaryPaneCWD(windows[0])
		for wi, wPanes := range windows {
			if windowFilter != "" && fmt.Sprintf("%s:%d", sessionName, wi) != windowFilter {
				continue
			}
			for _, pane := range wPanes {
				gitCol := "—"
				if includeGit {
					gitCol = resolvePaneGitCol(pane, wi, sessionName, primaryCWD, gitResults, cfg)
				}
				rows = append(rows, buildPaneRows(pane, wi, sessionName, allProcs, cwdByPID, portByPID, gitCol, includeGit)...)
			}
		}
	}
	return rows
}

func runProcs(cmd *cobra.Command, _ []string) error {
	cfg := loadConfig()

	allPanes, err := tmux.ListPanes()
	if err != nil {
		return fmt.Errorf("tmux not available: %w", err)
	}

	allProcs, err := proc.Snapshot()
	if err != nil {
		return fmt.Errorf("snapshot procs: %w", err)
	}

	cwdByPID, err := proc.CWDAll()
	if err != nil {
		demuxlog.Warn("cwd fetch failed", "err", err)
	}

	ports, err := proc.ListeningPorts()
	if err != nil {
		demuxlog.Warn("port list failed", "err", err)
	}
	portByPID := resolvePortMap(ports)

	grouped := tmux.GroupBySessions(allPanes)

	headers := []string{"SESSION", "WINDOW", "PANE", "PROCESS", "PID", "CPU", "MEM", "PORT", "UP", "CWD"}
	if procsGit {
		headers = append(headers, "GIT")
	}

	// Pre-fetch git info in parallel for all deviant panes.
	var gitWork []git.ConcurrentWork
	if procsGit {
		gitWork = collectProcsGitWork(grouped, cfg, procsSession, procsWindow)
	}
	gitResults := git.FetchConcurrent(gitWork, cfg.Git.TimeoutMs)

	rows := buildProcRows(grouped, allProcs, cwdByPID, portByPID, gitResults, cfg, procsSession, procsWindow, procsGit)

	isTTYVal := isatty.IsTerminal(os.Stdout.Fd())
	fmt.Println(format.Render(resolveFormat(cmd), headers, rows, isTTYVal))
	return nil
}
