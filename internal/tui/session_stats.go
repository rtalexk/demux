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
