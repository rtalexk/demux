package tui

import (
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "strings"
    "time"

    "github.com/charmbracelet/lipgloss"
    runewidth "github.com/mattn/go-runewidth"
    "github.com/rtalexk/demux/internal/config"
    "github.com/rtalexk/demux/internal/db"
    "github.com/rtalexk/demux/internal/git"
    "github.com/rtalexk/demux/internal/query"
    "github.com/rtalexk/demux/internal/session"
)

// SidebarFilter determines which sessions are visible in the sidebar.
type SidebarFilter string

const (
    FilterTmux     SidebarFilter = "t"
    FilterAll      SidebarFilter = "a"
    FilterConfig   SidebarFilter = "c"
    FilterWorktree SidebarFilter = "w"
    FilterPriority SidebarFilter = "!"
)

type SidebarNode struct {
    Session string
}

type SidebarModel struct {
    nodes       []SidebarNode
    cursor      int
    offset      int // viewport scroll offset
    visibleRows int // last known visible row count; used by CursorDown/CursorUp
    sessions    []session.Session
    alerts      map[string]db.Alert
    gitInfo     map[string]git.Info
    cfg         config.Config
    filter      SidebarFilter
    prevSession string // selected session before last filter switch; restored on toggle-off
    filterHint  string
    queryResult query.Result
    launchErr   string // shown inline when last launch attempt failed
}

func (s *SidebarModel) SetData(sessions []session.Session, alerts []db.Alert, gitInfo map[string]git.Info, cfg config.Config) {
    s.sessions = sessions
    s.alerts = make(map[string]db.Alert, len(alerts))
    for _, a := range alerts {
        s.alerts[a.Target] = a
    }
    s.gitInfo = gitInfo
    s.cfg = cfg
    s.rebuildNodes()
}

// SetFilter changes the active sidebar filter. Pressing the current filter's
// key again toggles back to FilterTmux (the default) and restores the cursor
// to the session that was selected before the filter was applied.
func (s *SidebarModel) SetFilter(f SidebarFilter, visibleRows int) {
    curSession := ""
    if node := s.Selected(); node != nil {
        curSession = node.Session
    }

    var restoreSession string
    if f == s.filter {
        restoreSession = s.prevSession
        s.prevSession = curSession
        f = FilterTmux
    } else {
        s.prevSession = curSession
    }

    s.filter = f
    s.rebuildNodes()
    s.clampViewport(visibleRows)

    if restoreSession != "" {
        for i, n := range s.nodes {
            if n.Session == restoreSession {
                s.cursor = i
                s.clampViewport(visibleRows)
                break
            }
        }
    }
}

// ActiveFilter returns the current active filter.
func (s SidebarModel) ActiveFilter() SidebarFilter {
    return s.filter
}

// alertSeverity maps alert level to a numeric priority (higher = more severe).
func alertSeverity(level string) int {
    switch level {
    case "error":
        return 3
    case "warn":
        return 2
    case "info":
        return 1
    default: // "defer" and unknown values
        return 0
    }
}

// newestSessionAlert returns the most recent alert CreatedAt for a session
// (checking pane-level targets "session:window.pane"), or zero time if none.
func (s *SidebarModel) newestSessionAlert(sess string) time.Time {
    var newest time.Time
    for target, a := range s.alerts {
        if strings.HasPrefix(target, sess+":") || target == sess {
            if a.CreatedAt.After(newest) {
                newest = a.CreatedAt
            }
        }
    }
    return newest
}

// newestAlertAtSeverity returns the most recent CreatedAt among alerts for a
// session whose alertSeverity equals sv, or zero time if none match.
func (s *SidebarModel) newestAlertAtSeverity(sess string, sv int) time.Time {
    var newest time.Time
    for target, a := range s.alerts {
        if strings.HasPrefix(target, sess+":") || target == sess {
            if alertSeverity(a.Level) == sv && a.CreatedAt.After(newest) {
                newest = a.CreatedAt
            }
        }
    }
    return newest
}

// highestSessionAlertSeverity returns the maximum alertSeverity value across
// all alerts belonging to the session, or -1 if the session has no alerts.
func (s *SidebarModel) highestSessionAlertSeverity(sess string) int {
    best := -1
    for target, a := range s.alerts {
        if strings.HasPrefix(target, sess+":") || target == sess {
            if sv := alertSeverity(a.Level); sv > best {
                best = sv
            }
        }
    }
    return best
}

// isBetterAlert reports whether candidate should replace current as the "best"
// alert for a session under the given priority strategy.
func isBetterAlert(candidate, current db.Alert, priority string) bool {
    switch priority {
    case "newest":
        return candidate.CreatedAt.After(current.CreatedAt)
    case "oldest":
        return candidate.CreatedAt.Before(current.CreatedAt)
    default: // "severity" and unknown values
        return alertSeverity(candidate.Level) > alertSeverity(current.Level) ||
            (alertSeverity(candidate.Level) == alertSeverity(current.Level) && candidate.CreatedAt.After(current.CreatedAt))
    }
}

// BestAlertTargetInSession returns the tmux target string of the best alert
// in the given session according to the session_switch_focus setting.
// Returns "" for "default" priority or when the session has no alerts.
// Unknown values fall back to "severity".
func (s *SidebarModel) BestAlertTargetInSession(sess, priority string) string {
    if priority == "default" {
        return ""
    }
    prefix := sess + ":"
    var best *db.Alert
    for target, a := range s.alerts {
        if target != sess && !strings.HasPrefix(target, prefix) {
            continue
        }
        a := a
        if best == nil {
            best = &a
            continue
        }
        if isBetterAlert(a, *best, priority) {
            best = &a
        }
    }
    if best == nil {
        return ""
    }
    return best.Target
}

// visibleSessions returns sessions matching the current filter with IgnoredSessions removed.
// For FilterWorktree with no resolvable root, returns nil and sets s.filterHint.
// isIgnoredSession reports whether the named session is in the ignore list.
func (s *SidebarModel) isIgnoredSession(name string) bool {
    for _, ig := range s.cfg.IgnoredSessions {
        if ig == name {
            return true
        }
    }
    return false
}

// worktreeRootRef resolves the worktree root for the session currently under
// the cursor, returning "" when no root can be determined.
func (s *SidebarModel) worktreeRootRef() string {
    var curDisplayName string
    if s.cursor >= 0 && s.cursor < len(s.nodes) {
        curDisplayName = s.nodes[s.cursor].Session
    }
    for _, sess := range s.sessions {
        if sess.DisplayName == curDisplayName {
            return s.sessionWorktreeRoot(sess)
        }
    }
    return ""
}

// matchesWorktreeFilter reports whether sess belongs to the same worktree root.
func (s *SidebarModel) matchesWorktreeFilter(sess session.Session, rootRef string) bool {
    r := s.sessionWorktreeRoot(sess)
    return r != "" && r == rootRef
}

// filterAll returns all non-ignored sessions.
func (s *SidebarModel) filterAll() []session.Session {
    var out []session.Session
    for _, sess := range s.sessions {
        if !s.isIgnoredSession(sess.DisplayName) {
            out = append(out, sess)
        }
    }
    return out
}

// filterConfig returns non-ignored config sessions.
func (s *SidebarModel) filterConfig() []session.Session {
    var out []session.Session
    for _, sess := range s.sessions {
        if sess.IsConfig && !s.isIgnoredSession(sess.DisplayName) {
            out = append(out, sess)
        }
    }
    return out
}

// filterPriority returns non-ignored live sessions that have at least one alert.
func (s *SidebarModel) filterPriority() []session.Session {
    var out []session.Session
    for _, sess := range s.sessions {
        if sess.IsLive && !s.newestSessionAlert(sess.DisplayName).IsZero() && !s.isIgnoredSession(sess.DisplayName) {
            out = append(out, sess)
        }
    }
    return out
}

// filterTmux returns non-ignored live sessions.
func (s *SidebarModel) filterTmux() []session.Session {
    var out []session.Session
    for _, sess := range s.sessions {
        if sess.IsLive && !s.isIgnoredSession(sess.DisplayName) {
            out = append(out, sess)
        }
    }
    return out
}

func (s *SidebarModel) visibleSessions() []session.Session {
    switch s.filter {
    case FilterAll:
        return s.filterAll()
    case FilterConfig:
        return s.filterConfig()
    case FilterPriority:
        return s.filterPriority()
    case FilterWorktree:
        rootRef := s.worktreeRootRef()
        if rootRef == "" {
            s.filterHint = "no sessions in this worktree"
            return nil
        }
        var out []session.Session
        for _, sess := range s.sessions {
            if !s.isIgnoredSession(sess.DisplayName) && s.matchesWorktreeFilter(sess, rootRef) {
                out = append(out, sess)
            }
        }
        return out
    default: // FilterTmux and unknown values
        return s.filterTmux()
    }
}

// sessionWorktreeRoot returns the worktree root path for a session, or "".
// For live sessions: filepath.Dir(gitInfo.RepoRoot).
// For config sessions with Worktree=true: filepath.Dir(Config.Path).
func (s *SidebarModel) sessionWorktreeRoot(sess session.Session) string {
    if sess.IsLive {
        if info, ok := s.gitInfo[sess.DisplayName]; ok {
            if info.RepoRoot != "" {
                return filepath.Dir(info.RepoRoot)
            }
            // Session CWD is the worktree root (contains .bare/, git unavailable there).
            if info.IsWorktreeRoot {
                return info.Dir
            }
        }
    }
    if sess.IsConfig && sess.Config != nil && sess.Config.Worktree {
        p := sess.Config.Path
        // If p itself is the worktree root container (.bare/ lives here), return p.
        // Otherwise p is a specific worktree inside a container, so its parent is the root.
        if fi, err := os.Stat(filepath.Join(p, ".bare")); err == nil && fi.IsDir() {
            return p
        }
        return filepath.Dir(p)
    }
    return ""
}

func (s *SidebarModel) sessionSortLess(si, sj session.Session, sortKeys []string) bool {
    for _, k := range sortKeys {
        switch k {
        case "priority":
            svi := s.highestSessionAlertSeverity(si.DisplayName)
            svj := s.highestSessionAlertSeverity(sj.DisplayName)
            hasI := svi >= 0
            hasJ := svj >= 0
            if hasI != hasJ {
                return hasI
            }
            if hasI && hasJ {
                if svi != svj {
                    return svi > svj
                }
                ti := s.newestAlertAtSeverity(si.DisplayName, svi)
                tj := s.newestAlertAtSeverity(sj.DisplayName, svj)
                if !ti.Equal(tj) {
                    return ti.After(tj)
                }
            }
        case "last_seen":
            if !si.Activity.Equal(sj.Activity) {
                return si.Activity.After(sj.Activity)
            }
        case "alphabetical":
            return si.DisplayName < sj.DisplayName
        }
    }
    return false
}

// applySearchFilter narrows s.nodes to the query result set and optionally
// re-sorts by score, mutating s.nodes in place.
func (s *SidebarModel) applySearchFilter() {
    if s.queryResult.Sessions == nil {
        return
    }
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

// restoreCursor repositions s.cursor to curSession, defaulting to 0.
func (s *SidebarModel) restoreCursor(curSession string) {
    for i, n := range s.nodes {
        if n.Session == curSession {
            s.cursor = i
            return
        }
    }
    s.cursor = 0
    if s.cursor >= len(s.nodes) {
        s.cursor = max(0, len(s.nodes)-1)
    }
}

func (s *SidebarModel) rebuildNodes() {
    var curSession string
    if s.cursor >= 0 && s.cursor < len(s.nodes) {
        curSession = s.nodes[s.cursor].Session
    }

    // Call visibleSessions before clearing s.nodes so FilterWorktree can
    // read the current cursor session from s.nodes[s.cursor].
    s.filterHint = ""
    visible := s.visibleSessions()

    s.nodes = nil

    sortKeys := s.cfg.Sidebar.Sort
    if len(sortKeys) == 0 {
        sortKeys = []string{"priority", "last_seen", "alphabetical"}
    }
    sort.Slice(visible, func(i, j int) bool {
        return s.sessionSortLess(visible[i], visible[j], sortKeys)
    })

    for _, sess := range visible {
        s.nodes = append(s.nodes, SidebarNode{Session: sess.DisplayName})
    }

    s.applySearchFilter()
    s.restoreCursor(curSession)
    if s.cursor >= len(s.nodes) {
        s.cursor = max(0, len(s.nodes)-1)
    }
}

// sidebarViewport computes adjusted offset, content row budget, and scroll hints.
// Pure function — safe to call from the read-only View/Render method.
func sidebarViewport(cursor, offset, visibleRows, nodeCount int) (adjOffset, contentRows int, hasAbove, hasBelow bool) {
    if cursor < offset {
        offset = cursor
    }
    if cursor >= offset+visibleRows {
        offset = cursor - visibleRows + 1
    }
    if offset < 0 {
        offset = 0
    }
    hasAbove = offset > 0
    contentRows = visibleRows
    if hasAbove {
        contentRows--
    }
    hasBelow = offset+contentRows < nodeCount
    if hasBelow {
        contentRows--
    }
    if contentRows < 1 {
        contentRows = 1
    }
    return offset, contentRows, hasAbove, hasBelow
}

func (s SidebarModel) Render(width, height int, focused bool, title, rightTitle string) string {
    visibleRows := height - 2
    if visibleRows < 1 {
        visibleRows = 1
    }

    offset, contentRows, hasAbove, hasBelow := sidebarViewport(s.cursor, s.offset, visibleRows, len(s.nodes))

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
    if len(s.nodes) == 0 {
        var hintText string
        switch {
        case s.filterHint != "":
            hintText = s.filterHint
        case s.filter == FilterPriority:
            hintText = "no alerts"
        case s.queryResult.Sessions != nil:
            hintText = "no results"
        default:
            hintText = "no sessions"
        }
        lines = append(lines, centeredHint(hintText))
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
    if s.launchErr != "" {
        errLine := lipgloss.NewStyle().
            Foreground(activeTheme.ColorFgMuted).
            Italic(true).
            Width(width - 2).
            Align(lipgloss.Center).
            Render("⚠ " + s.launchErr)
        inner += "\n" + errLine
    }
    style := borderInactive
    if focused {
        style = borderActive
    }
    rendered := injectBorderTitles(style.Width(width-2).Height(height-2).Render(inner), title, rightTitle)
    shortcutBar := filterShortcutBar(s.filter, width-2)
    return injectBottomBorderLabel(rendered, shortcutBar)
}

func (s SidebarModel) renderNode(node SidebarNode, selected, focused bool, width int) string {
    return s.renderSession(node, selected, focused, width)
}

// alignedRow builds a single sidebar line with the name on the left and
// indicators right-aligned to availWidth. Both name and indicators are
// measured by display-column width (runewidth) after stripping ANSI codes,
// so multi-column glyphs (e.g. emoji) are accounted for correctly.
func alignedRow(name, indicators string, availWidth int) string {
    nameW := runewidth.StringWidth(stripANSI(name))
    indW := runewidth.StringWidth(stripANSI(indicators))
    pad := availWidth - nameW - indW
    if pad < 1 {
        pad = 1
    }
    return name + strings.Repeat(" ", pad) + indicators
}

// FindSession returns a pointer to the Session with the given display name, or nil.
func (s SidebarModel) FindSession(displayName string) *session.Session {
    for i := range s.sessions {
        if s.sessions[i].DisplayName == displayName {
            return &s.sessions[i]
        }
    }
    return nil
}

// SetLaunchErr stores a launch error message for inline sidebar display.
func (s *SidebarModel) SetLaunchErr(msg string) { s.launchErr = msg }

// ClearLaunchErr clears any stored launch error.
func (s *SidebarModel) ClearLaunchErr() { s.launchErr = "" }

// sessionIcon returns the rendered icon for a sidebar session row.
func sessionIcon(sess session.Session) string {
    var icon string
    if sess.IsConfig && sess.Config != nil && sess.Config.Icon != "" {
        icon = sess.Config.Icon
    } else if sess.IsLive {
        icon = activeTheme.IconTmuxSession
    } else {
        icon = activeTheme.IconCfgSession
    }
    return sessionIconStyle.Render(icon)
}

// sessionIndicators assembles the right-side indicator string for a sidebar row.
func (s SidebarModel) sessionIndicators(node SidebarNode, selected, focused bool) string {
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
    if s.cfg.Sidebar.ShowLastSeen {
        if sess := s.FindSession(node.Session); sess != nil && !sess.Activity.IsZero() {
            age := formatAge(sess.Activity, time.Now())
            if selected && focused {
                indParts = append(indParts, hintStyle.Background(activeTheme.ColorSelected).Render(age))
            } else {
                indParts = append(indParts, hintStyle.Render(age))
            }
        }
    }
    if selected && focused {
        sep := lipgloss.NewStyle().Background(activeTheme.ColorSelected).Render(" ")
        return strings.Join(indParts, sep)
    }
    return strings.Join(indParts, " ")
}

// truncateSessionName truncates name to fit within maxName display columns,
// appending "…" when truncation is needed.
func truncateSessionName(name string, maxName int) string {
    if runewidth.StringWidth(name) <= maxName {
        return name
    }
    runes := []rune(name)
    for runewidth.StringWidth(string(runes)) > maxName-1 {
        runes = runes[:len(runes)-1]
    }
    return string(runes) + "…"
}

// renderSelectedRow renders a sidebar row that is currently selected.
// When focused is true the row is highlighted with background colour and a trail.
func renderSelectedRow(iconPrefix, nameStr, indicators string, availW, indW int, focused bool) string {
    const gap = " "
    pad := availW - runewidth.StringWidth(nameStr) - indW
    if pad < 0 {
        pad = 0
    }
    if focused {
        indicatorGlyph := lipgloss.NewStyle().Foreground(activeTheme.ColorSession).Background(activeTheme.ColorSelected).Render("▌")
        trail := lipgloss.NewStyle().Background(activeTheme.ColorSelected).Render("  ")
        return indicatorGlyph + gap + iconPrefix + selectedBG.Bold(true).Render(nameStr+strings.Repeat(" ", pad)) + indicators + trail
    }
    indicatorGlyph := lipgloss.NewStyle().Foreground(activeTheme.ColorSession).Render("▌")
    return indicatorGlyph + gap + iconPrefix + selectedInactive.Bold(true).Render(nameStr+strings.Repeat(" ", pad)) + indicators
}

func (s SidebarModel) renderSession(node SidebarNode, selected, focused bool, width int) string {
    indicators := s.sessionIndicators(node, selected, focused)

    // Icon prefix: look up the session and render its icon.
    iconPrefix := ""
    if sess := s.FindSession(node.Session); sess != nil {
        iconPrefix = sessionIcon(*sess) + " "
    }
    iconW := runewidth.StringWidth(stripANSI(iconPrefix))

    // Row format: [focus(1)] [gap(2)] [icon(iconW)] [name+indicators(availW)]
    // Box content = width-2. Selected rows append trail(2), so body must fill
    // width-2 - 1 - 2 - iconW - 2 = width-7-iconW chars.
    availW := width - 6 - iconW
    if availW < 4 {
        availW = 4
    }
    indW := runewidth.StringWidth(stripANSI(indicators))
    maxName := availW - indW
    if indW > 0 {
        maxName-- // alignedRow enforces pad>=1 separator; reserve it so truncation doesn't overflow
    }
    if maxName < 4 {
        maxName = 4
    }
    nameStr := truncateSessionName(node.Session, maxName)

    const gap = " " // 1-space gap between focus indicator and icon
    if selected {
        return renderSelectedRow(iconPrefix, nameStr, indicators, availW, indW, focused)
    }
    text := alignedRow(nameStr, indicators, availW)
    return " " + gap + iconPrefix + sessionStyle.Render(text)
}

// formatAge returns a fixed-width 3-char age string for a session's last-seen
// timestamp. Special cases: <15s → "now", <1m → "<1m". For longer durations:
// ' Xm' / 'XXm' for minutes, ' Xh' / 'XXh' for hours, ' Xd' / 'XXd' for days.
// Single-digit values are space-padded on the left.
func formatAge(t, now time.Time) string {
    d := now.Sub(t)
    if d < 0 {
        d = 0
    }
    switch {
    case d < 15*time.Second:
        return "now"
    case d < time.Minute:
        return "<1m"
    case d < time.Hour:
        n := int(d.Minutes())
        if n < 10 {
            return fmt.Sprintf(" %dm", n)
        }
        return fmt.Sprintf("%dm", n)
    case d < 24*time.Hour:
        n := int(d.Hours())
        if n < 10 {
            return fmt.Sprintf(" %dh", n)
        }
        return fmt.Sprintf("%dh", n)
    default:
        n := int(d.Hours() / 24)
        if n < 10 {
            return fmt.Sprintf(" %dd", n)
        }
        return fmt.Sprintf("%dd", n)
    }
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

// FocusNode positions the cursor on the session node matching sess.
// Returns true if found, false otherwise.
func (s *SidebarModel) FocusNode(sess string, visibleRows int) bool {
    for i, n := range s.nodes {
        if n.Session == sess {
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

// filterShortcuts lists all filter shortcuts in trim-priority order (right-to-left trimming).
var filterShortcuts = []struct {
    filter SidebarFilter
    label  string
}{
    {FilterAll, "[a] All"},
    {FilterTmux, "[t] Tmux"},
    {FilterConfig, "[c] Cfg"},
    {FilterWorktree, "[w] Workt"},
    {FilterPriority, "[!] Prior"},
}

// filterShortcutBar builds the centered shortcut string for the sidebar bottom border.
// The active filter's label is highlighted with ColorSession + bold.
// Shortcuts are trimmed right-to-left when they don't fit in innerWidth runes.
// At minimum, "[t] Tmux" (index 1) is always kept.
func filterShortcutBar(active SidebarFilter, innerWidth int) string {
    parts := make([]string, len(filterShortcuts))
    for i, sc := range filterShortcuts {
        if sc.filter == active {
            parts[i] = lipgloss.NewStyle().Foreground(activeTheme.ColorSession).Bold(true).Render(sc.label)
        } else {
            parts[i] = hintStyle.Render(sc.label)
        }
    }
    // Trim right-to-left until the string fits, keeping at least [t] Tmux (index 1).
    for end := len(parts); end > 1; end-- {
        candidate := strings.Join(parts[:end], " ")
        if len([]rune(stripANSI(candidate))) <= innerWidth {
            return candidate
        }
    }
    return parts[1] // always show [t] Tmux as fallback
}

func (s SidebarModel) Selected() *SidebarNode {
    if s.cursor < 0 || s.cursor >= len(s.nodes) {
        return nil
    }
    n := s.nodes[s.cursor]
    return &n
}
