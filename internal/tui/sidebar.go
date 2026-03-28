package tui

import (
    "fmt"
    "sort"
    "strings"
    "time"

    "github.com/charmbracelet/lipgloss"
    "github.com/rtalex/demux/internal/config"
    "github.com/rtalex/demux/internal/db"
    "github.com/rtalex/demux/internal/git"
    "github.com/rtalex/demux/internal/tmux"
)

const windowIndent = "   "

type SidebarNode struct {
    Session     string
    WindowIndex int
    IsSession   bool
    Expanded    bool
}

type SidebarModel struct {
    nodes        []SidebarNode
    cursor       int
    offset       int // viewport scroll offset
    sessions     map[string]map[int][]tmux.Pane
    alerts       map[string]db.Alert
    gitInfo         map[string]git.Info
    sessionActivity map[string]time.Time
    cfg             config.Config
    filterAlerts    bool
}

func (s *SidebarModel) SetData(panes []tmux.Pane, alerts []db.Alert, gitInfo map[string]git.Info, sessionActivity map[string]time.Time, cfg config.Config) {
    s.sessions = tmux.GroupBySessions(panes)
    s.alerts = make(map[string]db.Alert, len(alerts))
    for _, a := range alerts {
        s.alerts[a.Target] = a
    }
    s.gitInfo = gitInfo
    s.sessionActivity = sessionActivity
    s.cfg = cfg

    // preserve cursor position, rebuild nodes
    selectedSession := ""
    selectedWindow := -1
    selectedIsSession := false
    if s.cursor < len(s.nodes) {
        n := s.nodes[s.cursor]
        selectedSession = n.Session
        selectedWindow = n.WindowIndex
        selectedIsSession = n.IsSession
    }

    s.rebuildNodes()

    // restore cursor — match IsSession too so window:0 doesn't collapse back to its parent session node
    for i, n := range s.nodes {
        if n.Session == selectedSession && n.WindowIndex == selectedWindow && n.IsSession == selectedIsSession {
            s.cursor = i
            break
        }
    }
    if s.cursor >= len(s.nodes) {
        s.cursor = max(0, len(s.nodes)-1)
    }
}

// alertSeverity maps alert level to a numeric priority (higher = more severe).
func alertSeverity(level string) int {
    switch level {
    case "error":
        return 2
    case "warn":
        return 1
    default:
        return 0
    }
}

// windowAlert returns the highest-severity pane-level alert for a window
// (target format "session:windowIndex.paneIndex"), or nil if none exist.
func (s *SidebarModel) windowAlert(session string, windowIndex int) *db.Alert {
    prefix := fmt.Sprintf("%s:%d.", session, windowIndex)
    var best *db.Alert
    for target, a := range s.alerts {
        if !strings.HasPrefix(target, prefix) {
            continue
        }
        a := a
        if best == nil || alertSeverity(a.Level) > alertSeverity(best.Level) ||
            (alertSeverity(a.Level) == alertSeverity(best.Level) && a.CreatedAt.After(best.CreatedAt)) {
            best = &a
        }
    }
    return best
}

// newestSessionAlert returns the most recent alert CreatedAt for a session
// (checking pane-level targets "session:window.pane"), or zero time if none.
func (s *SidebarModel) newestSessionAlert(session string) time.Time {
    var newest time.Time
    for target, a := range s.alerts {
        if strings.HasPrefix(target, session+":") || target == session {
            if a.CreatedAt.After(newest) {
                newest = a.CreatedAt
            }
        }
    }
    return newest
}

func (s *SidebarModel) rebuildNodes() {
    // preserve expanded state
    expanded := map[string]bool{}
    for _, n := range s.nodes {
        if n.IsSession {
            expanded[n.Session] = n.Expanded
        }
    }

    s.nodes = nil

    // collect non-ignored sessions
    sessions := make([]string, 0, len(s.sessions))
    for name := range s.sessions {
        ignored := false
        for _, ig := range s.cfg.IgnoredSessions {
            if ig == name {
                ignored = true
                break
            }
        }
        if !ignored {
            sessions = append(sessions, name)
        }
    }

    sortKeys := s.cfg.SessionSort
    if len(sortKeys) == 0 {
        sortKeys = []string{"priority", "last_seen", "alphabetical"}
    }
    sort.Slice(sessions, func(i, j int) bool {
        si, sj := sessions[i], sessions[j]
        for _, key := range sortKeys {
            switch key {
            case "priority":
                ti := s.newestSessionAlert(si)
                tj := s.newestSessionAlert(sj)
                hasI := !ti.IsZero()
                hasJ := !tj.IsZero()
                if hasI != hasJ {
                    return hasI
                }
                if hasI && hasJ && !ti.Equal(tj) {
                    return ti.After(tj)
                }
            case "last_seen":
                ai := s.sessionActivity[si]
                aj := s.sessionActivity[sj]
                if !ai.Equal(aj) {
                    return ai.After(aj)
                }
            case "alphabetical":
                return si < sj
            }
        }
        return false
    })

    for _, name := range sessions {
        if s.filterAlerts && s.newestSessionAlert(name).IsZero() {
            continue
        }
        exp, ok := expanded[name]
        if !ok {
            exp = true // default expanded
        }
        s.nodes = append(s.nodes, SidebarNode{Session: name, IsSession: true, Expanded: exp})
        if exp {
            windows := s.sessions[name]
            winIdxs := make([]int, 0, len(windows))
            for wi := range windows {
                winIdxs = append(winIdxs, wi)
            }

            // sort: windows with alerts first (highest severity, then newest), then by index
            sort.Slice(winIdxs, func(i, j int) bool {
                ai := s.windowAlert(name, winIdxs[i])
                aj := s.windowAlert(name, winIdxs[j])
                hasi := ai != nil
                hasj := aj != nil
                if hasi != hasj {
                    return hasi
                }
                if hasi && hasj && !ai.CreatedAt.Equal(aj.CreatedAt) {
                    return ai.CreatedAt.After(aj.CreatedAt)
                }
                return winIdxs[i] < winIdxs[j]
            })

            for _, wi := range winIdxs {
                if s.filterAlerts && s.cfg.AlertFilterWindows == "alerts_only" {
                    if s.windowAlert(name, wi) == nil {
                        continue
                    }
                }
                s.nodes = append(s.nodes, SidebarNode{Session: name, WindowIndex: wi})
            }
        }
    }
}

func (s SidebarModel) Render(width, height int, focused bool, title, rightTitle string) string {
    visibleRows := height - 2
    if visibleRows < 1 {
        visibleRows = 1
    }

    // compute display offset without mutating (Bubbletea passes model by value in View)
    offset := s.offset
    if s.cursor < offset {
        offset = s.cursor
    }
    if s.cursor >= offset+visibleRows {
        offset = s.cursor - visibleRows + 1
    }
    if offset < 0 {
        offset = 0
    }

    hasAbove := offset > 0

    // each hint costs one row; deduct ▲ first, then check ▼ against the
    // reduced budget so hasBelow is accurate when scrolled down.
    contentRows := visibleRows
    if hasAbove {
        contentRows--
    }
    hasBelow := offset+contentRows < len(s.nodes)
    if hasBelow {
        contentRows--
    }
    if contentRows < 1 {
        contentRows = 1
    }

    // inner content width (border takes 2)
    innerW := width - 2
    centeredHint := func(text string) string {
        pad := (innerW - len([]rune(text))) / 2
        if pad < 0 {
            pad = 0
        }
        return hintStyle.Render(strings.Repeat(" ", pad) + text)
    }

    var lines []string
    if len(s.nodes) == 0 && s.filterAlerts {
        lines = append(lines, centeredHint("no alerts"))
    } else {
        if hasAbove {
            lines = append(lines, centeredHint("▲ more"))
        }
        end := offset + contentRows
        if end > len(s.nodes) {
            end = len(s.nodes)
        }
        for i := offset; i < end; i++ {
            lines = append(lines, s.renderNode(s.nodes[i], i == s.cursor, focused, width))
        }
        if hasBelow {
            lines = append(lines, centeredHint("▼ more"))
        }
    }

    inner := strings.Join(lines, "\n")
    style := borderInactive
    if focused {
        style = borderActive
    }
    return injectBorderTitles(style.Width(width-2).Height(height-2).Render(inner), title, rightTitle)
}

func (s SidebarModel) renderNode(node SidebarNode, selected, focused bool, width int) string {
    if node.IsSession {
        return s.renderSession(node, selected, focused, width)
    }
    return s.renderWindow(node, selected, focused, width)
}

// alignedRow builds a single sidebar line with the name on the left and
// indicators right-aligned to availWidth. Both name and indicators are
// measured by rune count after stripping ANSI codes.
func alignedRow(name, indicators string, availWidth int) string {
    nameW := len([]rune(stripANSI(name)))
    indW := len([]rune(stripANSI(indicators)))
    pad := availWidth - nameW - indW
    if pad < 1 {
        pad = 1
    }
    return name + strings.Repeat(" ", pad) + indicators
}

func (s SidebarModel) renderSession(node SidebarNode, selected, focused bool, width int) string {
    prefix := " ▼ "
    if !node.Expanded {
        prefix = " ▶ "
    }

    // Build indicators; when focused-selected, bake in the BG colour so inner
    // ANSI resets don't strip the row background mid-line.
    var indParts []string
    if info, ok := s.gitInfo[node.Session]; ok {
        if selected && focused {
            if ind := compactGitIndicatorsOnBG(info, activeTheme.ColorSelected); ind != "" {
                indParts = append(indParts, ind)
            }
        } else {
            if ind := compactGitIndicators(info); ind != "" {
                indParts = append(indParts, ind)
            }
        }
    }
    for target, a := range s.alerts {
        if strings.HasPrefix(target, node.Session+":") {
            if selected && focused {
                indParts = append(indParts, alertIconOnBG(a.Level, activeTheme.ColorSelected))
            } else {
                indParts = append(indParts, alertIcon(a.Level))
            }
            break
        }
    }
    indicators := strings.Join(indParts, " ")

    availW := width - 4
    indW := len([]rune(stripANSI(indicators)))
    maxName := availW - indW - 1
    if maxName < 4 {
        maxName = 4
    }
    nameRunes := []rune(prefix + node.Session)
    if len(nameRunes) > maxName {
        nameRunes = append(nameRunes[:maxName-1], '…')
    }
    nameStr := string(nameRunes)

    if selected && focused {
        pad := availW - len([]rune(nameStr)) - indW
        if pad < 0 {
            pad = 0
        }
        trail := lipgloss.NewStyle().Background(activeTheme.ColorSelected).Render("  ")
        return selectedBG.Bold(true).Render(nameStr+strings.Repeat(" ", pad)) + indicators + trail
    }
    text := alignedRow(nameStr, indicators, availW)
    if selected {
        return selectedInactive.Bold(true).Render(text)
    }
    return sessionStyle.Render(text)
}

func (s SidebarModel) renderWindow(node SidebarNode, selected, focused bool, width int) string {
    windows := s.sessions[node.Session]
    primaryCWD := primaryCWDForPanes(windows)
    wPanes := windows[node.WindowIndex]

    name := fmt.Sprintf("%d", node.WindowIndex)
    if len(wPanes) > 0 && wPanes[0].WindowName != "" {
        name = fmt.Sprintf("%d: %s", node.WindowIndex, wPanes[0].WindowName)
    }

    // build indicators
    var indParts []string
    winCWD := windowCWDFromPanes(wPanes)
    if winCWD != "" && !git.IsDescendant(winCWD, primaryCWD) && winCWD != primaryCWD {
        gitKey := fmt.Sprintf("%s:%d", node.Session, node.WindowIndex)
        devInd := "↪"
        if info, ok := s.gitInfo[gitKey]; ok {
            if selected && focused {
                if ind := compactGitIndicatorsOnBG(info, activeTheme.ColorSelected); ind != "" {
                    devInd += " " + ind
                }
            } else {
                if ind := compactGitIndicators(info); ind != "" {
                    devInd += " " + ind
                }
            }
        }
        indParts = append(indParts, devInd)
    }
    if a := s.windowAlert(node.Session, node.WindowIndex); a != nil {
        if selected && focused {
            indParts = append(indParts, alertIconOnBG(a.Level, activeTheme.ColorSelected))
        } else {
            indParts = append(indParts, alertIcon(a.Level))
        }
    }
    indicators := strings.Join(indParts, " ")

    availW := width - 4
    indW := len([]rune(stripANSI(indicators)))
    maxName := availW - indW - 1
    if maxName < 4 {
        maxName = 4
    }
    nameRunes := []rune(windowIndent + name)
    if len(nameRunes) > maxName {
        nameRunes = append(nameRunes[:maxName-1], '…')
    }
    nameStr := string(nameRunes)

    if selected {
        nameRunes2 := []rune(nameStr)
        bodyStr := string(nameRunes2[1:])
        pad := availW - 1 - len(nameRunes2[1:]) - indW
        if pad < 0 {
            pad = 0
        }
        if focused {
            accent := lipgloss.NewStyle().Foreground(activeTheme.ColorSession).Background(activeTheme.ColorSelected).Render("▌")
            trail := lipgloss.NewStyle().Background(activeTheme.ColorSelected).Render("  ")
            return accent + selectedBG.Render(bodyStr+strings.Repeat(" ", pad)) + indicators + trail
        }
        accent := lipgloss.NewStyle().Foreground(activeTheme.ColorSession).Render("▌")
        return accent + selectedInactive.Render(bodyStr+strings.Repeat(" ", pad)) + indicators
    }
    return alignedRow(nameStr, indicators, availW)
}

func compactGitIndicators(info git.Info) string {
    var parts []string
    if info.Ahead > 0 {
        parts = append(parts, gitAheadStyle.Render(fmt.Sprintf("↑%d", info.Ahead)))
    }
    if info.Behind > 0 {
        parts = append(parts, gitBehindStyle.Render(fmt.Sprintf("↓%d", info.Behind)))
    }
    if info.Dirty {
        parts = append(parts, gitDirtyStyle.Render("*"))
    }
    return strings.Join(parts, " ")
}

// compactGitIndicatorsOnBG renders git indicators with bg baked into each
// piece so that inner ANSI resets don't strip a parent background colour.
func compactGitIndicatorsOnBG(info git.Info, bg lipgloss.Color) string {
    var parts []string
    if info.Ahead > 0 {
        parts = append(parts, gitAheadStyle.Background(bg).Render(fmt.Sprintf("↑%d", info.Ahead)))
    }
    if info.Behind > 0 {
        parts = append(parts, gitBehindStyle.Background(bg).Render(fmt.Sprintf("↓%d", info.Behind)))
    }
    if info.Dirty {
        parts = append(parts, gitDirtyStyle.Background(bg).Render("*"))
    }
    sep := lipgloss.NewStyle().Background(bg).Render(" ")
    return strings.Join(parts, sep)
}

func alertIcon(level string) string {
    switch level {
    case "info":
        return lipgloss.NewStyle().Foreground(activeTheme.ColorAlertInfo).Render("●")
    case "warn":
        return lipgloss.NewStyle().Foreground(activeTheme.ColorAlertWarn).Render("")
    case "error":
        return lipgloss.NewStyle().Foreground(activeTheme.ColorAlertError).Bold(true).Render("⚑")
    }
    return ""
}

func alertIconOnBG(level string, bg lipgloss.Color) string {
    switch level {
    case "info":
        return lipgloss.NewStyle().Foreground(activeTheme.ColorAlertInfo).Background(bg).Render("●")
    case "warn":
        return lipgloss.NewStyle().Foreground(activeTheme.ColorAlertWarn).Background(bg).Render("")
    case "error":
        return lipgloss.NewStyle().Foreground(activeTheme.ColorAlertError).Bold(true).Background(bg).Render("⚑")
    }
    return ""
}

func windowCWDFromPanes(panes []tmux.Pane) string {
    for _, p := range panes {
        if p.PaneIndex == 0 {
            return p.CWD
        }
    }
    if len(panes) > 0 {
        return panes[0].CWD
    }
    return ""
}

func (s *SidebarModel) clampViewport(visibleRows int) {
    // Reserve 2 rows for the ▲/▼ hint lines so the cursor is always
    // within the rendered content area regardless of which hints appear.
    effective := visibleRows - 2
    if effective < 1 {
        effective = 1
    }
    if s.cursor < s.offset {
        s.offset = s.cursor
    }
    if s.cursor >= s.offset+effective {
        s.offset = s.cursor - effective + 1
    }
    // At the bottom of the list ▼ is absent; reclaim the freed row so the
    // viewport fills without a trailing blank line.
    if s.offset > 0 {
        contentRows := visibleRows - 1 // ▲ is present whenever offset > 0
        if s.offset+contentRows >= len(s.nodes) {
            newOffset := len(s.nodes) - contentRows
            if newOffset < 0 {
                newOffset = 0
            }
            if s.cursor >= newOffset {
                s.offset = newOffset
            }
        }
    }
    if s.offset < 0 {
        s.offset = 0
    }
}

func (s *SidebarModel) MoveUp(visibleRows int) {
    if s.cursor > 0 {
        s.cursor--
    }
    s.clampViewport(visibleRows)
}

func (s *SidebarModel) MoveDown(visibleRows int) {
    if s.cursor < len(s.nodes)-1 {
        s.cursor++
    }
    s.clampViewport(visibleRows)
}

func (s SidebarModel) WindowsForSession(session string) map[int][]tmux.Pane {
    return s.sessions[session]
}

func (s *SidebarModel) ToggleExpand() {
    if s.cursor < len(s.nodes) && s.nodes[s.cursor].IsSession {
        s.nodes[s.cursor].Expanded = !s.nodes[s.cursor].Expanded
        s.rebuildNodes()
    }
}

func (s *SidebarModel) GotoTop(visibleRows int) {
    s.cursor = 0
    s.clampViewport(visibleRows)
}

func (s *SidebarModel) GotoBottom(visibleRows int) {
    s.cursor = max(0, len(s.nodes)-1)
    s.clampViewport(visibleRows)
}

func (s *SidebarModel) MoveToSessionLevel() {
    for s.cursor > 0 && !s.nodes[s.cursor].IsSession {
        s.cursor--
    }
}

// TabPrevSession moves the cursor to the previous session node, wrapping around.
func (s *SidebarModel) TabPrevSession(visibleRows int) {
    if len(s.nodes) == 0 {
        return
    }
    var sessions []int
    for i, n := range s.nodes {
        if n.IsSession {
            sessions = append(sessions, i)
        }
    }
    if len(sessions) == 0 {
        return
    }
    cur := s.cursor
    for i := len(sessions) - 1; i >= 0; i-- {
        if sessions[i] < cur {
            s.cursor = sessions[i]
            s.clampViewport(visibleRows)
            return
        }
    }
    // wrap: go to the last session
    s.cursor = sessions[len(sessions)-1]
    s.clampViewport(visibleRows)
}

// TabNextSession advances the cursor to the next session node, wrapping around.
func (s *SidebarModel) TabNextSession(visibleRows int) {
    if len(s.nodes) == 0 {
        return
    }
    var sessions []int
    for i, n := range s.nodes {
        if n.IsSession {
            sessions = append(sessions, i)
        }
    }
    if len(sessions) == 0 {
        return
    }
    cur := s.cursor
    for _, idx := range sessions {
        if idx > cur {
            s.cursor = idx
            s.clampViewport(visibleRows)
            return
        }
    }
    // wrap: go back to the first session
    s.cursor = sessions[0]
    s.clampViewport(visibleRows)
}

// SessionCount returns the number of visible (non-ignored) sessions.
func (s SidebarModel) SessionCount() int {
    count := 0
    for _, n := range s.nodes {
        if n.IsSession {
            count++
        }
    }
    return count
}

// AlertFilterActive reports whether the alert filter is currently on.
func (s SidebarModel) AlertFilterActive() bool {
    return s.filterAlerts
}

// ToggleAlertFilter flips the alert filter flag, rebuilds nodes, and returns the new state.
func (s *SidebarModel) ToggleAlertFilter(visibleRows int) bool {
    s.filterAlerts = !s.filterAlerts
    s.rebuildNodes()
    if s.filterAlerts {
        // Move cursor to first window node with an alert.
        for i, n := range s.nodes {
            if n.IsSession {
                continue
            }
            if s.windowAlert(n.Session, n.WindowIndex) != nil {
                s.cursor = i
                break
            }
        }
    }
    // Clamp cursor to valid range before calling clampViewport.
    if s.cursor >= len(s.nodes) {
        s.cursor = max(0, len(s.nodes)-1)
    }
    s.clampViewport(visibleRows)
    return s.filterAlerts
}

func (s SidebarModel) Selected() *SidebarNode {
    if s.cursor < 0 || s.cursor >= len(s.nodes) {
        return nil
    }
    n := s.nodes[s.cursor]
    return &n
}
