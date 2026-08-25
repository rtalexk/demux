package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	runewidth "github.com/mattn/go-runewidth"
	"github.com/rtalexk/demux/internal/config"
	"github.com/rtalexk/demux/internal/db"
	"github.com/rtalexk/demux/internal/git"
	"github.com/rtalexk/demux/internal/procmatch"
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
	FilterWatch    SidebarFilter = "@"
)

// Session row layout constants.
const (
	// focusGlyph is the left-edge bar rendered on selected/focused rows.
	focusGlyph = "▌"

	// selectedTrail is the trailing pad that extends the highlight to the right border.
	selectedTrail = " "

	// sidebarRowOverhead is the total fixed-width cost of the non-name columns in a
	// session row: border(2) + focus/indicator(1) + gap(1) + sep(1) + trail(1).
	sidebarRowOverhead = 6

	// minRowWidth is the minimum display-column budget reserved for the session name.
	minRowWidth = 4

	// warningGlyph is the icon prepended to sidebar error messages.
	warningGlyph = "⚠"
)

type SidebarNode struct {
	Session string
}

type SidebarModel struct {
	nodes             []SidebarNode
	cursor            int
	offset            int // viewport scroll offset
	visibleRows       int // last known visible row count; used by CursorDown/CursorUp
	sessions          []session.Session
	states            []db.ToolState
	nameToID          map[string]string // session display name → session_id ($N)
	gitInfo           map[string]git.Info
	cfg               config.Config
	filter            SidebarFilter
	prevSession       string // selected session before last filter switch; restored on toggle-off
	filterHint        string
	queryResult       query.Result
	launchErr         string // shown inline when last launch attempt failed
	activeSession     string // tmux session the user is currently attached to
	watches           map[string]struct{}
	itemSessions      map[string]struct{} // sessions with open (unchecked) TODOs
	noteSessions      map[string]struct{} // sessions with at least one note
	procLabels        map[string]procmatch.Label
	marquee           Marquee
	lastMarqueeCursor int
}

func (s *SidebarModel) SetData(sessions []session.Session, states []db.ToolState, gitInfo map[string]git.Info, cfg config.Config) {
	s.sessions = sessions
	s.states = states
	s.gitInfo = gitInfo
	s.cfg = cfg
	s.rebuildNodes()
}

// SetNameToIDMap stores the mapping from session display name to stable session_id.
// Called by the Model after each panes refresh.
func (s *SidebarModel) SetNameToIDMap(m map[string]string) {
	s.nameToID = m
}

// SetActiveSession records which tmux session the user is currently attached to.
// Pass an empty string when not inside tmux.
func (s *SidebarModel) SetActiveSession(name string) {
	s.activeSession = name
}

// SetWatches updates the set of watched session names.
func (s *SidebarModel) SetWatches(sessions []string) {
	s.watches = make(map[string]struct{}, len(sessions))
	for _, name := range sessions {
		s.watches[name] = struct{}{}
	}
}

// SetItemSessions updates the set of session names that have open (unchecked) TODO items.
func (s *SidebarModel) SetItemSessions(sessions []string) {
	s.itemSessions = make(map[string]struct{}, len(sessions))
	for _, name := range sessions {
		s.itemSessions[name] = struct{}{}
	}
}

// SetNoteSessions updates the set of session names that have at least one note.
func (s *SidebarModel) SetNoteSessions(sessions []string) {
	s.noteSessions = make(map[string]struct{}, len(sessions))
	for _, name := range sessions {
		s.noteSessions[name] = struct{}{}
	}
}

// SetProcLabels updates the per-session sidebar process labels. Pass nil to clear.
func (s *SidebarModel) SetProcLabels(labels map[string]procmatch.Label) {
	s.procLabels = labels
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

// sessionTargetStateFor returns the highest-priority ToolState whose target IS the
// session itself (TargetTypeSession). Pane and window states are excluded. Use this
// for the Proclist title where only session-level annotations should be displayed.
func (s *SidebarModel) sessionTargetStateFor(sess string) *db.ToolState {
	sessID := s.nameToID[sess]
	var best *db.ToolState
	bestPri := 0
	for i := range s.states {
		st := &s.states[i]
		if st.Target.Type != db.TargetTypeSession {
			continue
		}
		var matches bool
		if sessID != "" {
			matches = st.Target.SessionID == sessID
		} else {
			matches = st.Target.ID == sess
		}
		if !matches {
			continue
		}
		pri := st.Value.Priority()
		if best == nil || pri > bestPri {
			bestPri = pri
			best = st
		}
	}
	return best
}

// stateForSession returns the highest-priority ToolState for the session, or nil if none.
func (s *SidebarModel) stateForSession(sess string) *db.ToolState {
	sessID := s.nameToID[sess]
	var best *db.ToolState
	bestPri := 0
	for i := range s.states {
		st := &s.states[i]
		// Match by session_id when available; fall back to session-level target by name.
		var matches bool
		if sessID != "" {
			matches = st.Target.SessionID == sessID
		} else {
			matches = st.Target.Type == db.TargetTypeSession && st.Target.ID == sess
		}
		if !matches {
			continue
		}
		pri := st.Value.Priority()
		if best == nil || pri > bestPri {
			bestPri = pri
			best = st
		}
	}
	return best
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

// filterWatch returns sessions that are in the watch set.
func (s *SidebarModel) filterWatch() []session.Session {
	var out []session.Session
	for _, sess := range s.sessions {
		if s.isIgnoredSession(sess.DisplayName) {
			continue
		}
		if _, ok := s.watches[sess.DisplayName]; ok {
			out = append(out, sess)
		}
	}
	return out
}

// filterPriority returns non-ignored live sessions that have an attention state
// (waiting, error, flagged, or optionally working).
func (s *SidebarModel) filterPriority() []session.Session {
	var out []session.Session
	for _, sess := range s.sessions {
		if !sess.IsLive || s.isIgnoredSession(sess.DisplayName) {
			continue
		}
		st := s.stateForSession(sess.DisplayName)
		if st == nil {
			continue
		}
		switch st.Value {
		case db.StateWaiting, db.StateError, db.StateFlagged:
			out = append(out, sess)
		case db.StateWorking:
			if s.cfg.Tui.AttentionFilterIncludeWorking {
				out = append(out, sess)
			}
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
	case FilterWatch:
		return s.filterWatch()
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

// comparePriority compares two sessions by state priority.
// Returns -1 if si sorts before sj, 1 if sj sorts before si, 0 if equal.
func (s *SidebarModel) comparePriority(si, sj session.Session) int {
	priI := -1
	if stI := s.stateForSession(si.DisplayName); stI != nil {
		priI = stI.Value.Priority()
	}
	priJ := -1
	if stJ := s.stateForSession(sj.DisplayName); stJ != nil {
		priJ = stJ.Value.Priority()
	}
	if priI != priJ {
		if priI > priJ {
			return -1
		}
		return 1
	}
	return 0
}

func (s *SidebarModel) sessionSortLess(si, sj session.Session, sortKeys []string) bool {
	for _, k := range sortKeys {
		switch k {
		case "priority":
			if cmp := s.comparePriority(si, sj); cmp != 0 {
				return cmp < 0
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

// countFit walks nodes starting at offset, summing heights, and returns the
// index just past the last fitting node plus whether the cursor index falls
// within that range. Iteration continues until budget is exhausted or the
// list ends, so consumed reflects the actual fitting range.
func countFit(offset, nodeCount, cursor, budget int, height func(i int, isLast bool) int) (consumed int, cursorFits bool) {
	rows := 0
	consumed = offset
	for i := offset; i < nodeCount; i++ {
		h := height(i, i == nodeCount-1)
		if rows+h > budget {
			return consumed, cursorFits
		}
		rows += h
		consumed = i + 1
		if i == cursor {
			cursorFits = true
		}
	}
	return consumed, cursorFits
}

// sidebarViewport computes adjusted offset, content row budget, and scroll hints.
// Pure function, safe to call from the read-only View/Render method.
//
// height(i, isLast) returns the row cost of node i. Cursor and offset are
// still node indices; the function adjusts them so the cursor's node fits
// within the row budget.
func sidebarViewport(cursor, offset, visibleRows, nodeCount int, height func(i int, isLast bool) int) (adjOffset, contentRows int, hasAbove, hasBelow bool) {
	if cursor < offset {
		offset = cursor
	}
	if offset < 0 {
		offset = 0
	}
	for {
		hasAbove = offset > 0
		contentRows = visibleRows
		if hasAbove {
			contentRows--
		}
		consumed, cursorFits := countFit(offset, nodeCount, cursor, contentRows, height)
		hasBelow = consumed < nodeCount
		if hasBelow {
			// Reserve a row for the ▼ hint and re-measure: the first pass above
			// did not know hasBelow yet, so the budget was one too generous.
			if contentRows-1 < 1 {
				contentRows = 1
			} else {
				contentRows--
			}
			consumed, cursorFits = countFit(offset, nodeCount, cursor, contentRows, height)
			hasBelow = consumed < nodeCount
		}
		if cursorFits || offset >= cursor {
			break
		}
		offset++
	}
	if contentRows < 1 {
		contentRows = 1
	}
	return offset, contentRows, hasAbove, hasBelow
}

// emptyHintText returns the hint text to display when the sidebar has no nodes.
func (s SidebarModel) emptyHintText() string {
	switch {
	case s.filterHint != "":
		return s.filterHint
	case s.filter == FilterPriority:
		return "no attention states"
	case s.queryResult.Sessions != nil:
		return "no results"
	default:
		return "no sessions"
	}
}

// buildSidebarLines builds the list of rendered node lines for the sidebar
// content area. Honors per-node height (card mode emits a blank separator
// after each session except the last visible one).
func (s SidebarModel) buildSidebarLines(offset, contentRows int, hasAbove, hasBelow, focused bool, width int, centeredHint func(string) string) []string {
	var lines []string
	if hasAbove {
		lines = append(lines, centeredHint(scrollHintAbove))
	}
	end := offset + contentRows
	if end > len(s.nodes) {
		end = len(s.nodes)
	}
	innerW := width - borderOverhead
	sep := s.cardSeparatorRow(innerW)
	rowsLeft := contentRows
	for i := offset; i < end; i++ {
		isLastInList := (i == len(s.nodes)-1)
		h := s.nodeHeight(isLastInList)
		if h > rowsLeft {
			break
		}
		rendered := s.renderNode(s.nodes[i], i == s.cursor, focused, width)
		lines = append(lines, strings.Split(rendered, "\n")...)
		rowsLeft -= h
		if s.cardView() && sep != "" && !isLastInList {
			// Only emit the separator if the next card will actually be rendered.
			// Without this guard, scrolled-mid-list views produce a spurious row
			// between the last visible card and the ▼ scroll hint. nodeHeight
			// includes the separator for non-last cards, so rowsLeft was already
			// charged for it; skipping the append here over-subtracts rowsLeft by
			// 1, but the loop is about to exit so no further damage.
			nextH := s.nodeHeight((i + 1) == len(s.nodes)-1)
			if nextH <= rowsLeft {
				lines = append(lines, sep)
			}
		}
	}
	if hasBelow {
		lines = append(lines, centeredHint(scrollHintBelow))
	}
	return lines
}

func (s SidebarModel) Render(width, height int, focused bool, title, rightTitle string) string {
	visibleRows := height - 2
	if visibleRows < 1 {
		visibleRows = 1
	}

	heightFn := func(_ int, isLast bool) int { return s.nodeHeight(isLast) }
	offset, contentRows, hasAbove, hasBelow := sidebarViewport(s.cursor, s.offset, visibleRows, len(s.nodes), heightFn)

	innerW := width - borderOverhead
	centeredHint := func(text string) string {
		pad := (innerW - len([]rune(text))) / 2
		if pad < 0 {
			pad = 0
		}
		return hintStyle.Render(strings.Repeat(" ", pad) + text)
	}

	var lines []string
	if len(s.nodes) == 0 {
		lines = append(lines, centeredHint(s.emptyHintText()))
	} else {
		lines = s.buildSidebarLines(offset, contentRows, hasAbove, hasBelow, focused, width, centeredHint)
	}

	inner := strings.Join(lines, "\n")
	if s.launchErr != "" {
		errLine := lipgloss.NewStyle().
			Foreground(activeTheme.ColorFgMuted).
			Italic(true).
			Width(width - borderOverhead).
			Align(lipgloss.Center).
			Render(warningGlyph + " " + s.launchErr)
		inner += "\n" + errLine
	}
	style := borderInactive
	if focused {
		style = borderActive
	}
	rendered := injectBorderTitles(style.Width(width-borderOverhead).Height(height-borderOverhead).Render(inner), title, rightTitle)
	shortcutBar := filterShortcutBar(s.filter, width-borderOverhead)
	return injectBottomBorderLabel(rendered, shortcutBar)
}

func (s SidebarModel) renderNode(node SidebarNode, selected, focused bool, width int) string {
	return s.renderSession(node, selected, focused, width)
}

// cardView reports whether the resolved session view (taking into account the
// per-mode overrides) is the card layout.
func (s SidebarModel) cardView() bool {
	return s.cfg.Sidebar.ResolvedSessionView(s.cfg.Mode) == config.SidebarViewCard
}

// cardSeparatorRow returns the row rendered between cards in card view. It
// returns "" when separators are disabled, a row of spaces for "blank", and a
// styled horizontal rule for "rule".
func (s SidebarModel) cardSeparatorRow(innerW int) string {
	switch s.cfg.Sidebar.CardSeparator {
	case config.CardSeparatorNone:
		return ""
	case config.CardSeparatorRule:
		return lipgloss.NewStyle().Foreground(activeTheme.ColorBorder).Render(strings.Repeat("─", innerW))
	default:
		return strings.Repeat(" ", innerW)
	}
}

// nodeHeight returns the viewport rows consumed by a session in the current
// view. Card mode reserves 2 content rows; a trailing separator row adds 1
// for every card except the last visible one when card_separator is enabled.
func (s SidebarModel) nodeHeight(isLast bool) int {
	if s.cardView() {
		if isLast || s.cfg.Sidebar.CardSeparator == config.CardSeparatorNone {
			return 2
		}
		return 3
	}
	return 1
}

// alignedRow builds a single sidebar line with the name on the left and
// indicators right-aligned to availWidth. Both name and indicators are
// measured by display-column width (runewidth) after stripping ANSI codes,
// so multi-column glyphs (e.g. emoji) are accounted for correctly.
func alignedRow(name, indicators string, availWidth int) string {
	nameW := visualWidth(name)
	indW := visualWidth(indicators)
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

// sessionIcon returns the raw (unstyled) icon glyph for a sidebar session
// row. Styling is applied by the caller via styledIconPrefix so the glyph can
// be composed into a highlighted row without an embedded SGR reset breaking
// the selection background.
func sessionIcon(sess session.Session) string {
	if sess.IsConfig && sess.Config != nil && sess.Config.Icon != "" {
		return sess.Config.Icon
	}
	if sess.IsLive {
		return activeTheme.IconTmuxSession
	}
	return activeTheme.IconCfgSession
}

// sessionIconPrefix returns the raw "<icon> " prefix for a sidebar session
// row, or "" when the session cannot be resolved. The trailing space separates
// the icon from the name; styledIconPrefix applies the colour.
func (s SidebarModel) sessionIconPrefix(node SidebarNode) string {
	sess := s.FindSession(node.Session)
	if sess == nil {
		return ""
	}
	return sessionIcon(*sess) + " "
}

// styledIconPrefix styles a raw "<icon> " prefix for a sidebar session row.
// When highlighted, the muted icon colour and the selection background are
// applied as a single lipgloss style: rendering a pre-styled string (which
// ends in an SGR reset) inside a background style would reset that background
// mid-row and leave the space after the icon un-highlighted.
func styledIconPrefix(iconPrefix string, highlighted bool) string {
	if iconPrefix == "" {
		return ""
	}
	if highlighted {
		return selectedBG.Foreground(activeTheme.ColorFgMuted).Render(iconPrefix)
	}
	return sessionIconStyle.Render(iconPrefix)
}

// isIconLabel reports whether a proc-label text is a single icon glyph
// (emoji, Nerd Font, symbol) rather than text like "py" or "node". A label
// is either pure text or a single icon, so we check the first non-space
// rune. Icons read better without a chip background.
func isIconLabel(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(trimmed)
	return isIconRune(r)
}

// resolveProcLabelColors picks the effective fg/bg for a sidebar proc label.
// Text labels get a chip background (theme default or, when selected+focused,
// the inverted session-name color so the chip pops against the selection
// row). Icon labels never get a chip background applied by us — but when
// the row is selected+focused, they must adopt the selection bg so the
// icon blends into the row rather than punching a hole to terminal default.
func resolveProcLabelColors(lbl procmatch.Label, selected, focused bool) (fg, bg lipgloss.Color) {
	fg = lipgloss.Color(lbl.FG)
	if fg == "" {
		fg = activeTheme.ColorProcLabelFG
	}
	bg = lipgloss.Color(lbl.BG)

	icon := isIconLabel(lbl.Text)
	highlighted := selected && focused

	switch {
	case highlighted && icon:
		bg = activeTheme.ColorSelected
	case highlighted:
		fg = activeTheme.ColorSelected
		bg = activeTheme.ColorFgPrimary
	case !icon && bg == "":
		bg = activeTheme.ColorProcLabelBG
	}
	return fg, bg
}

// procLabelIndicator renders the configured sidebar process label for a
// session row. Returns "" when the session has no match.
func (s SidebarModel) procLabelIndicator(node SidebarNode, selected, focused bool) string {
	lbl, ok := s.procLabels[node.Session]
	if !ok {
		return ""
	}
	fg, bg := resolveProcLabelColors(lbl, selected, focused)
	style := lipgloss.NewStyle()
	// Empty fg/bg means "leave the attribute unset" so the terminal's default
	// (or the row's underlying background) shows through. Don't apply zero
	// values — lipgloss would force them to black.
	if fg != "" {
		style = style.Foreground(fg)
	}
	if bg != "" {
		style = style.Background(bg)
	}
	return style.Render(lbl.Text)
}

// gitIndicator returns the rendered git status indicator for a sidebar row, or "".
func (s SidebarModel) gitIndicator(node SidebarNode, selected, focused bool) string {
	info, ok := s.gitInfo[node.Session]
	if !ok {
		return ""
	}
	if selected && focused {
		return compactGitIndicatorsOnBG(info, activeTheme.ColorSelected)
	}
	return compactGitIndicators(info)
}

// stateIndicator returns the rendered tool and state indicator for a sidebar row, or "".
func (s SidebarModel) stateIndicator(node SidebarNode, selected, focused bool) string {
	st := s.stateForSession(node.Session)
	if st == nil {
		return ""
	}
	effective := *st
	effective.Value = ageDrivenValue(*st, s.cfg.Tui.DoneIdleAfterSecs)
	if selected && focused {
		return toolStateIndicator(effective, s.cfg, activeTheme.ColorSelected)
	}
	return toolStateIndicator(effective, s.cfg, "")
}

// lastSeenIndicator returns the rendered last-seen age string for a sidebar row, or "".
func (s SidebarModel) lastSeenIndicator(node SidebarNode, selected, focused bool) string {
	if !s.cfg.Sidebar.ShowLastSeen {
		return ""
	}
	sess := s.FindSession(node.Session)
	if sess == nil || sess.Activity.IsZero() {
		return ""
	}
	age := formatAge(sess.Activity, time.Now())
	if selected && focused {
		return hintStyle.Background(activeTheme.ColorSelected).Render(age)
	}
	return hintStyle.Render(age)
}

// watchIndicator returns a fixed-width string for the watch slot.
// When watched: the configured icon. When not watched: blank spaces of the same
// display width, so other indicators never shift position.
// Returns "" when no icon is configured.
func (s SidebarModel) watchIndicator(node SidebarNode, selected, focused bool) string {
	if activeTheme.IconWatch == "" {
		return ""
	}
	iconW := runewidth.StringWidth(activeTheme.IconWatch)
	_, watched := s.watches[node.Session]
	if selected && focused {
		if watched {
			return watchStyle.Background(activeTheme.ColorSelected).Render(activeTheme.IconWatch)
		}
		return selectedBG.Render(strings.Repeat(" ", iconW))
	}
	if watched {
		return watchStyle.Render(activeTheme.IconWatch)
	}
	return strings.Repeat(" ", iconW)
}

// itemIndicator returns an icon reflecting the session's item state:
//   - open TODOs (with or without notes): the configured todo icon
//   - notes only (no open TODOs): ✏
//   - neither: ""
func (s SidebarModel) itemIndicator(node SidebarNode, selected, focused bool) string {
	_, hasOpen := s.itemSessions[node.Session]
	_, hasNotes := s.noteSessions[node.Session]

	switch {
	case hasOpen:
		if activeTheme.IconTodo == "" {
			return ""
		}
		if selected && focused {
			return todoStyle.Background(activeTheme.ColorSelected).Render(activeTheme.IconTodo)
		}
		return todoStyle.Render(activeTheme.IconTodo)
	case hasNotes:
		if activeTheme.IconNote == "" {
			return ""
		}
		if selected && focused {
			return selectedBG.Render(activeTheme.IconNote)
		}
		return activeTheme.IconNote
	default:
		return ""
	}
}

// sessionIndicators assembles the right-side indicator string for a sidebar row.
// Order: [state] [git] [todo] [last-seen] [proc] [watch]
func (s SidebarModel) sessionIndicators(node SidebarNode, selected, focused bool) string {
	var indParts []string
	if ind := s.stateIndicator(node, selected, focused); ind != "" {
		indParts = append(indParts, ind)
	}
	if ind := s.gitIndicator(node, selected, focused); ind != "" {
		indParts = append(indParts, ind)
	}
	if ind := s.itemIndicator(node, selected, focused); ind != "" {
		indParts = append(indParts, ind)
	}
	if ind := s.lastSeenIndicator(node, selected, focused); ind != "" {
		indParts = append(indParts, ind)
	}
	if ind := s.procLabelIndicator(node, selected, focused); ind != "" {
		indParts = append(indParts, ind)
	}
	var other string
	if selected && focused {
		indSep := lipgloss.NewStyle().Background(activeTheme.ColorSelected).Render(" ")
		other = strings.Join(indParts, indSep)
	} else {
		other = strings.Join(indParts, " ")
	}
	return other + s.watchIndicator(node, selected, focused)
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
	return string(runes) + ellipsis
}

// renderSelectedRow renders a sidebar row that is currently selected.
// When focused is true the row is highlighted with background colour and a trail.
func renderSelectedRow(iconPrefix, nameStr, indicators, gap string, availW, indW int, focused bool) string {
	pad := availW - runewidth.StringWidth(nameStr) - indW
	if pad < 0 {
		pad = 0
	}
	if focused {
		indicatorGlyph := selectedSession.Render(focusGlyph)
		trail := lipgloss.NewStyle().Background(activeTheme.ColorSelected).Render(selectedTrail)
		name := selectedBG.Bold(true).Render(nameStr + strings.Repeat(" ", pad))
		spacer := selectedBG.Render(" ")
		styledIcon := styledIconPrefix(iconPrefix, true)
		return indicatorGlyph + gap + spacer + styledIcon + name + indicators + trail
	}
	indicatorGlyph := lipgloss.NewStyle().Foreground(activeTheme.ColorSession).Render(focusGlyph)
	name := selectedInactive.Bold(true).Render(nameStr + strings.Repeat(" ", pad))
	return indicatorGlyph + gap + " " + styledIconPrefix(iconPrefix, false) + name + indicators
}

func (s SidebarModel) renderSession(node SidebarNode, selected, focused bool, width int) string {
	if s.cardView() {
		return s.renderSessionCard(node, selected, selected && focused, width)
	}
	indicators := s.sessionIndicators(node, selected, focused)

	iconPrefix := s.sessionIconPrefix(node)
	iconW := visualWidth(iconPrefix)

	// Row format: [focus(1)] [gap(1)] [active-slot(1)] [icon(iconW)] [name+indicators(availW)]
	// The active-slot is always reserved (indicator or space) so the session icon
	// stays at a fixed column regardless of which icon is configured.
	availW := width - sidebarRowOverhead - iconW
	if availW < minRowWidth {
		availW = minRowWidth
	}
	indW := visualWidth(indicators)
	maxName := availW - indW
	if indW > 0 {
		maxName-- // alignedRow enforces pad>=1 separator; reserve it so truncation doesn't overflow
	}
	if maxName < minRowWidth {
		maxName = minRowWidth
	}
	var nameStr string
	if selected && focused && runewidth.StringWidth(node.Session) > maxName {
		nameStr = s.marquee.View(node.Session, maxName)
	} else {
		nameStr = truncateSessionName(node.Session, maxName)
	}

	gap := s.activeIndicator(node, selected && focused)
	if selected {
		return renderSelectedRow(iconPrefix, nameStr, indicators, gap, availW, indW, focused)
	}
	text := alignedRow(nameStr, indicators, availW)
	return " " + gap + " " + styledIconPrefix(iconPrefix, false) + sessionStyle.Render(text)
}

// renderSessionCard renders a session as a two-row card.
//
//	header: <focus(1)><active(1)> <session_icon> <session_name> <watch> <trail>
//	status: <focus(1)> <state_icon> <state_label> <gap> <proc> <git> <todo> <last_seen>
//
// Rows are joined with "\n". The blank separator row between cards is the
// caller's responsibility (emitted by buildSidebarLines), so this renderer
// stays usable in single-card contexts like the future sticky sidebar.
//
// focused == true means: the cursor is on this session AND the sidebar pane
// has focus. The highlight background is applied to both content rows when
// focused.
func (s SidebarModel) renderSessionCard(node SidebarNode, selected, focused bool, width int) string {
	innerW := width - borderOverhead
	if innerW < minRowWidth+4 {
		innerW = minRowWidth + 4
	}
	header := s.renderCardHeader(node, selected, focused, innerW)
	status := s.renderCardStatus(node, selected, focused, innerW)
	return header + "\n" + status
}

// cardLeadingGlyph returns the column-0 glyph for a card row. Selected cards
// (regardless of pane focus) get the blue focus glyph; everything else gets a
// plain space so unselected cards never look highlighted.
func cardLeadingGlyph(selected, focused bool) string {
	switch {
	case focused:
		return selectedSession.Render(focusGlyph)
	case selected:
		return lipgloss.NewStyle().Foreground(activeTheme.ColorSession).Render(focusGlyph)
	default:
		return " "
	}
}

// activeIconChar returns the configured active-session indicator, falling
// back to the package default when unset.
func (s SidebarModel) activeIconChar() string {
	if icon := s.cfg.Sidebar.ActiveSessionIcon; icon != "" {
		return icon
	}
	return config.DefaultActiveSessionIcon
}

// activeIndicator returns the 1-col cell that goes in the "active session"
// slot of a sidebar row. When the row represents the attached tmux session,
// the configured icon is styled with the session color (with the selected
// background when highlighted). Otherwise a plain space is returned, also
// styled with the highlight background when highlighted so the slot blends
// into the selection bar.
func (s SidebarModel) activeIndicator(node SidebarNode, highlighted bool) string {
	if node.Session == s.activeSession {
		if highlighted {
			return selectedSession.Render(s.activeIconChar())
		}
		return lipgloss.NewStyle().Foreground(activeTheme.ColorSession).Render(s.activeIconChar())
	}
	if highlighted {
		return selectedBG.Render(" ")
	}
	return " "
}

// renderCardHeader builds the top row of a session card: focus + active +
// session icon + name + watch, with the highlight background extended to the
// full innerW when focused.
func (s SidebarModel) renderCardHeader(node SidebarNode, selected, focused bool, innerW int) string {
	active := s.activeIndicator(node, focused)

	iconPrefix := s.sessionIconPrefix(node)
	iconW := visualWidth(iconPrefix)

	// Watch slot (fixed width even when unwatched).
	watch := s.watchIndicator(node, focused, focused)
	watchW := visualWidth(watch)

	// Budget for name: total - focus(1) - active(1) - sep(1 between active & icon) - icon - sep(1) - watch - trail(1).
	overhead := 1 /*focus*/ + 1 /*active*/ + 1 /*sep*/ + iconW + 1 /*sep*/ + watchW + 1 /*trail*/
	maxName := innerW - overhead
	if maxName < minRowWidth {
		maxName = minRowWidth
	}

	var nameStr string
	if focused && runewidth.StringWidth(node.Session) > maxName {
		nameStr = s.marquee.View(node.Session, maxName)
	} else {
		nameStr = truncateSessionName(node.Session, maxName)
	}

	// Compute pad so highlight stretches to innerW on focus.
	used := overhead + visualWidth(nameStr)
	pad := innerW - used
	if pad < 0 {
		pad = 0
	}

	focusGl := cardLeadingGlyph(selected, focused)
	if focused {
		iconStyled := styledIconPrefix(iconPrefix, true)
		nameStyled := selectedBG.Bold(true).Render(nameStr)
		sep := selectedBG.Render(" ")
		trail := selectedBG.Render(strings.Repeat(" ", pad+1)) // +1 for the trailing slot in overhead
		return focusGl + active + sep + iconStyled + nameStyled + sep + watch + trail
	}

	nameStyled := sessionStyle.Render(nameStr)
	return focusGl + active + " " + styledIconPrefix(iconPrefix, false) + nameStyled + " " + watch
}

// renderCardStatus builds the bottom row of a session card: focus +
// state-icon + state-label (left) and proc/git/todo/last-seen (right), with
// justify-between spacing. When the session is idle or has no state, the
// left state group is blank.
func (s SidebarModel) renderCardStatus(node SidebarNode, selected, focused bool, innerW int) string {
	// Left group: state icon + label.
	var leftIcon, leftLabel string
	st := s.stateForSession(node.Session)
	// Gate on the raw state, not the age-driven value: a "done" state that has
	// aged into StateIdle (via ageDrivenValue) must still render its idle icon,
	// matching the full dashboard. Only a genuine StateIdle row blanks the group.
	if st != nil && st.Value.IsDisplayable() {
		effective := *st
		effective.Value = ageDrivenValue(*st, s.cfg.Tui.DoneIdleAfterSecs)
		if focused {
			leftIcon = toolStateIndicator(effective, s.cfg, activeTheme.ColorSelected)
		} else {
			leftIcon = toolStateIndicator(effective, s.cfg, "")
		}
		labelStyle := lipgloss.NewStyle().Foreground(activeTheme.ColorFgSubtext)
		if focused {
			labelStyle = labelStyle.Background(activeTheme.ColorSelected)
		}
		leftLabel = labelStyle.Render(effective.Value.String())
	}

	// Right group: proc, git, todo, last-seen. Joined by a single space, styled
	// when focused so the gap inherits the highlight.
	indicatorFns := []func(SidebarNode, bool, bool) string{
		s.procLabelIndicator, s.gitIndicator, s.itemIndicator, s.lastSeenIndicator,
	}
	var rightParts []string
	for _, fn := range indicatorFns {
		if ind := fn(node, focused, focused); ind != "" {
			rightParts = append(rightParts, ind)
		}
	}
	sep := " "
	if focused {
		sep = selectedBG.Render(" ")
	}
	right := strings.Join(rightParts, sep)

	// If state is blank, the icon column is still reserved as a space so right group alignment
	// stays consistent regardless of state presence.
	leftIconW := visualWidth(leftIcon)
	leftLabelW := visualWidth(leftLabel)
	if leftIconW == 0 {
		leftIcon = " "
		leftIconW = 1
	}
	rightW := visualWidth(right)
	used := 1 /*focus*/ + 1 /*sep*/ + leftIconW + 1 /*sep*/ + leftLabelW
	trailW := 1
	gap := innerW - used - rightW - trailW
	if gap < 1 {
		gap = 1
	}

	focusGl := cardLeadingGlyph(selected, focused)
	if focused {
		sep := selectedBG.Render(" ")
		var leftIconStyled string
		if leftLabel == "" {
			leftIconStyled = selectedBG.Render(leftIcon)
		} else {
			leftIconStyled = leftIcon // stateIconOnBG already applied selected bg
		}
		gapStr := selectedBG.Render(strings.Repeat(" ", gap))
		trail := selectedBG.Render(" ")
		return focusGl + sep + leftIconStyled + sep + leftLabel + gapStr + right + trail
	}

	return focusGl + " " + leftIcon + " " + leftLabel + strings.Repeat(" ", gap) + right + " "
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
		parts = append(parts, gitAheadStyle.Render(fmt.Sprintf("%s%d", gitAheadGlyph, info.Ahead)))
	}
	if info.Behind > 0 {
		parts = append(parts, gitBehindStyle.Render(fmt.Sprintf("%s%d", gitBehindGlyph, info.Behind)))
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
		parts = append(parts, gitAheadStyle.Background(bg).Render(fmt.Sprintf("%s%d", gitAheadGlyph, info.Ahead)))
	}
	if info.Behind > 0 {
		parts = append(parts, gitBehindStyle.Background(bg).Render(fmt.Sprintf("%s%d", gitBehindGlyph, info.Behind)))
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
	heightFn := func(_ int, isLast bool) int { return s.nodeHeight(isLast) }
	if s.cursor < s.offset {
		s.offset = s.cursor
	}
	for s.offset < s.cursor {
		_, fits := countFit(s.offset, len(s.nodes), s.cursor, effective, heightFn)
		if fits {
			break
		}
		s.offset++
	}
	// At the bottom of the list ▼ is absent; reclaim the freed row so the
	// viewport fills without a trailing blank line. With offset > 0 we have
	// visibleRows-1 rows of content (▲ present). Pull offset back as far as
	// possible without dropping the cursor. The first loop above ensures
	// s.offset <= s.cursor, so newOffset starts at or below s.cursor; no extra
	// cursor-bound guard is needed here.
	if s.offset > 0 {
		contentRows := visibleRows - 1
		if contentRows < 1 {
			contentRows = 1
		}
		// Find smallest newOffset such that nodes [newOffset, len-1] fit in contentRows.
		for newOffset := s.offset - 1; newOffset >= 0; newOffset-- {
			consumed, _ := countFit(newOffset, len(s.nodes), s.cursor, contentRows, heightFn)
			if consumed < len(s.nodes) {
				break
			}
			s.offset = newOffset
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

// FirstStateSession returns the display name of the first node (in sorted order)
// that has a recorded state, or "" if none. Used by the state_session focus mode.
func (s *SidebarModel) FirstStateSession() string {
	for _, n := range s.nodes {
		if s.stateForSession(n.Session) != nil {
			return n.Session
		}
	}
	return ""
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
	{FilterWatch, "[@] Watch"},
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

// TickMarquee advances the sidebar marquee by one step.
// Resets to offset 0 when the cursor has moved since the last tick,
// producing a brief pause at each new selection before scrolling begins.
func (s *SidebarModel) TickMarquee() {
	s.marquee.TickTracked(s.cursor, &s.lastMarqueeCursor)
}
