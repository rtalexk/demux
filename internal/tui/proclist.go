package tui

import (
    "fmt"
    "sort"
    "strings"
    "time"

    "github.com/charmbracelet/lipgloss"
    "github.com/rtalex/demux/internal/config"
    "github.com/rtalex/demux/internal/git"
    "github.com/rtalex/demux/internal/proc"
    "github.com/rtalex/demux/internal/tmux"
)

var (
    procBorderActive   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62"))
    procBorderInactive = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("244"))
    paneHeaderStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("33"))
    procLine1Style     = lipgloss.NewStyle().PaddingLeft(2)
    procLine2Style     = lipgloss.NewStyle().PaddingLeft(4).Foreground(lipgloss.Color("240"))
)

type ProcListNode struct {
    IsPaneHeader bool
    Pane         tmux.Pane
    GitDeviant   bool
    GitInfo      git.Info
    Proc         proc.Process
    Port         int
    Depth        int // 0=pane header, 1=process, 2=subprocess
}

type ProcListModel struct {
    nodes      []ProcListNode
    cursor     int
    offset     int // viewport scroll offset (by node index)
    filterText string
    primaryCWD string
}

// SetWindowData rebuilds the node list from pre-fetched data.
// procs is the process snapshot, cwdMap maps PID to CWD (pre-fetched), and
// gitInfo is keyed by "session:windowIndex:paneIndex" for deviant panes.
func (p *ProcListModel) SetWindowData(panes []tmux.Pane, session string, windowIndex int, procs []proc.Process, cwdMap map[int32]string, gitInfo map[string]git.Info, cfg config.Config) {
    grouped := tmux.GroupBySessions(panes)
    windows := grouped[session]
    p.primaryCWD = primaryCWDForPanes(windows)
    wPanes := windows[windowIndex]

    tree := proc.BuildTree(procs)

    // build a set of all PIDs that match any pane in this window, so we can
    // detect parent-child relationships within the match set
    matchSet := make(map[int32]bool)
    for _, pane := range wPanes {
        for _, pr := range procs {
            cwd, ok := cwdMap[pr.PID]
            if !ok {
                continue
            }
            if cwd == pane.CWD || git.IsDescendant(cwd, pane.CWD) {
                matchSet[pr.PID] = true
            }
        }
    }

    // globalSeen prevents the same PID from appearing under multiple panes
    globalSeen := make(map[int32]bool)

    p.nodes = nil
    for _, pane := range sortPanes(wPanes) {
        paneCWD := pane.CWD
        gitKey := fmt.Sprintf("%s:%d:%d", pane.Session, pane.WindowIndex, pane.PaneIndex)
        info := gitInfo[gitKey]
        deviant := p.primaryCWD != "" && !git.IsDescendant(paneCWD, p.primaryCWD) && paneCWD != p.primaryCWD

        p.nodes = append(p.nodes, ProcListNode{
            IsPaneHeader: true,
            Pane:         pane,
            GitDeviant:   deviant,
            GitInfo:      info,
        })

        for _, pr := range procs {
            if globalSeen[pr.PID] {
                continue
            }
            cwd, ok := cwdMap[pr.PID]
            if !ok || (cwd != paneCWD && !git.IsDescendant(cwd, paneCWD)) {
                continue
            }
            // skip if this process's parent is also in the match set — it will
            // appear as a depth-2 child under its parent instead
            if matchSet[pr.PPID] {
                continue
            }
            globalSeen[pr.PID] = true
            p.nodes = append(p.nodes, ProcListNode{Proc: pr, Depth: 1})
            for _, child := range tree[pr.PID] {
                if !globalSeen[child.PID] {
                    globalSeen[child.PID] = true
                    p.nodes = append(p.nodes, ProcListNode{Proc: child, Depth: 2})
                }
            }
        }
    }
}

func sortPanes(panes []tmux.Pane) []tmux.Pane {
    sorted := make([]tmux.Pane, len(panes))
    copy(sorted, panes)
    sort.Slice(sorted, func(i, j int) bool { return sorted[i].PaneIndex < sorted[j].PaneIndex })
    return sorted
}

func (p *ProcListModel) SetFilter(text string) {
    p.filterText = text
}

func (p ProcListModel) Render(width, height int, focused bool) string {
    border := procBorderInactive
    if focused {
        border = procBorderActive
    }

    if len(p.nodes) == 0 {
        hint := "Select a window with Enter"
        inner := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true).Render(hint)
        return border.Width(width - 2).Height(height - 2).Render(inner)
    }

    hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
    filter := strings.ToLower(p.filterText)

    // build the full rendered line list (respecting filter), tracking node index
    type renderedLine struct {
        nodeIdx int
        text    string
    }
    var allLines []renderedLine
    for i, node := range p.nodes {
        selected := i == p.cursor
        var line string
        if node.IsPaneHeader {
            line = p.renderPaneHeader(node, selected)
        } else {
            line = p.renderProc(node, selected)
        }
        if filter != "" && !strings.Contains(strings.ToLower(stripANSI(line)), filter) {
            continue
        }
        allLines = append(allLines, renderedLine{nodeIdx: i, text: line})
    }

    // clamp offset so cursor's line is visible
    offset := p.offset
    if p.cursor < offset {
        offset = p.cursor
    }

    // determine scroll hints based on node-level offset
    hasAbove := offset > 0
    hasBelow := false // determined after we know how many fit

    maxRows := height - 2
    if maxRows < 1 {
        maxRows = 1
    }
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
    return border.Width(width - 2).Height(height - 2).Render(inner)
}

func (p ProcListModel) renderPaneHeader(node ProcListNode, selected bool) string {
    text := fmt.Sprintf("pane %d", node.Pane.PaneIndex)
    if node.Pane.CWD != "" {
        text += "  " + node.Pane.CWD
    }
    if node.GitDeviant {
        if node.GitInfo.Loading {
            text += "  ↪ …"
        } else {
            text += "  ↪ " + compactGitIndicators(node.GitInfo)
        }
    }
    line := paneHeaderStyle.Render(text)
    if selected {
        return selectedBG.Render(text)
    }
    return line
}

func (p ProcListModel) renderProc(node ProcListNode, selected bool) string {
    pr := node.Proc
    indent := strings.Repeat("  ", node.Depth)
    line1 := indent + pr.Name
    if pr.PID > 0 {
        line1 += fmt.Sprintf("  pid:%d", pr.PID)
    }
    if node.Port > 0 {
        line1 += fmt.Sprintf("  :%d", node.Port)
    }
    line2 := fmt.Sprintf("cpu:%.1f%%  mem:%.1fMB  up:%s",
        pr.CPU,
        float64(pr.MemRSS)/1024/1024,
        formatProcDuration(pr.Uptime),
    )

    l1 := procLine1Style.Render(line1)
    l2 := procLine2Style.Render(strings.Repeat("  ", node.Depth) + line2)
    if selected {
        l1 = selectedBG.Render(line1)
    }
    return l1 + "\n" + l2
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

// nodeDepth returns the logical depth of a node: 0 for pane headers, otherwise Depth.
func nodeDepth(n ProcListNode) int {
    if n.IsPaneHeader {
        return 0
    }
    return n.Depth
}

// clampOffset adjusts the viewport offset so the cursor is always visible.
// visibleNodes is the number of node slots available (accounting for hint rows).
func (p *ProcListModel) clampOffset(visibleNodes int) {
    effective := visibleNodes - 2
    if effective < 1 {
        effective = 1
    }
    if p.cursor < p.offset {
        p.offset = p.cursor
    }
    if p.cursor >= p.offset+effective {
        p.offset = p.cursor - effective + 1
    }
    if p.offset < 0 {
        p.offset = 0
    }
}

// MoveUp moves the cursor one item up (linear navigation).
func (p *ProcListModel) MoveUp() {
    if p.cursor > 0 {
        p.cursor--
    }
}

// MoveDown moves the cursor one item down (linear navigation).
func (p *ProcListModel) MoveDown() {
    if p.cursor < len(p.nodes)-1 {
        p.cursor++
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
            if n.IsPaneHeader {
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
        if nodeDepth(p.nodes[i]) == depth {
            peers = append(peers, i)
        }
    }
    return peers
}

func (p *ProcListModel) JumpToNextPane() {
    for i := p.cursor + 1; i < len(p.nodes); i++ {
        if p.nodes[i].IsPaneHeader {
            p.cursor = i
            return
        }
    }
}

func (p *ProcListModel) JumpToPrevPane() {
    for i := p.cursor - 1; i >= 0; i-- {
        if p.nodes[i].IsPaneHeader {
            p.cursor = i
            return
        }
    }
}

func (p ProcListModel) SelectedNode() *ProcListNode {
    if p.cursor < 0 || p.cursor >= len(p.nodes) {
        return nil
    }
    n := p.nodes[p.cursor]
    return &n
}
