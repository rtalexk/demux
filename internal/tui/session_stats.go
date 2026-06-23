package tui

import (
	"fmt"
	"strings"

	"github.com/rtalexk/demux/internal/proc"
	"github.com/rtalexk/demux/internal/tmux"
)

// SessionStat is the resource readout for one tmux session: live totals,
// rolling peaks, and whether caffeinate is live in the session's pane trees.
type SessionStat struct {
	CPUNow      float64
	MemNow      uint64
	CPUPeak     float64
	MemPeak     uint64
	Caffeinated bool
}

// indexByPID builds a PID -> Process lookup over a snapshot.
func indexByPID(procs []proc.Process) map[int32]proc.Process {
	byPID := make(map[int32]proc.Process, len(procs))
	for _, p := range procs {
		byPID[p.PID] = p
	}
	return byPID
}

// subtreeHasCaffeinate reports whether rootPID or any descendant is a
// caffeinate process. Matches on the lowercased FriendlyName.
func subtreeHasCaffeinate(rootPID int32, byPID map[int32]proc.Process, tree map[int32][]proc.Process) bool {
	if p, ok := byPID[rootPID]; ok {
		if strings.ToLower(p.FriendlyName()) == "caffeinate" {
			return true
		}
	}
	for _, child := range tree[rootPID] {
		if subtreeHasCaffeinate(child.PID, byPID, tree) {
			return true
		}
	}
	return false
}

// computeSessionTotals sums CPU% and MemRSS across every pane's process tree
// for a single session, and reports whether caffeinate runs anywhere within.
// panes must all belong to the same session. Panes whose shell PID cannot be
// resolved (PanePID == 0 or absent from byPID) contribute nothing.
func computeSessionTotals(panes []tmux.Pane, byPID map[int32]proc.Process, tree map[int32][]proc.Process) (cpu float64, mem uint64, caffeinated bool) {
	for _, pane := range panes {
		if pane.PanePID == 0 {
			continue
		}
		shell, ok := byPID[pane.PanePID]
		if !ok {
			continue
		}
		c, m := aggStats(shell, tree)
		cpu += c
		mem += m
		if !caffeinated && subtreeHasCaffeinate(pane.PanePID, byPID, tree) {
			caffeinated = true
		}
	}
	return cpu, mem, caffeinated
}

// humanizeBytes renders a byte count as "<n>MB" below 1GiB, else "<n.n>GB".
func humanizeBytes(b uint64) string {
	const mib = 1024 * 1024
	const gib = 1024 * mib
	if b < gib {
		return fmt.Sprintf("%dMB", b/mib)
	}
	return fmt.Sprintf("%.1fGB", float64(b)/float64(gib))
}

// humanizeBytesShort is the compact form for the inline per-row readout:
// "<n>M" below 1GiB, else "<n.n>G" (no trailing "B" to save sidebar width).
func humanizeBytesShort(b uint64) string {
	const mib = 1024 * 1024
	const gib = 1024 * mib
	if b < gib {
		return fmt.Sprintf("%dM", b/mib)
	}
	return fmt.Sprintf("%.1fG", float64(b)/float64(gib))
}

// statsPeakWindow is the number of recent samples retained per session for the
// rolling peak. At the ~2s proc-poll interval this is ~2 minutes of history.
const statsPeakWindow = 60

type statSample struct {
	cpu float64
	mem uint64
}

// SessionStatsModel computes per-session resource totals each refresh and
// tracks a rolling-window peak. The zero value is ready to use; maps are
// lazily initialized in Update.
type SessionStatsModel struct {
	rings map[string][]statSample
	cur   map[string]SessionStat
}

// Update recomputes current totals for every live session and rolls the peak
// window forward. Sessions absent from panes are pruned.
func (m *SessionStatsModel) Update(panes []tmux.Pane, procs []proc.Process) {
	if m.rings == nil {
		m.rings = make(map[string][]statSample)
	}
	tree := proc.BuildTree(procs)
	byPID := indexByPID(procs)
	grouped := tmux.GroupBySessions(panes)

	newRings := make(map[string][]statSample, len(grouped))
	newCur := make(map[string]SessionStat, len(grouped))

	for session, windows := range grouped {
		var sessionPanes []tmux.Pane
		for _, wPanes := range windows {
			sessionPanes = append(sessionPanes, wPanes...)
		}
		cpu, mem, caf := computeSessionTotals(sessionPanes, byPID, tree)

		ring := append(m.rings[session], statSample{cpu: cpu, mem: mem})
		if len(ring) > statsPeakWindow {
			ring = ring[len(ring)-statsPeakWindow:]
		}
		newRings[session] = ring

		var cpuPeak float64
		var memPeak uint64
		for _, s := range ring {
			if s.cpu > cpuPeak {
				cpuPeak = s.cpu
			}
			if s.mem > memPeak {
				memPeak = s.mem
			}
		}
		newCur[session] = SessionStat{
			CPUNow:      cpu,
			MemNow:      mem,
			CPUPeak:     cpuPeak,
			MemPeak:     memPeak,
			Caffeinated: caf,
		}
	}

	m.rings = newRings // pruning: sessions not in `grouped` are dropped
	m.cur = newCur
}

// Snapshot returns the latest per-session stats. The returned map is replaced
// wholesale on each Update; callers must treat it as read-only.
func (m SessionStatsModel) Snapshot() map[string]SessionStat {
	if m.cur == nil {
		return map[string]SessionStat{}
	}
	return m.cur
}
