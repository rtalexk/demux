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


type ProcListNode struct {
    IsPaneHeader bool
    IsIdle       bool // placeholder row shown when a pane has no processes
    Pane         tmux.Pane
    GitDeviant   bool
    GitInfo      git.Info
    Proc         proc.Process
    Port         int
    Depth        int // 0=pane header, 1=process, 2=subprocess
}

type ProcListModel struct {
    nodes       []ProcListNode
    cursor      int
    offset      int // viewport scroll offset (by node index)
    filterText  string
    primaryCWD  string
    curSession  string
    curWindow   int
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

    windowChanged := session != p.curSession || windowIndex != p.curWindow
    p.curSession = session
    p.curWindow = windowIndex
    p.nodes = nil
    if windowChanged {
        p.cursor = 0
        p.offset = 0
    }

    for _, pane := range sortPanes(wPanes) {
        paneCWD := pane.CWD
        gitKey := fmt.Sprintf("%s:%d:%d", pane.Session, pane.WindowIndex, pane.PaneIndex)
        info := gitInfo[gitKey]
        deviant := p.primaryCWD != "" && !git.IsDescendant(paneCWD, p.primaryCWD) && paneCWD != p.primaryCWD

        headerIdx := len(p.nodes)
        p.nodes = append(p.nodes, ProcListNode{
            IsPaneHeader: true,
            Pane:         pane,
            GitDeviant:   deviant,
            GitInfo:      info,
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
        for _, pr := range children {
            if seen[pr.PID] {
                continue
            }
            seen[pr.PID] = true
            p.nodes = append(p.nodes, ProcListNode{Proc: pr, Depth: 1})
            for _, grandchild := range tree[pr.PID] {
                if !seen[grandchild.PID] {
                    seen[grandchild.PID] = true
                    p.nodes = append(p.nodes, ProcListNode{Proc: grandchild, Depth: 2})
                }
            }
        }
        if len(p.nodes) == headerIdx+1 {
            // no children were added — insert an idle placeholder at process depth
            p.nodes = append(p.nodes, ProcListNode{IsIdle: true, Depth: 1})
        }
    }
}

func sortPanes(panes []tmux.Pane) []tmux.Pane {
    sorted := make([]tmux.Pane, len(panes))
    copy(sorted, panes)
    sort.Slice(sorted, func(i, j int) bool { return sorted[i].PaneIndex < sorted[j].PaneIndex })
    return sorted
}

// CurrentWindow returns the session name and window index currently displayed.
func (p ProcListModel) CurrentWindow() (string, int) {
    return p.curSession, p.curWindow
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
        inner := noSelectionStyle.Render(hint)
        return border.Width(width - 2).Height(height - 2).Render(inner)
    }
    filter := strings.ToLower(p.filterText)

    // build the full rendered line list (respecting filter), tracking node index
    type renderedLine struct {
        nodeIdx int
        text    string
    }
    var allLines []renderedLine
    for i, node := range p.nodes {
        selected := focused && i == p.cursor
        var line string
        if node.IsPaneHeader {
            line = p.renderPaneHeader(node, selected)
        } else if node.IsIdle {
            line = paneIdleStyle.Render("    idle")
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
    label := fmt.Sprintf("pane %d", node.Pane.PaneIndex)
    if selected {
        text := label
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
        return selectedBG.Render(text)
    }
    out := paneHeaderStyle.Render(label)
    if node.Pane.CWD != "" {
        out += "  " + panePathStyle.Render(node.Pane.CWD)
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

var procEditors = []string{"nvim", "vim", "vi", "nano", "emacs", "hx", "micro", "helix"}
var procAgents = []string{"claude", "aider", "cursor", "copilot", "continue", "cody"}
var procServers = []string{
    "railway", "rails", "node", "deno", "bun",
    "python", "python3", "uvicorn", "gunicorn", "fastapi", "django", "flask",
    "cargo", "go", "air", "watchexec",
    "vite", "webpack", "next", "nuxt",
    "caddy", "nginx", "httpd",
}
var procShells = []string{"zsh", "bash", "sh", "fish", "dash", "nu", "pwsh"}

// procNameStyle returns the appropriate lipgloss style for a process name
// based on its type and tree depth.
func procNameStyle(pr proc.Process, depth int) lipgloss.Style {
    if depth >= 2 {
        return lipgloss.NewStyle().Foreground(activeTheme.ColorProcChild)
    }
    name := strings.ToLower(pr.FriendlyName())
    switch {
    case containsStr(procEditors, name):
        return lipgloss.NewStyle().Foreground(activeTheme.ColorProcEditor)
    case containsStr(procAgents, name) || strings.HasPrefix(name, "claude-"):
        return lipgloss.NewStyle().Foreground(activeTheme.ColorProcClaude)
    case containsStr(procServers, name):
        return lipgloss.NewStyle().Foreground(activeTheme.ColorProcServer)
    case containsStr(procShells, name):
        return lipgloss.NewStyle().Foreground(activeTheme.ColorFgSubtext)
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

func (p ProcListModel) renderProc(node ProcListNode, selected bool) string {
    pr := node.Proc
    indent := "  " + strings.Repeat("  ", node.Depth) // 2-space base + depth indent

    // line 1: name  pid:N  :port
    name := indent + pr.FriendlyName()
    var line1 string
    if selected {
        line1 = selectedBG.Render(name)
    } else {
        line1 = procNameStyle(pr, node.Depth).Render(name)
    }
    if pr.PID > 0 {
        line1 += "  " + statLabelStyle.Render(fmt.Sprintf("pid:%d", pr.PID))
    }
    if node.Port > 0 {
        line1 += "  " + statValueStyle.Render(fmt.Sprintf(":%d", node.Port))
    }

    // line 2: cpu:V  mem:V  up:V  (labels dim, values muted)
    statsIndent := "    " + strings.Repeat("  ", node.Depth)
    l := statLabelStyle.Render
    v := statValueStyle.Render
    line2 := statsIndent +
        l("cpu:") + v(fmt.Sprintf("%.1f%%", pr.CPU)) + "  " +
        l("mem:") + v(fmt.Sprintf("%.1fMB", float64(pr.MemRSS)/1024/1024)) + "  " +
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
    return &n
}
