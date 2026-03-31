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
    "github.com/rtalex/demux/internal/query"
    "github.com/rtalex/demux/internal/tmux"
)


type SidebarNode struct {
    Session string
}

type SidebarModel struct {
    nodes        []SidebarNode
    cursor       int
    offset       int // viewport scroll offset
    visibleRows  int // last known visible row count; used by CursorDown/CursorUp
    sessions     map[string]map[int][]tmux.Pane
    alerts       map[string]db.Alert
    gitInfo         map[string]git.Info
    sessionActivity map[string]time.Time
    cfg             config.Config
    filterAlerts    bool
    queryResult     query.Result
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
    s.rebuildNodes()
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

// BestAlertTargetInSession returns the tmux target string of the best alert
// in the given session according to the session_switch_focus setting.
// Returns "" for "default" priority or when the session has no alerts.
// Unknown values fall back to "severity".
func (s *SidebarModel) BestAlertTargetInSession(session, priority string) string {
    if priority == "default" {
        return ""
    }
    prefix := session + ":"
    var best *db.Alert
    for target, a := range s.alerts {
        if target != session && !strings.HasPrefix(target, prefix) {
            continue
        }
        a := a
        if best == nil {
            best = &a
            continue
        }
        switch priority {
        case "newest":
            if a.CreatedAt.After(best.CreatedAt) {
                best = &a
            }
        case "oldest":
            if a.CreatedAt.Before(best.CreatedAt) {
                best = &a
            }
        default: // "severity" and unknown values
            if alertSeverity(a.Level) > alertSeverity(best.Level) ||
                (alertSeverity(a.Level) == alertSeverity(best.Level) && a.CreatedAt.After(best.CreatedAt)) {
                best = &a
            }
        }
    }
    if best == nil {
        return ""
    }
    return best.Target
}

func (s *SidebarModel) rebuildNodes() {
    var curSession string
    if s.cursor >= 0 && s.cursor < len(s.nodes) {
        curSession = s.nodes[s.cursor].Session
    }

    s.nodes = nil

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

    sortKeys := s.cfg.Sidebar.Sort
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
        s.nodes = append(s.nodes, SidebarNode{Session: name})
    }

    // Filter and optionally re-sort by search result.
    if len(s.queryResult.Sessions) > 0 {
        matchSet := make(map[string]query.SessionMatch, len(s.queryResult.Sessions))
        for _, sm := range s.queryResult.Sessions {
            matchSet[sm.Name] = sm
        }

        filtered := s.nodes[:0:0]
        for _, node := range s.nodes {
            if _, ok := matchSet[node.Session]; ok {
                filtered = append(filtered, node)
            }
        }
        s.nodes = filtered

        if s.cfg.Sidebar.SearchSort == "score" {
            sort.SliceStable(s.nodes, func(i, j int) bool {
                return matchSet[s.nodes[i].Session].Score > matchSet[s.nodes[j].Session].Score
            })
        }
    }

    found := false
    for i, n := range s.nodes {
        if n.Session == curSession {
            s.cursor = i
            found = true
            break
        }
    }
    if !found {
        s.cursor = 0
    }
    if s.cursor >= len(s.nodes) {
        s.cursor = max(0, len(s.nodes)-1)
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
    return s.renderSession(node, selected, focused, width)
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
    var bestSessionAlert *db.Alert
    for target, a := range s.alerts {
        if !strings.HasPrefix(target, node.Session+":") && target != node.Session {
            continue
        }
        a := a
        if bestSessionAlert == nil || alertSeverity(a.Level) > alertSeverity(bestSessionAlert.Level) ||
            (alertSeverity(a.Level) == alertSeverity(bestSessionAlert.Level) && a.CreatedAt.After(bestSessionAlert.CreatedAt)) {
            bestSessionAlert = &a
        }
    }
    if bestSessionAlert != nil {
        if selected && focused {
            indParts = append(indParts, alertIconOnBG(bestSessionAlert.Level, activeTheme.ColorSelected))
        } else {
            indParts = append(indParts, alertIcon(bestSessionAlert.Level))
        }
    }
    var indicators string
    if selected && focused {
        sep := lipgloss.NewStyle().Background(activeTheme.ColorSelected).Render(" ")
        indicators = strings.Join(indParts, sep)
    } else {
        indicators = strings.Join(indParts, " ")
    }

    availW := width - 4
    indW := len([]rune(stripANSI(indicators)))
    maxName := availW - indW - 1
    if maxName < 4 {
        maxName = 4
    }
    nameRunes := []rune(" " + node.Session)
    if len(nameRunes) > maxName {
        nameRunes = append(nameRunes[:maxName-1], '…')
    }
    nameStr := string(nameRunes)

    if selected {
        bodyStr := string([]rune(nameStr)[1:])
        pad := availW - 1 - len([]rune(bodyStr)) - indW
        if pad < 0 {
            pad = 0
        }
        if focused {
            accent := lipgloss.NewStyle().Foreground(activeTheme.ColorSession).Background(activeTheme.ColorSelected).Render("▌")
            trail := lipgloss.NewStyle().Background(activeTheme.ColorSelected).Render("  ")
            return accent + selectedBG.Bold(true).Render(bodyStr+strings.Repeat(" ", pad)) + indicators + trail
        }
        accent := lipgloss.NewStyle().Foreground(activeTheme.ColorSession).Render("▌")
        return accent + selectedInactive.Bold(true).Render(bodyStr+strings.Repeat(" ", pad)) + indicators
    }
    text := alignedRow(nameStr, indicators, availW)
    return sessionStyle.Render(text)
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

func (s *SidebarModel) GotoTop(visibleRows int) {
    s.cursor = 0
    s.clampViewport(visibleRows)
}

func (s *SidebarModel) GotoBottom(visibleRows int) {
    s.cursor = max(0, len(s.nodes)-1)
    s.clampViewport(visibleRows)
}

func (s *SidebarModel) MoveToSessionLevel() {}

// TabPrevSession moves the cursor to the previous session node, wrapping around.
func (s *SidebarModel) TabPrevSession(visibleRows int) {
    if len(s.nodes) == 0 {
        return
    }
    if s.cursor > 0 {
        s.cursor--
    } else {
        s.cursor = len(s.nodes) - 1
    }
    s.clampViewport(visibleRows)
}

// TabNextSession advances the cursor to the next session node, wrapping around.
func (s *SidebarModel) TabNextSession(visibleRows int) {
    if len(s.nodes) == 0 {
        return
    }
    if s.cursor < len(s.nodes)-1 {
        s.cursor++
    } else {
        s.cursor = 0
    }
    s.clampViewport(visibleRows)
}

// SessionCount returns the number of visible (non-ignored) sessions.
func (s SidebarModel) SessionCount() int {
    return len(s.nodes)
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
        s.FocusFirstAlertSession(visibleRows)
    }
    if s.cursor >= len(s.nodes) {
        s.cursor = max(0, len(s.nodes)-1)
    }
    s.clampViewport(visibleRows)
    return s.filterAlerts
}

// FocusNode positions the cursor on the session node matching session.
// Returns true if found, false otherwise.
func (s *SidebarModel) FocusNode(session string, visibleRows int) bool {
    for i, n := range s.nodes {
        if n.Session == session {
            s.cursor = i
            s.clampViewport(visibleRows)
            return true
        }
    }
    return false
}

// FocusFirstAlertSession positions the cursor on the first session node that has any alert.
// Returns true if a matching node was found, false otherwise.
func (s *SidebarModel) FocusFirstAlertSession(visibleRows int) bool {
    for i, n := range s.nodes {
        if !s.newestSessionAlert(n.Session).IsZero() {
            s.cursor = i
            s.clampViewport(visibleRows)
            return true
        }
    }
    return false
}

// SetSearchResult filters and optionally re-sorts the sidebar nodes by the
// given query result. Passing an empty Result clears any active filter.
func (s *SidebarModel) SetSearchResult(r query.Result) {
    // When clearing the search (empty result), keep the cursor on the same
    // session so the proclist doesn't change and lose its scroll position.
    var prevSession string
    if len(r.Sessions) == 0 {
        if node := s.Selected(); node != nil {
            prevSession = node.Session
        }
    }
    s.queryResult = r
    s.rebuildNodes()
    if prevSession != "" {
        for i, node := range s.nodes {
            if node.Session == prevSession {
                s.cursor = i
                s.clampViewport(s.visibleRows)
                return
            }
        }
    }
    s.cursor = 0
    s.offset = 0
}

// CursorDown moves the cursor down by one row (used during search insert mode).
func (s *SidebarModel) CursorDown() {
    if s.cursor < len(s.nodes)-1 {
        s.cursor++
        vr := s.visibleRows
        if vr < 1 {
            vr = 1
        }
        s.clampViewport(vr)
    }
}

// CursorUp moves the cursor up by one row (used during search insert mode).
func (s *SidebarModel) CursorUp() {
    if s.cursor > 0 {
        s.cursor--
        vr := s.visibleRows
        if vr < 1 {
            vr = 1
        }
        s.clampViewport(vr)
    }
}

func (s SidebarModel) Selected() *SidebarNode {
    if s.cursor < 0 || s.cursor >= len(s.nodes) {
        return nil
    }
    n := s.nodes[s.cursor]
    return &n
}
