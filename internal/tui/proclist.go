package tui

import (
    "fmt"
    "sort"
    "strings"

    "github.com/rtalex/demux/internal/config"
    "github.com/rtalex/demux/internal/db"
    "github.com/rtalex/demux/internal/git"
    "github.com/rtalex/demux/internal/proc"
    "github.com/rtalex/demux/internal/query"
    "github.com/rtalex/demux/internal/tmux"
)

type ProcListNode struct {
    IsPaneHeader bool
    IsIdle         bool // placeholder row shown when a pane has no processes
    IsWindowHeader bool // true for window-level header rows in session mode
    Pane         tmux.Pane
    GitDeviant   bool
    GitInfo      git.Info
    Alert        *db.Alert
    Proc         proc.Process
    Port         int
    Depth        int // 0=pane header, 1=process, 2=subprocess
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
    nodes         []ProcListNode
    cursor        int
    offset        int // viewport scroll offset (by node index)
    primaryCWD    string
    curSession    string
    curWindow     int
    collapsedPIDs   map[int32]bool // persists collapse state across SetWindowData calls
    pendingSeekKey  string         // node identity to restore cursor after next rebuild
    inSessionMode   bool           // true when displaying all windows of a session
    cfg             config.Config
    searchQuery     query.ParsedQuery
    queryResult     query.Result
}

// SetWindowData rebuilds the node list from pre-fetched data.
// procs is the process snapshot, cwdMap maps PID to CWD (pre-fetched),
// gitInfo is keyed by "session:windowIndex:paneIndex" for deviant panes, and
// alertMap is keyed by "session:windowIndex.paneIndex" for pane-level alerts.
func (p *ProcListModel) SetWindowData(panes []tmux.Pane, session string, windowIndex int, procs []proc.Process, cwdMap map[int32]string, gitInfo map[string]git.Info, alertMap map[string]db.Alert, cfg config.Config) {
    p.cfg = cfg
    p.inSessionMode = false
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
        paneCWD := pane.CWD
        gitKey := fmt.Sprintf("%s:%d:%d", pane.Session, pane.WindowIndex, pane.PaneIndex)
        info := gitInfo[gitKey]
        deviant := p.primaryCWD != "" && !git.IsDescendant(paneCWD, p.primaryCWD) && paneCWD != p.primaryCWD

        headerIdx := len(p.nodes)
        var paneAlert *db.Alert
        if alertMap != nil {
            paneKey := fmt.Sprintf("%s:%d.%d", pane.Session, pane.WindowIndex, pane.PaneIndex)
            if a, ok := alertMap[paneKey]; ok {
                paneAlert = &a
            }
        }
        p.nodes = append(p.nodes, ProcListNode{
            IsPaneHeader: true,
            Pane:         pane,
            GitDeviant:   deviant,
            GitInfo:      info,
            Alert:        paneAlert,
        })

        // collect depth-1 children of the pane's shell process
        seen := make(map[int32]bool)
        var children []proc.Process
        if pane.PanePID != 0 {
            // PID-based: direct children of the shell
            children = tree[pane.PanePID]
        } else {
            // fallback: CWD-based match for panes without a known PID
            for _, pr := range procs {
                cwd, ok := cwdMap[pr.PID]
                if !ok || (cwd != paneCWD && !git.IsDescendant(cwd, paneCWD)) {
                    continue
                }
                children = append(children, pr)
            }
        }
        var addProc func(pr proc.Process, depth int)
        addProc = func(pr proc.Process, depth int) {
            if seen[pr.PID] {
                return
            }
            seen[pr.PID] = true
            if containsStr(activeIgnoredProcs, strings.ToLower(pr.FriendlyName())) {
                // skip shell — promote its children to the same depth
                for _, child := range tree[pr.PID] {
                    addProc(child, depth)
                }
                return
            }

            // For depth-1 nodes: compute collapse metadata
            var hasChildren bool
            var aggCPU float64
            var aggMem uint64
            var collapsed bool
            if depth == 1 {
                for _, child := range tree[pr.PID] {
                    if !containsStr(activeIgnoredProcs, strings.ToLower(child.FriendlyName())) {
                        hasChildren = true
                        break
                    }
                }
                aggCPU, aggMem = aggStats(pr, tree)
                if _, ok := p.collapsedPIDs[pr.PID]; !ok {
                    p.collapsedPIDs[pr.PID] = true
                }
                collapsed = p.collapsedPIDs[pr.PID]
            }

            p.nodes = append(p.nodes, ProcListNode{
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
        if len(p.nodes) == headerIdx+1 {
            // no children were added — insert an idle placeholder at process depth
            p.nodes = append(p.nodes, ProcListNode{IsIdle: true, Depth: 1})
        }
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
        wPanes := sortPanes(windows[wi])
        if len(wPanes) == 0 {
            continue
        }
        winCWD := wPanes[0].CWD

        p.nodes = append(p.nodes, ProcListNode{
            IsWindowHeader: true,
            Pane:           wPanes[0],
            Alert:          windowAlertFromMap(alertMap, session, wi),
        })

        for _, pane := range wPanes {
            paneCWD := pane.CWD
            gitKey := fmt.Sprintf("%s:%d:%d", pane.Session, pane.WindowIndex, pane.PaneIndex)
            info := gitInfo[gitKey]
            deviant := winCWD != "" && !git.IsDescendant(paneCWD, winCWD) && paneCWD != winCWD

            displayPane := pane
            if paneCWD == winCWD {
                displayPane.CWD = ""
            }

            headerIdx := len(p.nodes)
            var paneAlert *db.Alert
            if alertMap != nil {
                paneKey := fmt.Sprintf("%s:%d.%d", pane.Session, pane.WindowIndex, pane.PaneIndex)
                if a, ok := alertMap[paneKey]; ok {
                    paneAlert = &a
                }
            }
            p.nodes = append(p.nodes, ProcListNode{
                IsPaneHeader: true,
                Pane:         displayPane,
                GitDeviant:   deviant,
                GitInfo:      info,
                Alert:        paneAlert,
            })

            seen := make(map[int32]bool)
            var children []proc.Process
            if pane.PanePID != 0 {
                children = tree[pane.PanePID]
            } else {
                for _, pr := range procs {
                    cwd, ok := cwdMap[pr.PID]
                    if !ok || (cwd != paneCWD && !git.IsDescendant(cwd, paneCWD)) {
                        continue
                    }
                    children = append(children, pr)
                }
            }
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
                    for _, child := range tree[pr.PID] {
                        if !containsStr(activeIgnoredProcs, strings.ToLower(child.FriendlyName())) {
                            hasChildren = true
                            break
                        }
                    }
                    aggCPU, aggMem = aggStats(pr, tree)
                    if _, ok := p.collapsedPIDs[pr.PID]; !ok {
                        p.collapsedPIDs[pr.PID] = true
                    }
                    collapsed = p.collapsedPIDs[pr.PID]
                }
                p.nodes = append(p.nodes, ProcListNode{
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
            if len(p.nodes) == headerIdx+1 {
                p.nodes = append(p.nodes, ProcListNode{IsIdle: true, Depth: 1})
            }
        }
    }
    assignTreePrefixes(p.nodes)
    p.applyPendingSeek()
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
