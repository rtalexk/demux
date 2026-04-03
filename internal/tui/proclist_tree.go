package tui

import (
	"fmt"
	"sort"

	"github.com/rtalexk/demux/internal/db"
	"github.com/rtalexk/demux/internal/proc"
	"github.com/rtalexk/demux/internal/tmux"
)

func sortPanes(panes []tmux.Pane) []tmux.Pane {
	sorted := make([]tmux.Pane, len(panes))
	copy(sorted, panes)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].PaneIndex < sorted[j].PaneIndex })
	return sorted
}

// assignTreePrefixes fills TreePrefix and StatPrefix on every non-header node.
// It must be called after p.nodes is fully built by SetWindowData or SetSessionData.
//
// Connectors used:
//
//	"├─ " non-last sibling at this depth
//	"└─ " last sibling at this depth
//	"│  " ancestor continuation (non-last ancestor)
//	"   " ancestor continuation (last ancestor)
//
// The base indent is 2 spaces (pane-level offset), matching the current
// plain-indent scheme. Each connector/continuation segment is 3 visual columns.
func assignTreePrefixes(nodes []ProcListNode) {
	n := len(nodes)
	for i := range nodes {
		nd := &nodes[i]
		if nd.Depth == 0 {
			continue
		}
		depth := nd.Depth

		// Determine if this node is the last sibling at its depth.
		// Stop scanning at a pane header (depth 0) or a shallower depth.
		isLast := true
		for j := i + 1; j < n; j++ {
			jd := nodes[j].Depth
			if jd == 0 || jd < depth {
				break
			}
			if jd == depth {
				isLast = false
				break
			}
		}

		// Build the ancestor continuation chain for depths 1..depth-1.
		// For each ancestor level, scan backward to find the nearest ancestor
		// node at that depth and check whether it was itself last at that depth.
		cont := "  " // base 2-space indent
		for d := 1; d < depth; d++ {
			ancestorIsLast := true
			for j := i - 1; j >= 0; j-- {
				if nodes[j].Depth == 0 {
					break // crossed pane boundary
				}
				if nodes[j].Depth == d {
					// Check whether this ancestor node is last at depth d.
					for k := j + 1; k < n; k++ {
						kd := nodes[k].Depth
						if kd == 0 || kd < d {
							break
						}
						if kd == d {
							ancestorIsLast = false
							break
						}
					}
					break
				}
			}
			if ancestorIsLast {
				cont += "   "
			} else {
				cont += "│  "
			}
		}

		if isLast {
			nd.TreePrefix = cont + "└─ "
			nd.StatPrefix = cont + "   "
		} else {
			nd.TreePrefix = cont + "├─ "
			nd.StatPrefix = cont + "│  "
		}
	}
}

// aggStats returns the total CPU% and MemRSS for pr and all its descendants.
func aggStats(pr proc.Process, tree map[int32][]proc.Process) (cpu float64, mem uint64) {
	cpu = pr.CPU
	mem = pr.MemRSS
	for _, child := range tree[pr.PID] {
		c, m := aggStats(child, tree)
		cpu += c
		mem += m
	}
	return
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// windowAlertFromMap returns the window-level alert for a window from alertMap.
// It checks only the exact window target ("session:N"); pane-level alerts
// ("session:N.P") are not included so they appear only on their pane header.
func windowAlertFromMap(alertMap map[string]db.Alert, session string, windowIndex int) *db.Alert {
	if alertMap == nil {
		return nil
	}
	exact := fmt.Sprintf("%s:%d", session, windowIndex)
	if a, ok := alertMap[exact]; ok {
		return &a
	}
	return nil
}

// isSelectable reports whether the cursor may land on n.
func isSelectable(n ProcListNode) bool { return !n.IsIdle }

// nodeDepth returns the logical depth of a node: 0 for pane headers, otherwise Depth.
func nodeDepth(n ProcListNode) int {
	if n.IsPaneHeader {
		return 0
	}
	return n.Depth
}

// nodeRows returns how many terminal rows a node occupies when rendered.
func nodeRows(n ProcListNode) int {
	if n.IsIdle {
		return 0
	}
	if n.IsPaneHeader || n.IsWindowHeader {
		return 1
	}
	return 2 // process: name line + stats line
}

// windowMatchPos returns the match positions for a window in the current query
// result, or nil if the window is not a match.
func (p *ProcListModel) windowMatchPos(sessionName string, windowIdx int) []int {
	for _, sm := range p.queryResult.Sessions {
		if sm.Name != sessionName {
			continue
		}
		for _, wm := range sm.Windows {
			if wm.Index == windowIdx {
				return wm.MatchPos
			}
		}
	}
	return nil
}

// procMatchPos returns the match positions for a process in the current query
// result, or nil if the process is not a match.
func (p *ProcListModel) procMatchPos(sessionName string, pid int32) []int {
	for _, sm := range p.queryResult.Sessions {
		if sm.Name != sessionName {
			continue
		}
		for _, pm := range sm.Procs {
			if pm.PID == pid {
				return pm.MatchPos
			}
		}
	}
	return nil
}
