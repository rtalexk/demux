package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/rtalexk/demux/internal/config"
	"github.com/rtalexk/demux/internal/db"
	"github.com/rtalexk/demux/internal/format"
	"github.com/rtalexk/demux/internal/git"
	demuxlog "github.com/rtalexk/demux/internal/log"
	"github.com/rtalexk/demux/internal/proc"
	"github.com/rtalexk/demux/internal/tmux"
	"github.com/spf13/cobra"
)

var (
	sessionListGit     bool
	sessionListGitOnly bool
)

var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tmux sessions",
	RunE:  runSessions,
}

func init() {
	sessionCmd.AddCommand(sessionListCmd)
	sessionListCmd.Flags().BoolVar(&sessionListGit, "git", false, "Include git columns")
	sessionListCmd.Flags().BoolVar(&sessionListGitOnly, "git-only", false, "Show only session + git columns")
}

type sessionRow struct {
	session, windows, procs, state, status string
	branch, dirty, ahead, behind           string
	includeGit, gitOnly                    bool
}

func (r sessionRow) Fields() []string {
	base := []string{r.session, r.windows, r.procs, r.state, r.status}
	gitCols := []string{r.branch, r.dirty, r.ahead, r.behind}
	if r.gitOnly {
		return append([]string{r.session}, gitCols...)
	}
	if r.includeGit {
		return append(base, gitCols...)
	}
	return base
}

func buildSessionProcCounts(grouped map[string]map[int][]tmux.Pane, allProcs []proc.Process, cwdByPID map[int32]string) map[string]int {
	counts := map[string]int{}
	for sessionName, windows := range grouped {
		primaryCWD := tmux.PrimaryPaneCWD(windows[0])
		if primaryCWD == "" {
			continue
		}
		for _, p := range allProcs {
			cwd, ok := cwdByPID[p.PID]
			if !ok {
				continue
			}
			if cwd == primaryCWD || git.IsDescendant(cwd, primaryCWD) {
				counts[sessionName]++
			}
		}
	}
	return counts
}

func buildSessionGitWork(grouped map[string]map[int][]tmux.Pane, cfg config.Config) []git.ConcurrentWork {
	var work []git.ConcurrentWork
	for sessionName, windows := range grouped {
		if isIgnored(cfg, sessionName) {
			continue
		}
		if cwd := tmux.PrimaryPaneCWD(windows[0]); cwd != "" {
			work = append(work, git.ConcurrentWork{Key: sessionName, Dir: cwd})
		}
	}
	return work
}

func fillSessionGitFields(row *sessionRow, sessionName string, windows map[int][]tmux.Pane, gitResults map[string]git.Info, cfg config.Config) {
	primaryCWD := tmux.PrimaryPaneCWD(windows[0])
	if primaryCWD == "" {
		row.branch = cfg.Git.FallbackDisplay
		row.dirty = "—"
		row.ahead = "—"
		row.behind = "—"
		return
	}
	info, ok := gitResults[sessionName]
	if !ok {
		row.branch = cfg.Git.ErrorDisplay
		row.dirty = "—"
		row.ahead = "—"
		row.behind = "—"
		return
	}
	row.branch = info.Branch
	if info.Dirty {
		row.dirty = "yes"
	} else {
		row.dirty = "no"
	}
	row.ahead = fmt.Sprint(info.Ahead)
	row.behind = fmt.Sprint(info.Behind)
}

func buildSessionRows(grouped map[string]map[int][]tmux.Pane, sessionProcCount map[string]int, statesBySession map[string]db.ToolState, gitResults map[string]git.Info, cfg config.Config, includeGit, gitOnly bool) []format.Row {
	var rows []format.Row
	for sessionName, windows := range grouped {
		if isIgnored(cfg, sessionName) {
			continue
		}
		sessionState := statesBySession[sessionName]
		stateStr := ""
		if sessionState.Value != 0 {
			stateStr = sessionState.Value.String()
		}
		status := resolveSessionStatus(statesBySession)

		row := sessionRow{
			session:    sessionName,
			windows:    fmt.Sprint(len(windows)),
			procs:      fmt.Sprint(sessionProcCount[sessionName]),
			state:      stateStr,
			status:     status,
			includeGit: includeGit,
			gitOnly:    gitOnly,
		}

		if includeGit || gitOnly {
			fillSessionGitFields(&row, sessionName, windows, gitResults, cfg)
		}

		rows = append(rows, row)
	}
	return rows
}

func buildStatesBySession(states []db.ToolState) map[string]db.ToolState {
	out := map[string]db.ToolState{}
	for _, st := range states {
		parts := strings.SplitN(st.Target, ":", 2)
		if len(parts) > 0 {
			sessName := parts[0]
			// Keep highest-priority state per session
			if existing, ok := out[sessName]; !ok || st.Value > existing.Value {
				out[sessName] = st
			}
		}
	}
	return out
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
	states, err := database.StateList(0, "")
	if err != nil {
		demuxlog.Warn("failed to list states", "err", err)
	}

	statesBySession := buildStatesBySession(states)

	headers := []string{"SESSION", "WINDOWS", "PROCS", "STATE", "STATUS"}
	if sessionListGitOnly {
		headers = []string{"SESSION", "BRANCH", "DIRTY", "AHEAD", "BEHIND"}
	} else if sessionListGit {
		headers = append(headers, "BRANCH", "DIRTY", "AHEAD", "BEHIND")
	}

	allProcs, err := proc.Snapshot()
	if err != nil {
		demuxlog.Warn("proc snapshot failed", "err", err)
	}
	cwdByPID, err := proc.CWDAll()
	if err != nil {
		demuxlog.Warn("cwd fetch failed", "err", err)
	}

	sessionProcCount := buildSessionProcCounts(grouped, allProcs, cwdByPID)

	var gitWork []git.ConcurrentWork
	if sessionListGit || sessionListGitOnly {
		gitWork = buildSessionGitWork(grouped, cfg)
	}
	gitResults := git.FetchConcurrent(gitWork, cfg.Git.TimeoutMs)

	rows := buildSessionRows(grouped, sessionProcCount, statesBySession, gitResults, cfg, sessionListGit, sessionListGitOnly)

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

func resolveSessionStatus(statesBySession map[string]db.ToolState) string {
	for _, st := range statesBySession {
		switch st.Value {
		case db.StateError:
			return "error"
		case db.StateFlagged:
			return "flagged"
		}
	}
	for _, st := range statesBySession {
		if st.Value == db.StateWaiting {
			return "waiting"
		}
	}
	return "ok"
}
