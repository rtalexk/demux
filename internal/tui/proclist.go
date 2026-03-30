package tui

import (
    "fmt"
    "sort"
    "strings"
    "time"

    "github.com/charmbracelet/lipgloss"
    "github.com/rtalex/demux/internal/config"
    "github.com/rtalex/demux/internal/db"
    "github.com/rtalex/demux/internal/format"
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
    p.primaryCWD = primaryCWDForPanes(windows)
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
//   "├─ " non-last sibling at this depth
//   "└─ " last sibling at this depth
//   "│  " ancestor continuation (non-last ancestor)
//   "   " ancestor continuation (last ancestor)
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

// CurrentWindow returns the session name and window index currently displayed.
func (p ProcListModel) CurrentWindow() (string, int) {
    return p.curSession, p.curWindow
}

func (p ProcListModel) Render(width, height int, focused bool, title string) string {
    border := procBorderInactive
    if focused {
        border = procBorderActive
    }
    innerW := width - 2

    // Right border title: show the primary CWD (session or window path).
    rightTitle := ""
    if p.primaryCWD != "" {
        rightTitle = " " + format.ShortenPath(p.primaryCWD, p.cfg.PathAliases) + " "
    }

    if len(p.nodes) == 0 {
        hint := "Select a window with Enter"
        inner := noSelectionStyle.Render(hint)
        return injectBorderTitles(border.Width(width-2).Height(height-2).Render(inner), title, rightTitle)
    }

    // build the full rendered line list, tracking node index
    type renderedLine struct {
        nodeIdx int
        text    string
    }
    var allLines []renderedLine
    for i, node := range p.nodes {
        selected := focused && i == p.cursor
        var line string
        if node.IsWindowHeader {
            line = p.renderWindowHeader(node, selected, innerW)
        } else if node.IsPaneHeader {
            paneInnerW := innerW
            if p.inSessionMode {
                paneInnerW -= 4
                if paneInnerW < 0 {
                    paneInnerW = 0
                }
            }
            rendered := p.renderPaneHeader(node, selected, paneInnerW)
            if p.inSessionMode {
                rendered = "    " + rendered
            }
            line = rendered
        } else if node.IsIdle {
            idleText := "    idle"
            if p.inSessionMode {
                idleText = "    " + idleText
            }
            line = paneIdleStyle.Render(idleText)
        } else {
            procInnerW := innerW
            if p.inSessionMode {
                procInnerW = innerW - 4
                if procInnerW < 0 {
                    procInnerW = 0
                }
            }
            rendered := p.renderProc(node, selected, procInnerW)
            if p.inSessionMode {
                parts := strings.SplitN(rendered, "\n", 2)
                if len(parts) == 2 {
                    rendered = "    " + parts[0] + "\n" + "    " + parts[1]
                } else {
                    rendered = "    " + rendered
                }
            }
            line = rendered
        }
        allLines = append(allLines, renderedLine{nodeIdx: i, text: line})
    }

    maxRows := height - 2
    if maxRows < 1 {
        maxRows = 1
    }

    // Safety clamps (read-only): handle cases where the viewport shrank since
    // the last clampOffset call (e.g. detail pane expanding after selection change).
    offset := p.offset
    if p.cursor < offset {
        offset = p.cursor
    }
    for offset < p.cursor && !procCursorVisible(p.nodes, p.cursor, offset, maxRows) {
        offset++
    }

    // determine scroll hints based on node-level offset
    hasAbove := offset > 0
    hasBelow := false // determined after we know how many fit
    contentRows := maxRows
    if hasAbove {
        contentRows--
    }
    // tentatively check hasBelow: collect rows from offset
    rowCount := 0
    var visible []string
    startIdx := 0
    for i, rl := range allLines {
        if rl.nodeIdx < offset {
            continue
        }
        if startIdx == 0 {
            startIdx = i
        }
        entryRows := strings.Count(rl.text, "\n") + 1
        if rowCount+entryRows > contentRows {
            hasBelow = true
            break
        }
        visible = append(visible, rl.text)
        rowCount += entryRows
    }
    // if hasBelow discovered, shrink contentRows by 1 and rebuild visible
    if hasBelow {
        contentRows = maxRows
        if hasAbove {
            contentRows--
        }
        contentRows-- // for ▼ hint
        rowCount = 0
        visible = visible[:0]
        for _, rl := range allLines {
            if rl.nodeIdx < offset {
                continue
            }
            entryRows := strings.Count(rl.text, "\n") + 1
            if rowCount+entryRows > contentRows {
                break
            }
            visible = append(visible, rl.text)
            rowCount += entryRows
        }
    }

    var resultLines []string
    if hasAbove {
        resultLines = append(resultLines, hintStyle.Render("▲ more"))
    }
    resultLines = append(resultLines, visible...)
    if hasBelow {
        resultLines = append(resultLines, hintStyle.Render("▼ more"))
    }

    inner := strings.Join(resultLines, "\n")
    return injectBorderTitles(border.Width(width-2).Height(height-2).Render(inner), title, rightTitle)
}

func (p ProcListModel) renderPaneHeader(node ProcListNode, selected bool, innerW int) string {
    label := fmt.Sprintf("pane %d", node.Pane.PaneIndex)

    alertSuffix := ""
    if node.Alert != nil {
        alertSuffix = " ---- " + alertIcon(node.Alert.Level) + " " + alertBadge(node.Alert.Level, node.Alert.Reason)
    }

    pathStr := ""
    if node.Pane.CWD != "" && node.Pane.CWD != p.primaryCWD {
        pathStr = format.ShortenPath(node.Pane.CWD, p.cfg.PathAliases)
    }
    gitSuffix := ""
    if node.GitDeviant {
        if node.GitInfo.Loading {
            gitSuffix = "  ↪ …"
        } else {
            gitSuffix = "  ↪ " + stripANSI(compactGitIndicators(node.GitInfo))
        }
    }

    if selected {
        left := label + stripANSI(alertSuffix)
        rightPart := pathStr + gitSuffix
        if rightPart != "" && p.cfg.ProcessList.PathRightAlign && innerW > 0 {
            rightW := len([]rune(rightPart))
            padCount := innerW - len([]rune(left)) - rightW
            if padCount < 1 {
                padCount = 1
            }
            return selectedBG.Render(left + strings.Repeat(" ", padCount) + rightPart)
        }
        if rightPart != "" {
            content := left + "  " + rightPart
            padCount := innerW - len([]rune(content))
            if padCount < 0 {
                padCount = 0
            }
            return selectedBG.Render(content + strings.Repeat(" ", padCount))
        }
        padCount := innerW - len([]rune(left))
        if padCount < 0 {
            padCount = 0
        }
        return selectedBG.Render(left + strings.Repeat(" ", padCount))
    }

    rightPart := pathStr + gitSuffix
    if rightPart == "" || !p.cfg.ProcessList.PathRightAlign || innerW <= 0 {
        out := paneHeaderStyle.Render(label) + alertSuffix
        if pathStr != "" {
            out += "  " + panePathStyle.Render(pathStr)
        }
        if node.GitDeviant {
            if node.GitInfo.Loading {
                out += "  " + panePathStyle.Render("↪ …")
            } else {
                out += "  " + panePathStyle.Render("↪") + " " + compactGitIndicators(node.GitInfo)
            }
        }
        return out
    }

    labelW := len([]rune(label + stripANSI(alertSuffix)))
    rightW := len([]rune(rightPart))
    fillCount := innerW - labelW - 2 - 2 - rightW
    if fillCount < 1 {
        fillCount = 1
    }
    out := paneHeaderStyle.Render(label) +
        alertSuffix +
        "  " +
        paneSepStyle.Render(strings.Repeat("─", fillCount)) +
        "  " +
        panePathStyle.Render(pathStr)
    if node.GitDeviant {
        if node.GitInfo.Loading {
            out += "  " + panePathStyle.Render("↪ …")
        } else {
            out += "  " + panePathStyle.Render("↪") + " " + compactGitIndicators(node.GitInfo)
        }
    }
    return out
}

func (p ProcListModel) renderWindowHeader(node ProcListNode, selected bool, innerW int) string {
    label := fmt.Sprintf("Win %d", node.Pane.WindowIndex)
    if node.Pane.WindowName != "" {
        label = fmt.Sprintf("Win %d: %s", node.Pane.WindowIndex, node.Pane.WindowName)
    }

    alertSuffix := ""
    if node.Alert != nil {
        alertSuffix = " ---- " + alertIcon(node.Alert.Level) + " " + alertBadge(node.Alert.Level, node.Alert.Reason)
    }

    pathStr := ""
    if node.Pane.CWD != "" && node.Pane.CWD != p.primaryCWD {
        pathStr = format.ShortenPath(node.Pane.CWD, p.cfg.PathAliases)
    }

    if selected {
        left := label + stripANSI(alertSuffix)
        if pathStr != "" && p.cfg.ProcessList.PathRightAlign && innerW > 0 {
            rightW := len([]rune(pathStr))
            padCount := innerW - len([]rune(left)) - rightW
            if padCount < 1 {
                padCount = 1
            }
            return selectedBG.Render(left + strings.Repeat(" ", padCount) + pathStr)
        }
        if pathStr != "" {
            content := left + "  " + pathStr
            padCount := innerW - len([]rune(content))
            if padCount < 0 {
                padCount = 0
            }
            return selectedBG.Render(content + strings.Repeat(" ", padCount))
        }
        padCount := innerW - len([]rune(left))
        if padCount < 0 {
            padCount = 0
        }
        return selectedBG.Render(left + strings.Repeat(" ", padCount))
    }

    if pathStr == "" || !p.cfg.ProcessList.PathRightAlign || innerW <= 0 {
        out := windowHeaderStyle.Render(label) + alertSuffix
        if pathStr != "" {
            out += "  " + panePathStyle.Render(pathStr)
        }
        return out
    }

    labelW := len([]rune(label + stripANSI(alertSuffix)))
    rightW := len([]rune(pathStr))
    fillCount := innerW - labelW - 2 - 2 - rightW
    if fillCount < 1 {
        fillCount = 1
    }
    return windowHeaderStyle.Render(label) +
        alertSuffix +
        "  " +
        paneSepStyle.Render(strings.Repeat("─", fillCount)) +
        "  " +
        panePathStyle.Render(pathStr)
}

// procNameStyle returns the appropriate lipgloss style for a process name
// based on its type and tree depth.
func procNameStyle(pr proc.Process, depth int) lipgloss.Style {
    if depth >= 2 {
        return lipgloss.NewStyle().Foreground(activeTheme.ColorProcChild)
    }
    name := strings.ToLower(pr.FriendlyName())
    switch {
    case containsStr(activeProcEditors, name):
        return lipgloss.NewStyle().Foreground(activeTheme.ColorProcEditor)
    case containsStr(activeProcAgents, name) || strings.HasPrefix(name, "claude-"):
        return lipgloss.NewStyle().Foreground(activeTheme.ColorProcClaude)
    case containsStr(activeProcServers, name):
        return lipgloss.NewStyle().Foreground(activeTheme.ColorProcServer)
    default:
        return lipgloss.NewStyle().Foreground(activeTheme.ColorFgPrimary)
    }
}

func containsStr(list []string, s string) bool {
    for _, v := range list {
        if v == s {
            return true
        }
    }
    return false
}

func (p ProcListModel) renderProc(node ProcListNode, selected bool, innerW int) string {
    pr := node.Proc
    indent := node.TreePrefix

    // collapse indicator prefix for depth-1 nodes with children
    collapsePrefix := ""
    if node.Depth == 1 && node.HasChildren {
        if node.Collapsed {
            collapsePrefix = "▶ "
        } else {
            collapsePrefix = "▼ "
        }
    }

    // line 1: [indicator]name  pid:N  :port
    var line1 string
    if selected {
        plain := indent + collapsePrefix + pr.FriendlyName()
        if pr.PID > 0 {
            plain += fmt.Sprintf("  pid:%d", pr.PID)
        }
        if node.Port > 0 {
            plain += fmt.Sprintf("  :%d", node.Port)
        }
        padCount := innerW - len([]rune(plain))
        if padCount < 0 {
            padCount = 0
        }
        line1 = selectedBG.Render(plain + strings.Repeat(" ", padCount))
    } else {
        line1 = treeConnectorStyle.Render(indent) + procNameStyle(pr, node.Depth).Render(collapsePrefix+pr.FriendlyName())
        if pr.PID > 0 {
            line1 += "  " + statLabelStyle.Render(fmt.Sprintf("pid:%d", pr.PID))
        }
        if node.Port > 0 {
            line1 += "  " + statValueStyle.Render(fmt.Sprintf(":%d", node.Port))
        }
    }

    // line 2: cpu/mem stats; show aggregated totals in parens when collapsed with children
    statsIndent := treeConnectorStyle.Render(node.StatPrefix) + "  "
    l := statLabelStyle.Render
    v := statValueStyle.Render

    cpuStr := v(fmt.Sprintf("%.1f%%", pr.CPU))
    memStr := v(fmt.Sprintf("%.1fMB", float64(pr.MemRSS)/1024/1024))
    if node.Depth == 1 && node.HasChildren && node.Collapsed {
        cpuStr += v(fmt.Sprintf(" (%.1f%%)", node.AggCPU))
        memStr += v(fmt.Sprintf(" (%.1fMB)", float64(node.AggMemRSS)/1024/1024))
    }

    line2 := statsIndent +
        l("cpu:") + cpuStr + "  " +
        l("mem:") + memStr + "  " +
        l("up:") + v(formatProcDuration(pr.Uptime))

    return line1 + "\n" + line2
}

func formatProcDuration(d time.Duration) string {
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

func stripANSI(s string) string {
    var result strings.Builder
    i := 0
    for i < len(s) {
        if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
            i += 2
            for i < len(s) && s[i] != 'm' {
                i++
            }
            i++ // skip 'm'
            continue
        }
        result.WriteByte(s[i])
        i++
    }
    return result.String()
}

// injectBorderTitle splices title into the top border line of a lipgloss-rendered
// box. The title is placed immediately after the top-left corner, with the
// remaining width filled by the border's horizontal fill character.
// ANSI color codes wrapping the original top line are preserved.
func injectBorderTitle(rendered, title string) string {
    if title == "" {
        return rendered
    }
    nl := strings.IndexByte(rendered, '\n')
    if nl < 0 {
        return rendered
    }
    topLine := rendered[:nl]
    rest := rendered[nl:] // includes the leading \n

    plain := stripANSI(topLine)
    runes := []rune(plain)
    if len(runes) < 4 {
        return rendered
    }

    // runes[0]=╭, runes[1..len-2]=─ fill, runes[len-1]=╮
    cornerLeft := string(runes[0])
    cornerRight := string(runes[len(runes)-1])
    fill := string(runes[1])
    totalInner := len(runes) - 2

    titleRunes := []rune(title)
    titleVisible := []rune(stripANSI(title))
    if len(titleVisible) > totalInner-1 {
        titleRunes = titleRunes[:totalInner-1]
        titleVisible = titleVisible[:totalInner-1]
    }
    fillCount := totalInner - len(titleVisible)

    // Extract ANSI prefix (border color) and suffix (reset) from the original top line.
    cornerLeftIdx := strings.Index(topLine, cornerLeft)
    cornerRightIdx := strings.LastIndex(topLine, cornerRight)
    ansiPrefix := ""
    ansiSuffix := ""
    if cornerLeftIdx > 0 {
        ansiPrefix = topLine[:cornerLeftIdx]
    }
    if cornerRightIdx >= 0 {
        after := cornerRightIdx + len(cornerRight)
        if after <= len(topLine) {
            ansiSuffix = topLine[after:]
        }
    }

    // Color the border chars but not the title text, so the title uses the
    // terminal's default foreground rather than the border color.
    newTop := ansiPrefix + cornerLeft + ansiSuffix +
        title +
        ansiPrefix + strings.Repeat(fill, fillCount) + cornerRight + ansiSuffix

    return newTop + rest
}

// injectBorderTitles is like injectBorderTitle but also places rightTitle
// flush against the right corner of the top border line.
func injectBorderTitles(rendered, leftTitle, rightTitle string) string {
    if rightTitle == "" {
        return injectBorderTitle(rendered, leftTitle)
    }
    nl := strings.IndexByte(rendered, '\n')
    if nl < 0 {
        return rendered
    }
    topLine := rendered[:nl]
    rest := rendered[nl:]

    plain := stripANSI(topLine)
    runes := []rune(plain)
    if len(runes) < 4 {
        return rendered
    }

    cornerLeft := string(runes[0])
    cornerRight := string(runes[len(runes)-1])
    fill := string(runes[1])
    totalInner := len(runes) - 2

    leftVisible := []rune(stripANSI(leftTitle))
    rightVisible := []rune(stripANSI(rightTitle))
    totalUsed := len(leftVisible) + len(rightVisible)
    if totalUsed >= totalInner {
        return injectBorderTitle(rendered, leftTitle)
    }
    fillCount := totalInner - totalUsed

    cornerLeftIdx := strings.Index(topLine, cornerLeft)
    cornerRightIdx := strings.LastIndex(topLine, cornerRight)
    ansiPrefix := ""
    ansiSuffix := ""
    if cornerLeftIdx > 0 {
        ansiPrefix = topLine[:cornerLeftIdx]
    }
    if cornerRightIdx >= 0 {
        after := cornerRightIdx + len(cornerRight)
        if after <= len(topLine) {
            ansiSuffix = topLine[after:]
        }
    }

    newTop := ansiPrefix + cornerLeft + ansiSuffix +
        leftTitle +
        ansiPrefix + strings.Repeat(fill, fillCount) + ansiSuffix +
        rightTitle +
        ansiPrefix + cornerRight + ansiSuffix

    return newTop + rest
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
    if n.IsPaneHeader || n.IsIdle || n.IsWindowHeader {
        return 1
    }
    return 2 // process: name line + stats line
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

// cursorNodeKey returns a stable string identity for the node at the cursor so
// that applyPendingSeek can restore focus after a rebuild changes indices.
//
// Key format:
//   - window header → "win:<windowIndex>"
//   - pane header   → "pane:<windowIndex>:<paneIndex>"
//   - idle marker   → "idle:<windowIndex>:<paneIndex>"  (uses containing pane)
//   - process       → "pid:<PID>"  (for depth>1 uses the depth-1 ancestor's PID)
func (p *ProcListModel) cursorNodeKey() string {
    if p.cursor < 0 || p.cursor >= len(p.nodes) {
        return ""
    }
    n := p.nodes[p.cursor]
    switch {
    case n.IsWindowHeader:
        return fmt.Sprintf("win:%d", n.Pane.WindowIndex)
    case n.IsPaneHeader:
        return fmt.Sprintf("pane:%d:%d", n.Pane.WindowIndex, n.Pane.PaneIndex)
    case n.IsIdle:
        for i := p.cursor - 1; i >= 0; i-- {
            if p.nodes[i].IsPaneHeader {
                ph := p.nodes[i]
                return fmt.Sprintf("idle:%d:%d", ph.Pane.WindowIndex, ph.Pane.PaneIndex)
            }
        }
        return ""
    case n.Depth > 1:
        for i := p.cursor - 1; i >= 0; i-- {
            if p.nodes[i].Depth == 1 {
                return fmt.Sprintf("pid:%d", p.nodes[i].Proc.PID)
            }
        }
        return ""
    default:
        return fmt.Sprintf("pid:%d", n.Proc.PID)
    }
}

// applyPendingSeek moves the cursor to the node matching pendingSeekKey, then
// clears the field. Called at the end of each SetWindowData / SetSessionData.
func (p *ProcListModel) applyPendingSeek() {
    if p.pendingSeekKey == "" {
        return
    }
    key := p.pendingSeekKey
    p.pendingSeekKey = ""
    // Track the most recent pane header for idle-node matching.
    var curPaneWin, curPanePane int
    for i, n := range p.nodes {
        if n.IsPaneHeader {
            curPaneWin = n.Pane.WindowIndex
            curPanePane = n.Pane.PaneIndex
        }
        var nkey string
        switch {
        case n.IsWindowHeader:
            nkey = fmt.Sprintf("win:%d", n.Pane.WindowIndex)
        case n.IsPaneHeader:
            nkey = fmt.Sprintf("pane:%d:%d", n.Pane.WindowIndex, n.Pane.PaneIndex)
        case n.IsIdle:
            nkey = fmt.Sprintf("idle:%d:%d", curPaneWin, curPanePane)
        case n.Proc.PID != 0:
            nkey = fmt.Sprintf("pid:%d", n.Proc.PID)
        }
        if nkey != "" && nkey == key {
            p.cursor = i
            return
        }
    }
}

// clampOffset adjusts p.offset so the cursor node is always within the
// visible row window. maxRows is the total inner height of the proc panel
// (border already subtracted).
func (p *ProcListModel) clampOffset(maxRows int) {
    if len(p.nodes) == 0 {
        p.offset = 0
        p.cursor = 0
        return
    }
    if p.cursor >= len(p.nodes) {
        p.cursor = len(p.nodes) - 1
    }
    if p.cursor < p.offset {
        p.offset = p.cursor
    }
    for p.offset < p.cursor {
        if procCursorVisible(p.nodes, p.cursor, p.offset, maxRows) {
            break
        }
        p.offset++
    }
    if p.offset < 0 {
        p.offset = 0
    }
}

// procCursorVisible mirrors the Render hint logic to determine whether the
// cursor node would be visible with the given offset and maxRows.
//   - ▲ hint is shown when offset > 0 (costs 1 row)
//   - ▼ hint is shown when content overflows after accounting for ▲ (costs 1 row)
func procCursorVisible(nodes []ProcListNode, cursor, offset, maxRows int) bool {
    hasAbove := offset > 0
    contentRows := maxRows
    if hasAbove {
        contentRows--
    }

    // First pass: scan ALL nodes from offset (not stopping at cursor) so we
    // can detect whether nodes after the cursor would cause ▼ to appear.
    rows := 0
    cursorRows := -1 // rows consumed up to and including the cursor node
    hasBelow := false
    for i := offset; i < len(nodes); i++ {
        nr := nodeRows(nodes[i])
        if rows+nr > contentRows {
            hasBelow = true
            break
        }
        rows += nr
        if i == cursor {
            cursorRows = rows
        }
    }
    if cursorRows < 0 {
        return false // cursor not reached within contentRows
    }
    if !hasBelow {
        return true // no ▼ hint; cursor fits
    }
    // ▼ hint will be shown — re-check with one fewer row.
    rows = 0
    for i := offset; i < len(nodes); i++ {
        nr := nodeRows(nodes[i])
        if rows+nr > contentRows-1 {
            return false
        }
        rows += nr
        if i == cursor {
            return true
        }
    }
    return false
}

// MoveUp moves the cursor one item up, skipping idle placeholders.
func (p *ProcListModel) MoveUp() {
    for i := p.cursor - 1; i >= 0; i-- {
        if isSelectable(p.nodes[i]) {
            p.cursor = i
            return
        }
    }
}

// MoveDown moves the cursor one item down, skipping idle placeholders.
func (p *ProcListModel) MoveDown() {
    for i := p.cursor + 1; i < len(p.nodes); i++ {
        if isSelectable(p.nodes[i]) {
            p.cursor = i
            return
        }
    }
}

// TabNext moves to the next sibling at the same depth level, wrapping around
// within the current depth's peer set.
func (p *ProcListModel) TabNext() {
    if len(p.nodes) == 0 {
        return
    }
    depth := nodeDepth(p.nodes[p.cursor])

    // collect all sibling indices at the same depth within the same scope
    peers := p.peersAtDepth(p.cursor, depth)
    if len(peers) == 0 {
        return
    }

    // find current position among peers and advance (with wrap)
    for i, idx := range peers {
        if idx == p.cursor {
            p.cursor = peers[(i+1)%len(peers)]
            return
        }
    }
}

// peersAtDepth returns the indices of all nodes that are siblings of the node
// at pos within the same scope (pane for depth 1; parent process block for depth 2;
// all pane headers for depth 0).
func (p *ProcListModel) peersAtDepth(pos, depth int) []int {
    if depth == 0 {
        var peers []int
        for i, n := range p.nodes {
            if n.IsPaneHeader || n.IsWindowHeader {
                peers = append(peers, i)
            }
        }
        return peers
    }

    // find the scope boundaries for depth 1 or 2
    scopeStart, scopeEnd := 0, len(p.nodes)-1

    if depth == 1 {
        // scope is within the enclosing pane header section
        for i := pos - 1; i >= 0; i-- {
            if p.nodes[i].IsPaneHeader {
                scopeStart = i + 1
                break
            }
        }
        for i := pos + 1; i < len(p.nodes); i++ {
            if p.nodes[i].IsPaneHeader {
                scopeEnd = i - 1
                break
            }
        }
    } else {
        // depth == 2: scope is within the enclosing depth-1 parent process block
        for i := pos - 1; i >= 0; i-- {
            if p.nodes[i].IsPaneHeader || nodeDepth(p.nodes[i]) == 1 {
                scopeStart = i + 1
                break
            }
        }
        for i := pos + 1; i < len(p.nodes); i++ {
            if p.nodes[i].IsPaneHeader || nodeDepth(p.nodes[i]) == 1 {
                scopeEnd = i - 1
                break
            }
        }
    }

    var peers []int
    for i := scopeStart; i <= scopeEnd; i++ {
        if nodeDepth(p.nodes[i]) == depth && isSelectable(p.nodes[i]) {
            peers = append(peers, i)
        }
    }
    return peers
}

func (p *ProcListModel) JumpToNextPane() {
    for i := p.cursor + 1; i < len(p.nodes); i++ {
        if p.nodes[i].IsPaneHeader || p.nodes[i].IsWindowHeader {
            p.cursor = i
            return
        }
    }
}

func (p *ProcListModel) JumpToPrevPane() {
    for i := p.cursor - 1; i >= 0; i-- {
        if p.nodes[i].IsPaneHeader || p.nodes[i].IsWindowHeader {
            p.cursor = i
            return
        }
    }
}

func (p *ProcListModel) GotoTop() {
    p.cursor = 0
    p.offset = 0
}

func (p *ProcListModel) GotoBottom() {
    for i := len(p.nodes) - 1; i >= 0; i-- {
        if isSelectable(p.nodes[i]) {
            p.cursor = i
            return
        }
    }
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

// ToggleCollapse flips the collapsed state of the cursor node if it is a
// depth-1 process with children. Returns true if a toggle occurred.
// The caller must re-call SetWindowData to rebuild nodes after a toggle.
func (p *ProcListModel) ToggleCollapse() bool {
    if p.cursor < 0 || p.cursor >= len(p.nodes) {
        return false
    }
    n := p.nodes[p.cursor]
    if n.IsPaneHeader || n.IsIdle || n.Depth != 1 || !n.HasChildren {
        return false
    }
    if p.collapsedPIDs == nil {
        p.collapsedPIDs = make(map[int32]bool)
    }
    p.collapsedPIDs[n.Proc.PID] = !p.collapsedPIDs[n.Proc.PID]
    return true
}

// Expand expands the focused depth-1 process node if it has children and is collapsed.
// Returns true if a change occurred; the caller must re-call SetWindowData.
func (p *ProcListModel) Expand() bool {
    if p.cursor < 0 || p.cursor >= len(p.nodes) {
        return false
    }
    n := p.nodes[p.cursor]
    if n.IsPaneHeader || n.IsIdle || n.Depth != 1 || !n.HasChildren {
        return false
    }
    if p.collapsedPIDs == nil {
        p.collapsedPIDs = make(map[int32]bool)
    }
    if !p.collapsedPIDs[n.Proc.PID] {
        return false
    }
    p.collapsedPIDs[n.Proc.PID] = false
    return true
}

// Collapse collapses the focused node. For depth-1 nodes with children it collapses
// directly; for nodes at any deeper depth it walks up to the ancestor depth-1 node, moves
// the cursor there, and collapses it. Returns true if a change occurred; the caller must re-call SetWindowData.
func (p *ProcListModel) Collapse() bool {
    if p.cursor < 0 || p.cursor >= len(p.nodes) {
        return false
    }
    n := p.nodes[p.cursor]
    if n.IsPaneHeader || n.IsIdle {
        return false
    }
    if p.collapsedPIDs == nil {
        p.collapsedPIDs = make(map[int32]bool)
    }
    if n.Depth > 1 {
        // Walk up to the parent depth-1 node.
        for i := p.cursor - 1; i >= 0; i-- {
            if p.nodes[i].IsPaneHeader {
                break
            }
            if p.nodes[i].Depth == 1 {
                parent := p.nodes[i]
                if !parent.HasChildren || p.collapsedPIDs[parent.Proc.PID] {
                    return false
                }
                p.cursor = i
                p.collapsedPIDs[parent.Proc.PID] = true
                return true
            }
        }
        return false
    }
    if n.Depth != 1 || !n.HasChildren {
        return false
    }
    if p.collapsedPIDs[n.Proc.PID] {
        return false
    }
    p.collapsedPIDs[n.Proc.PID] = true
    return true
}

// ExpandAll expands all depth-1 process nodes that have children.
// Returns true if any change occurred; the caller must re-call SetWindowData.
// pendingSeekKey is set so that applyPendingSeek can restore focus after
// the rebuild changes indices.
func (p *ProcListModel) ExpandAll() bool {
    if p.collapsedPIDs == nil {
        p.collapsedPIDs = make(map[int32]bool)
    }
    p.pendingSeekKey = p.cursorNodeKey()
    changed := false
    for _, n := range p.nodes {
        if n.IsPaneHeader || n.IsIdle || n.Depth != 1 || !n.HasChildren {
            continue
        }
        if p.collapsedPIDs[n.Proc.PID] {
            p.collapsedPIDs[n.Proc.PID] = false
            changed = true
        }
    }
    return changed
}

// CollapseAll collapses all depth-1 process nodes that have children.
// Returns true if any change occurred; the caller must re-call SetWindowData.
// pendingSeekKey is set so that applyPendingSeek (called by SetWindowData) can
// restore focus to the same logical node after the rebuild changes indices.
func (p *ProcListModel) CollapseAll() bool {
    if p.collapsedPIDs == nil {
        p.collapsedPIDs = make(map[int32]bool)
    }
    p.pendingSeekKey = p.cursorNodeKey()
    changed := false
    for _, n := range p.nodes {
        if n.IsPaneHeader || n.IsIdle || n.Depth != 1 || !n.HasChildren {
            continue
        }
        if !p.collapsedPIDs[n.Proc.PID] {
            p.collapsedPIDs[n.Proc.PID] = true
            changed = true
        }
    }
    return changed
}

// SetSearchQuery is a stub for Task 9 — will dim/highlight proc list based on query results.
func (p *ProcListModel) SetSearchQuery(pq query.ParsedQuery, r query.Result) {}
