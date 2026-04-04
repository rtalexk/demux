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
    Alert          *db.Alert
    Proc           proc.Process
    Port           int
    Depth          int // 0=pane header, 1=process, 2=subprocess
    // collapse support (depth-1 nodes only)
    HasChildren bool    // true if this depth-1 node has at least one non-ignored child
    Collapsed   bool    // true when children are hidden
    AggCPU      float64 // CPU% summed across parent + all descendants
    AggMemRSS   uint64  // MemRSS summed across parent + all descendants
    // tree drawing (set by assignTreePrefixes after SetWindowData)
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
    collapsedPIDs  map[int32]bool // persists collapse state across SetWindowData calls
    pendingSeekKey string         // node identity to restore cursor after next rebuild
    inSessionMode  bool           // true when displaying all windows of a session
    sessionAlert   *db.Alert
    cfg            config.Config
    searchQuery    query.ParsedQuery
    queryResult    query.Result
}

// TODO: single-window mode (SetWindowData) must be removed — selecting individual
// windows from the sidebar is no longer a feature.

// paneDirectChildren returns the immediate child processes for a pane.
// When PanePID is set it reads directly from the tree; otherwise it falls
// back to scanning procs by CWD.
func paneDirectChildren(pane tmux.Pane, procs []proc.Process, cwdMap map[int32]string, tree map[int32][]proc.Process) []proc.Process {
    if pane.PanePID != 0 {
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
// It collects direct children of the pane's shell process (or CWD-matched
// processes when PanePID is 0) and recurses into their subtrees, applying
// the same collapse/aggregate logic used by SetWindowData and SetSessionData.
func buildProcNodesForPane(pane tmux.Pane, procs []proc.Process, cwdMap map[int32]string, tree map[int32][]proc.Process, alertMap map[string]db.Alert, collapsedPIDs map[int32]bool, primaryCWD string) []ProcListNode {
    children := paneDirectChildren(pane, procs, cwdMap, tree)
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
    for _, pr := range children {
        addProc(pr, 1)
    }
    return nodes
}

// paneAlertFromMap looks up the alert for a single pane from alertMap.
// The key format is "session:windowIndex.paneIndex". Returns nil when not found.
func paneAlertFromMap(alertMap map[string]db.Alert, pane tmux.Pane) *db.Alert {
    if alertMap == nil {
        return nil
    }
    key := fmt.Sprintf("%s:%d.%d", pane.Session, pane.WindowIndex, pane.PaneIndex)
    if a, ok := alertMap[key]; ok {
        return &a
    }
    return nil
}

// appendPaneNodes appends a pane header node and its process nodes to p.nodes.
// displayPane may differ from pane (e.g. CWD suppressed when it matches winCWD).
// An idle placeholder is inserted when no process nodes are added.
func (p *ProcListModel) appendPaneNodes(pane tmux.Pane, displayPane tmux.Pane, winCWD string, procs []proc.Process, cwdMap map[int32]string, tree map[int32][]proc.Process, gitInfo map[string]git.Info, alertMap map[string]db.Alert) {
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
        Alert:        paneAlertFromMap(alertMap, pane),
    })

    procNodes := buildProcNodesForPane(pane, procs, cwdMap, tree, alertMap, p.collapsedPIDs, winCWD)
    p.nodes = append(p.nodes, procNodes...)

    if len(p.nodes) == headerIdx+1 {
        p.nodes = append(p.nodes, ProcListNode{IsIdle: true, Depth: 1})
    }
}

// SetWindowData rebuilds the node list from pre-fetched data.
// procs is the process snapshot, cwdMap maps PID to CWD (pre-fetched),
// gitInfo is keyed by "session:windowIndex:paneIndex" for deviant panes, and
// alertMap is keyed by "session:windowIndex.paneIndex" for pane-level alerts.
func (p *ProcListModel) SetWindowData(panes []tmux.Pane, session string, windowIndex int, procs []proc.Process, cwdMap map[int32]string, gitInfo map[string]git.Info, alertMap map[string]db.Alert, cfg config.Config) {
    p.cfg = cfg
    p.inSessionMode = false
    p.sessionAlert = nil
    grouped := tmux.GroupBySessions(panes)
    windows := grouped[session]
    p.primaryCWD = tmux.PrimaryPaneCWD(windows[0])
    wPanes := windows[windowIndex]

    tree := proc.BuildTree(procs)

    if p.collapsedPIDs == nil {
        p.collapsedPIDs = make(map[int32]bool)
    }

    windowChanged := session != p.curSession || windowIndex != p.curWindow
    p.curSession = session
    p.curWindow = windowIndex
    p.nodes = nil
    if windowChanged {
        p.cursor = 0
        p.offset = 0
        p.collapsedPIDs = make(map[int32]bool) // reset so new window's PIDs default to collapsed
    }

    for _, pane := range sortPanes(wPanes) {
        p.appendPaneNodes(pane, pane, p.primaryCWD, procs, cwdMap, tree, gitInfo, alertMap)
    }
    assignTreePrefixes(p.nodes)
    p.applyPendingSeek()
}

// SetSessionData rebuilds the node list for all windows of a session.
// A window header node (IsWindowHeader=true) is emitted before each window's
// pane and process nodes. Window CWD is taken as the CWD of the lowest-indexed pane;
// pane CWD is suppressed when it matches that value.
func (p *ProcListModel) SetSessionData(panes []tmux.Pane, session string, procs []proc.Process, cwdMap map[int32]string, gitInfo map[string]git.Info, alertMap map[string]db.Alert, cfg config.Config) {
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
    p.sessionAlert = nil
    if alertMap != nil {
        if a, ok := alertMap[session]; ok {
            p.sessionAlert = &a
        }
    }
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

    // Compute session-level primary CWD from the first window's first pane.
    p.primaryCWD = ""
    for _, wi := range winIdxs {
        ps := sortPanes(windows[wi])
        if len(ps) > 0 {
            p.primaryCWD = ps[0].CWD
            break
        }
    }

    for _, wi := range winIdxs {
        p.appendWindowNodes(windows[wi], session, wi, procs, cwdMap, tree, gitInfo, alertMap)
    }
    assignTreePrefixes(p.nodes)
    p.applyPendingSeek()
}

// appendWindowNodes appends a window header and all of its panes' nodes to p.nodes.
// It is a no-op when the window has no panes.
func (p *ProcListModel) appendWindowNodes(wPanes []tmux.Pane, session string, wi int, procs []proc.Process, cwdMap map[int32]string, tree map[int32][]proc.Process, gitInfo map[string]git.Info, alertMap map[string]db.Alert) {
    wPanes = sortPanes(wPanes)
    if len(wPanes) == 0 {
        return
    }
    winCWD := wPanes[0].CWD

    p.nodes = append(p.nodes, ProcListNode{
        IsWindowHeader: true,
        Pane:           wPanes[0],
        Alert:          windowAlertFromMap(alertMap, session, wi),
    })

    for _, pane := range wPanes {
        displayPane := pane
        if pane.CWD == winCWD {
            displayPane.CWD = ""
        }
        p.appendPaneNodes(pane, displayPane, winCWD, procs, cwdMap, tree, gitInfo, alertMap)
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
