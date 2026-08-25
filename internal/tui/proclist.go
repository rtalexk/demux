package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rtalexk/demux/internal/config"
	"github.com/rtalexk/demux/internal/db"
	"github.com/rtalexk/demux/internal/git"
	"github.com/rtalexk/demux/internal/proc"
	"github.com/rtalexk/demux/internal/query"
	"github.com/rtalexk/demux/internal/tmux"
)

type ProcListNode struct {
	IsPaneHeader   bool
	IsIdle         bool // placeholder row shown when a pane has no processes
	IsWindowHeader bool // true for window-level header rows in session mode
	Pane           tmux.Pane
	GitDeviant     bool
	GitInfo        git.Info
	Proc           proc.Process
	Port           int
	Depth          int // 0=pane header, 1=process, 2=subprocess
	// collapse support (depth-1 nodes only)
	HasChildren bool    // true if this depth-1 node has at least one non-ignored child
	Collapsed   bool    // true when children are hidden
	AggCPU      float64 // CPU% summed across parent + all descendants
	AggMemRSS   uint64  // MemRSS summed across parent + all descendants
	// tree drawing (set by assignTreePrefixes after SetSessionData)
	TreePrefix string // line-1 prefix, e.g. "  ├─ " or "  └─ "
	StatPrefix string // line-2 (stats) prefix, e.g. "  │  " or "     "
}

type ProcListModel struct {
	nodes          []ProcListNode
	cursor         int
	offset         int // viewport scroll offset (by node index)
	primaryCWD     string
	curSession     string
	curWindow      int
	collapsedPIDs  map[int32]bool // persists collapse state across SetSessionData rebuilds
	pendingSeekKey string         // node identity to restore cursor after next rebuild
	inSessionMode  bool           // true when displaying all windows of a session
	cfg            config.Config
	searchQuery    query.ParsedQuery
	queryResult    query.Result
	states         []db.ToolState // latest states snapshot for inline pane indicators
}

// SetStates updates the state snapshot used for inline pane-header indicators.
func (p *ProcListModel) SetStates(states []db.ToolState) {
	p.states = states
}

// stateForPane returns the state for the given pane, or nil.
func (p ProcListModel) stateForPane(pane tmux.Pane) *db.ToolState {
	return activeStateFor(p.states, db.TargetTypePane, pane.PaneID)
}

// stateForWindow returns the active state for a window-level target, or nil.
func (p ProcListModel) stateForWindow(windowID string) *db.ToolState {
	return activeStateFor(p.states, db.TargetTypeWindow, windowID)
}

// activeStateFor returns the state matching the target type and ID, or nil.
func activeStateFor(states []db.ToolState, targetType db.TargetType, targetID string) *db.ToolState {
	for i := range states {
		if states[i].Target.Type == targetType && states[i].Target.ID == targetID {
			return &states[i]
		}
	}
	return nil
}

// paneRoots returns the level-0 processes for a pane: the pane root proc
// itself (tmux may run a command directly, with no shell wrapper), its tree
// children if the root PID is missing, or CWD-matched procs when PanePID is 0.
// Ignored roots are flattened to their children by the caller.
func paneRoots(pane tmux.Pane, procs []proc.Process, cwdMap map[int32]string, tree map[int32][]proc.Process) []proc.Process {
	if pane.PanePID != 0 {
		for _, pr := range procs {
			if pr.PID == pane.PanePID {
				return []proc.Process{pr}
			}
		}
		return tree[pane.PanePID]
	}
	paneCWD := pane.CWD
	var children []proc.Process
	for _, pr := range procs {
		cwd, ok := cwdMap[pr.PID]
		if !ok || (cwd != paneCWD && !git.IsDescendant(cwd, paneCWD)) {
			continue
		}
		children = append(children, pr)
	}
	return children
}

// depth1Meta computes collapse metadata for a depth-1 process node and
// updates collapsedPIDs with a default-collapsed entry when first seen.
func depth1Meta(pr proc.Process, tree map[int32][]proc.Process, collapsedPIDs map[int32]bool) (hasChildren bool, aggCPU float64, aggMem uint64, collapsed bool) {
	for _, child := range tree[pr.PID] {
		if !containsStr(activeIgnoredProcs, strings.ToLower(child.FriendlyName())) {
			hasChildren = true
			break
		}
	}
	aggCPU, aggMem = aggStats(pr, tree)
	if _, ok := collapsedPIDs[pr.PID]; !ok {
		collapsedPIDs[pr.PID] = true
	}
	collapsed = collapsedPIDs[pr.PID]
	return
}

// buildProcNodesForPane builds ProcListNode entries for a single pane.
// It collects the pane's level-0 processes and recurses into their subtrees,
// applying the same collapse/aggregate logic used by SetSessionData.
func buildProcNodesForPane(pane tmux.Pane, procs []proc.Process, cwdMap map[int32]string, tree map[int32][]proc.Process, collapsedPIDs map[int32]bool, primaryCWD string) []ProcListNode {
	roots := paneRoots(pane, procs, cwdMap, tree)
	seen := make(map[int32]bool)
	var nodes []ProcListNode
	var addProc func(pr proc.Process, depth int)
	addProc = func(pr proc.Process, depth int) {
		if seen[pr.PID] {
			return
		}
		seen[pr.PID] = true
		if containsStr(activeIgnoredProcs, strings.ToLower(pr.FriendlyName())) {
			for _, child := range tree[pr.PID] {
				addProc(child, depth)
			}
			return
		}

		var hasChildren bool
		var aggCPU float64
		var aggMem uint64
		var collapsed bool
		if depth == 1 {
			hasChildren, aggCPU, aggMem, collapsed = depth1Meta(pr, tree, collapsedPIDs)
		}

		nodes = append(nodes, ProcListNode{
			Proc:        pr,
			Pane:        pane,
			Depth:       depth,
			HasChildren: hasChildren,
			Collapsed:   collapsed,
			AggCPU:      aggCPU,
			AggMemRSS:   aggMem,
		})

		if depth == 1 && collapsed {
			return
		}
		for _, child := range tree[pr.PID] {
			addProc(child, depth+1)
		}
	}
	for _, pr := range roots {
		addProc(pr, 1)
	}
	return nodes
}

// appendPaneNodes appends a pane header node and its process nodes to p.nodes.
// displayPane may differ from pane (e.g. CWD suppressed when it matches winCWD).
// An idle placeholder is inserted when no process nodes are added.
func (p *ProcListModel) appendPaneNodes(pane tmux.Pane, displayPane tmux.Pane, winCWD string, procs []proc.Process, cwdMap map[int32]string, tree map[int32][]proc.Process, gitInfo map[string]git.Info) {
	paneCWD := pane.CWD
	gitKey := fmt.Sprintf("%s:%d:%d", pane.Session, pane.WindowIndex, pane.PaneIndex)
	info := gitInfo[gitKey]
	deviant := winCWD != "" && !git.IsDescendant(paneCWD, winCWD) && paneCWD != winCWD

	headerIdx := len(p.nodes)
	p.nodes = append(p.nodes, ProcListNode{
		IsPaneHeader: true,
		Pane:         displayPane,
		GitDeviant:   deviant,
		GitInfo:      info,
	})

	procNodes := buildProcNodesForPane(pane, procs, cwdMap, tree, p.collapsedPIDs, winCWD)
	p.nodes = append(p.nodes, procNodes...)

	// A sticky sidebar slot pane never shows an idle marker.
	if len(p.nodes) == headerIdx+1 && !pane.IsSlot {
		p.nodes = append(p.nodes, ProcListNode{IsIdle: true, Depth: 1})
	}
}

// SetSessionData rebuilds the node list for all windows of a session.
// A window header node (IsWindowHeader=true) is emitted before each window's
// pane and process nodes. Window CWD is taken as the CWD of the lowest-indexed pane;
// pane CWD is suppressed when it matches that value.
func (p *ProcListModel) SetSessionData(panes []tmux.Pane, session string, procs []proc.Process, cwdMap map[int32]string, gitInfo map[string]git.Info, cfg config.Config) {
	p.cfg = cfg
	p.inSessionMode = true

	grouped := tmux.GroupBySessions(panes)
	windows := grouped[session]
	tree := proc.BuildTree(procs)

	if p.collapsedPIDs == nil {
		p.collapsedPIDs = make(map[int32]bool)
	}

	sessionChanged := session != p.curSession || p.curWindow != -1
	p.curSession = session
	p.curWindow = -1
	p.nodes = nil
	if sessionChanged {
		p.cursor = 0
		p.offset = 0
		p.collapsedPIDs = make(map[int32]bool)
	}

	winIdxs := make([]int, 0, len(windows))
	for wi := range windows {
		winIdxs = append(winIdxs, wi)
	}
	sort.Ints(winIdxs)

	// Compute the session-level primary CWD from the first window's primary
	// pane. Sticky sidebar slot panes are skipped so an open sidebar does not
	// hijack the session's primary path.
	p.primaryCWD = ""
	for _, wi := range winIdxs {
		if cwd := tmux.PrimaryPaneCWD(windows[wi]); cwd != "" {
			p.primaryCWD = cwd
			break
		}
	}

	for _, wi := range winIdxs {
		p.appendWindowNodes(windows[wi], session, wi, procs, cwdMap, tree, gitInfo)
	}
	assignTreePrefixes(p.nodes)
	p.applyPendingSeek()
}

// appendWindowNodes appends a window header and all of its panes' nodes to p.nodes.
// It is a no-op when the window has no panes. The window's primary pane (and
// thus its CWD) skips any sticky sidebar slot so the window path reflects real
// work rather than the sidebar's stale path.
func (p *ProcListModel) appendWindowNodes(wPanes []tmux.Pane, session string, wi int, procs []proc.Process, cwdMap map[int32]string, tree map[int32][]proc.Process, gitInfo map[string]git.Info) {
	wPanes = sortPanes(wPanes)
	if len(wPanes) == 0 {
		return
	}
	headerPane := wPanes[0]
	if pp := tmux.PrimaryPane(wPanes); pp != nil {
		headerPane = *pp
	}
	winCWD := headerPane.CWD

	p.nodes = append(p.nodes, ProcListNode{
		IsWindowHeader: true,
		Pane:           headerPane,
	})

	for _, pane := range wPanes {
		displayPane := pane
		if pane.CWD == winCWD {
			displayPane.CWD = ""
		}
		p.appendPaneNodes(pane, displayPane, winCWD, procs, cwdMap, tree, gitInfo)
	}
}

// CurrentWindow returns the session name and window index currently displayed.
func (p ProcListModel) CurrentWindow() (string, int) {
	return p.curSession, p.curWindow
}

func (p *ProcListModel) Reset() {
	p.nodes = nil
	p.cursor = 0
	p.offset = 0
	p.curSession = ""
	p.curWindow = -1
}

func (p ProcListModel) SelectedNode() *ProcListNode {
	if p.cursor < 0 || p.cursor >= len(p.nodes) {
		return nil
	}
	n := p.nodes[p.cursor]
	if !isSelectable(n) {
		return nil
	}
	return &n
}

// SelectedPane returns the tmux.Pane containing the cursor node.
// If the cursor is on a pane header, that pane is returned directly.
// If the cursor is on a process or subprocess node, the method walks
// backwards to find the nearest enclosing pane header.
// Returns nil if the node list is empty or no pane header is found.
func (p ProcListModel) SelectedPane() *tmux.Pane {
	n := len(p.nodes)
	if n == 0 {
		return nil
	}
	start := p.cursor
	if start >= n {
		start = n - 1
	}
	for i := start; i >= 0; i-- {
		if p.nodes[i].IsPaneHeader || p.nodes[i].IsWindowHeader {
			pane := p.nodes[i].Pane
			return &pane
		}
	}
	return nil
}

// SetSearchQuery stores the current search query and its results so that
// Render can dim non-matching nodes and highlight matching characters.
func (p *ProcListModel) SetSearchQuery(pq query.ParsedQuery, r query.Result) {
	p.searchQuery = pq
	p.queryResult = r
}
