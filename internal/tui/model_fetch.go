package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rtalexk/demux/internal/git"
	demuxlog "github.com/rtalexk/demux/internal/log"
	"github.com/rtalexk/demux/internal/proc"
	"github.com/rtalexk/demux/internal/session"
	"github.com/rtalexk/demux/internal/tmux"
)

func tick(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func resolveWindowSpecs(ids []string, templates map[string]session.WindowTemplate) []tmux.WindowSpec {
	specs, unknown := session.ResolveWindowSpecs(ids, templates)
	for _, id := range unknown {
		demuxlog.Warn("session references unknown window_template id", "id", id)
	}
	return specs
}

func (m Model) fetchPanes() tea.Cmd {
	return func() tea.Msg {
		panes, err := tmux.ListPanes()
		if err != nil {
			return panesMsg{}
		}
		session, _, _ := tmux.CurrentTarget()
		return panesMsg{panes: panes, currentSession: session}
	}
}

func (m Model) fetchStates() tea.Cmd {
	return func() tea.Msg {
		states, err := m.db.StateList(0, "")
		if err != nil {
			demuxlog.Warn("fetch states failed", "err", err)
			return statesMsg{}
		}
		return statesMsg{states: states}
	}
}

func (m Model) fetchWatches() tea.Cmd {
	return func() tea.Msg {
		watches, err := m.db.WatchList()
		if err != nil {
			demuxlog.Warn("fetch watches failed", "err", err)
			return watchesMsg{watches: []string{}}
		}
		return watchesMsg{watches: watches}
	}
}

// makeProcFetch returns a closure that snapshots processes and CWD maps,
// tagged with the given generation so stale results can be discarded.
func makeProcFetch(gen int) func() tea.Msg {
	return func() tea.Msg {
		procs, err := proc.Snapshot()
		if err != nil {
			return procDataMsg{gen: gen}
		}
		cwdMap, err := proc.CWDAll()
		if err != nil {
			demuxlog.Warn("cwd fetch failed", "err", err)
		}
		if cwdMap == nil {
			cwdMap = make(map[int32]string)
		}
		return procDataMsg{procs: procs, cwdMap: cwdMap, gen: gen}
	}
}

// scheduleProcFetch fires an immediate proc snapshot tagged with the current generation.
// Stale results (gen mismatch) are discarded in the procDataMsg handler.
func (m Model) scheduleProcFetch() tea.Cmd {
	return makeProcFetch(m.procGen)
}

// scheduleDelayedProcFetch schedules a proc snapshot after 2s, tagged with the current generation.
func (m Model) scheduleDelayedProcFetch() tea.Cmd {
	fetch := makeProcFetch(m.procGen)
	return tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return fetch()
	})
}

func fetchGit(k, dir string, timeoutMs int) tea.Cmd {
	return func() tea.Msg {
		info, err := git.Fetch(dir, timeoutMs)
		if err != nil {
			// Preserve Dir and IsWorktreeRoot even when git is unavailable.
			return gitResultMsg{key: k, info: git.Info{Dir: info.Dir, IsWorktreeRoot: info.IsWorktreeRoot}}
		}
		return gitResultMsg{key: k, info: info}
	}
}

func debounceSearch(gen int) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(150 * time.Millisecond)
		return searchDebounceMsg{gen: gen}
	}
}
