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

        // find processes for this pane by CWD match (exact or descendant), include subprocesses as grandchildren
        for _, pr := range procs {
            cwd, ok := cwdMap[pr.PID]
            if !ok || (cwd != paneCWD && !git.IsDescendant(cwd, paneCWD)) {
                continue
            }
            p.nodes = append(p.nodes, ProcListNode{Proc: pr, Depth: 1})
            // add subprocesses (grandchildren)
            for _, child := range tree[pr.PID] {
                p.nodes = append(p.nodes, ProcListNode{Proc: child, Depth: 2})
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

    filter := strings.ToLower(p.filterText)
    var lines []string

    for i, node := range p.nodes {
        selected := i == p.cursor
        var line string
        if node.IsPaneHeader {
            line = p.renderPaneHeader(node, selected)
        } else {
            line = p.renderProc(node, selected)
        }
        if filter == "" || strings.Contains(strings.ToLower(stripANSI(line)), filter) {
            lines = append(lines, line)
        }
    }

    inner := strings.Join(lines, "\n")
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

// MoveUp moves the cursor to the previous sibling at the same depth level.
// For depth 0 (pane headers): moves to previous pane header.
// For depth 1/2: moves to previous node of the same depth without crossing a pane header.
func (p *ProcListModel) MoveUp() {
    if len(p.nodes) == 0 {
        return
    }
    depth := nodeDepth(p.nodes[p.cursor])
    for i := p.cursor - 1; i >= 0; i-- {
        d := nodeDepth(p.nodes[i])
        if depth == 0 {
            // only stop at other pane headers
            if p.nodes[i].IsPaneHeader {
                p.cursor = i
                return
            }
        } else {
            // stop at a pane header (don't cross into another pane's scope)
            if p.nodes[i].IsPaneHeader {
                return
            }
            // for depth 2, stop at a depth-1 node (don't cross parent boundary)
            if depth == 2 && d == 1 {
                return
            }
            if d == depth {
                p.cursor = i
                return
            }
        }
    }
}

// MoveDown moves the cursor to the next sibling at the same depth level.
// For depth 0 (pane headers): moves to next pane header.
// For depth 1/2: moves to next node of the same depth without crossing a pane header.
func (p *ProcListModel) MoveDown() {
    if len(p.nodes) == 0 {
        return
    }
    depth := nodeDepth(p.nodes[p.cursor])
    for i := p.cursor + 1; i < len(p.nodes); i++ {
        d := nodeDepth(p.nodes[i])
        if depth == 0 {
            // only stop at other pane headers
            if p.nodes[i].IsPaneHeader {
                p.cursor = i
                return
            }
        } else {
            // stop at a pane header (don't cross into another pane's scope)
            if p.nodes[i].IsPaneHeader {
                return
            }
            // for depth 2, stop at a depth-1 node (don't cross parent boundary)
            if depth == 2 && d == 1 {
                return
            }
            if d == depth {
                p.cursor = i
                return
            }
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
