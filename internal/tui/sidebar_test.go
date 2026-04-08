package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/rtalexk/demux/internal/config"
	"github.com/rtalexk/demux/internal/db"
	"github.com/rtalexk/demux/internal/git"
	"github.com/rtalexk/demux/internal/query"
	"github.com/rtalexk/demux/internal/session"
)

// makeNodes builds a flat slice of session-only SidebarNodes for viewport tests.
func makeNodes(n int) []SidebarNode {
	nodes := make([]SidebarNode, n)
	for i := range nodes {
		nodes[i] = SidebarNode{Session: strings.Repeat("s", i+1)}
	}
	return nodes
}

func sidebarWithNodes(nodes []SidebarNode) SidebarModel {
	return SidebarModel{nodes: nodes}
}

// makeSessions builds a []session.Session for sidebar tests from a list of display names.
// Each session is live-only with no panes (sufficient for sort/filter tests).
func makeSessions(names ...string) []session.Session {
	sessions := make([]session.Session, len(names))
	for i, n := range names {
		sessions[i] = session.Session{
			DisplayName: n,
			IsLive:      true,
		}
	}
	return sessions
}

// --- clampViewport ---

func TestClampViewport_cursorAboveOffset(t *testing.T) {
	s := sidebarWithNodes(makeNodes(10))
	s.cursor = 3
	s.offset = 5
	s.clampViewport(4)
	if s.offset != 3 {
		t.Errorf("expected offset=3, got %d", s.offset)
	}
}

func TestClampViewport_cursorBelowWindow(t *testing.T) {
	s := sidebarWithNodes(makeNodes(10))
	s.cursor = 7
	s.offset = 0
	// effective = 4-2 = 2; offset = cursor - effective + 1 = 7-2+1 = 6
	s.clampViewport(4)
	if s.offset != 6 {
		t.Errorf("expected offset=6, got %d", s.offset)
	}
}

func TestClampViewport_cursorWithinWindow(t *testing.T) {
	s := sidebarWithNodes(makeNodes(10))
	s.cursor = 5
	s.offset = 4
	s.clampViewport(4) // window 4-7, cursor 5 is inside
	if s.offset != 4 {
		t.Errorf("expected offset unchanged=4, got %d", s.offset)
	}
}

// --- MoveUp / MoveDown ---

func TestMoveDown_advancesCursorAndClampsViewport(t *testing.T) {
	s := sidebarWithNodes(makeNodes(5))
	s.cursor = 0
	s.offset = 0
	// effective = 3-2 = 1; cursor=1 >= offset(0)+effective(1) → offset=1
	s.MoveDown(3)
	if s.cursor != 1 {
		t.Errorf("expected cursor=1, got %d", s.cursor)
	}
	// cursor=2 >= offset(1)+1 → offset=2
	s.MoveDown(3)
	// cursor=3 >= offset(2)+1 → offset=3
	s.MoveDown(3)
	if s.cursor != 3 {
		t.Errorf("expected cursor=3, got %d", s.cursor)
	}
	if s.offset != 3 {
		t.Errorf("expected offset=3, got %d", s.offset)
	}
}

func TestMoveUp_doesNotGoBelowZero(t *testing.T) {
	s := sidebarWithNodes(makeNodes(5))
	s.cursor = 0
	s.MoveUp(3)
	if s.cursor != 0 {
		t.Errorf("expected cursor=0, got %d", s.cursor)
	}
}

func TestMoveDown_doesNotExceedLastNode(t *testing.T) {
	s := sidebarWithNodes(makeNodes(3))
	s.cursor = 2
	s.MoveDown(5)
	if s.cursor != 2 {
		t.Errorf("expected cursor=2, got %d", s.cursor)
	}
}

// TestClampViewport_noBlankRowAtBottom is the regression test for the blank line
// that appeared when scrolled to the end of the list. clampViewport used
// effective=visibleRows-2, which left one slot vacant (no ▼ hint at bottom).
// The fix slides offset back so the freed slot is filled with content.
func TestClampViewport_noBlankRowAtBottom(t *testing.T) {
	// 10 nodes, visibleRows=4: conservative effective=2 would set offset=8 for
	// cursor=9, leaving only nodes 8,9 visible (2 rows) + ▲ = 3 rows and 1 blank.
	// After fix: offset should be pulled to 7 so nodes 7,8,9 fill all 3 content rows.
	s := sidebarWithNodes(makeNodes(10))
	s.cursor = 9
	s.offset = 0
	s.clampViewport(4)
	if s.offset != 7 {
		t.Errorf("expected offset=7 (fills viewport without blank row), got %d", s.offset)
	}
}

func TestRender_noBlankRowAtBottom(t *testing.T) {
	// Scroll to the last node; the ▲ hint must appear (we're scrolled) and
	// the last three nodes must all be visible — meaning no slot is wasted.
	s := sidebarWithNodes(makeNodes(10))
	s.cursor = 9
	s.clampViewport(4) // must set offset=7 so nodes 7,8,9 fill contentRows=3
	out := renderInner(s, 4)
	if !strings.Contains(out, "▲ more") {
		t.Error("expected ▲ more when scrolled to bottom")
	}
	// nodes[7..9] have names "ssssssss", "sssssssss", "ssssssssss"
	for _, name := range []string{"ssssssss", "sssssssss", "ssssssssss"} {
		if !strings.Contains(out, name) {
			t.Errorf("expected node %q visible at bottom; got:\n%s", name, out)
		}
	}
}

// --- Render: scroll hints ---

func renderInner(s SidebarModel, visibleRows int) string {
	// height = visibleRows + 2 (border), width = 40
	rendered := s.Render(40, visibleRows+2, false, "", "")
	// strip ANSI so we can search plain text
	return stripANSI(rendered)
}

func TestRender_noHintsWhenAllNodesVisible(t *testing.T) {
	s := sidebarWithNodes(makeNodes(3))
	out := renderInner(s, 5)
	if strings.Contains(out, "▲ more") {
		t.Error("unexpected ▲ more when all nodes fit")
	}
	if strings.Contains(out, "▼ more") {
		t.Error("unexpected ▼ more when all nodes fit")
	}
}

func TestRender_belowHintWhenNodesExtendDown(t *testing.T) {
	s := sidebarWithNodes(makeNodes(10))
	s.offset = 0
	out := renderInner(s, 4)
	if !strings.Contains(out, "▼ more") {
		t.Error("expected ▼ more when nodes extend below viewport")
	}
	if strings.Contains(out, "▲ more") {
		t.Error("unexpected ▲ more when offset=0")
	}
}

func TestRender_aboveHintWhenScrolledDown(t *testing.T) {
	s := sidebarWithNodes(makeNodes(10))
	s.offset = 3
	s.cursor = 3
	out := renderInner(s, 4)
	if !strings.Contains(out, "▲ more") {
		t.Error("expected ▲ more when offset > 0")
	}
}

func TestRender_bothHintsWhenScrolledMid(t *testing.T) {
	s := sidebarWithNodes(makeNodes(10))
	s.offset = 3
	s.cursor = 3
	out := renderInner(s, 4)
	if !strings.Contains(out, "▲ more") {
		t.Error("expected ▲ more")
	}
	if !strings.Contains(out, "▼ more") {
		t.Error("expected ▼ more")
	}
}

// TestRender_belowHintWhenScrolledNearBottom is the regression test for the bug
// where hasBelow was checked against offset+visibleRows instead of
// offset+contentRows (after the ▲ hint row was already deducted). When scrolled
// so that ▲ is showing, nodes just off the bottom were miscounted as fitting,
// suppressing ▼ and leaving a blank row instead.
func TestRender_belowHintWhenScrolledNearBottom(t *testing.T) {
	// 10 nodes, visibleRows=4, offset=6 → ▲ costs 1 row → only 3 content rows fit
	// (nodes 6,7,8). Node 9 is still below, so ▼ must appear.
	s := sidebarWithNodes(makeNodes(10))
	s.offset = 6
	s.cursor = 6
	out := renderInner(s, 4)
	if !strings.Contains(out, "▼ more") {
		t.Error("expected ▼ more: node 9 is out of view but hint was suppressed")
	}
	if !strings.Contains(out, "▲ more") {
		t.Error("expected ▲ more when offset=6")
	}
}

// hints must not add extra rows (total content <= visibleRows)
func TestRender_hintsDoNotExceedVisibleRows(t *testing.T) {
	s := sidebarWithNodes(makeNodes(10))
	s.offset = 3
	s.cursor = 3
	visibleRows := 4
	rendered := s.Render(40, visibleRows+2, false, "", "")
	// count newlines in the inner content (strip border lines)
	inner := stripANSI(rendered)
	lines := strings.Split(strings.TrimRight(inner, "\n"), "\n")
	// border adds 2 lines (top + bottom); remaining content lines <= visibleRows
	contentLines := len(lines) - 2
	if contentLines > visibleRows {
		t.Errorf("content lines %d exceed visibleRows %d", contentLines, visibleRows)
	}
}

// --- Name truncation ---

func TestRenderSession_longNameTruncated(t *testing.T) {
	s := sidebarWithNodes([]SidebarNode{
		{Session: "a-very-long-session-name-that-exceeds-width"},
	})
	width := 20
	text := s.renderSession(s.nodes[0], false, false, width)
	runes := []rune(stripANSI(text))
	if len(runes) > width-2 {
		t.Errorf("rendered length %d exceeds available width %d", len(runes), width-2)
	}
	if !strings.Contains(stripANSI(text), "…") {
		t.Error("expected ellipsis in truncated name")
	}
}

func TestRenderSession_truncatedNameWithIndicators_noOverflow(t *testing.T) {
	// Regression: when a name is truncated and indicators are present, the
	// separator space (enforced by alignedRow) must be pre-reserved in maxName
	// so the row does not exceed availW.
	initStyles(Theme{IconTmuxSession: "⊞", IconCfgSession: "⚙︎"}, config.ProcessesConfig{}, nil)
	activity := time.Now().Add(-6 * 24 * time.Hour) // "6d" indicator (3 chars)
	s := SidebarModel{
		sessions: []session.Session{
			{DisplayName: "hf-garmin-credentials-integration", IsLive: true, Activity: activity},
		},
		states:  map[string]db.ToolState{},
		gitInfo: map[string]git.Info{},
		cfg:     config.Config{Sidebar: config.SidebarConfig{ShowLastSeen: true}},
	}
	s.rebuildNodes()
	width := 30
	text := s.renderSession(s.nodes[0], false, false, width)
	runes := []rune(stripANSI(text))
	if len(runes) > width-2 {
		t.Errorf("truncated row length %d exceeds width-2=%d", len(runes), width-2)
	}
	if !strings.Contains(stripANSI(text), "…") {
		t.Error("expected ellipsis in truncated name")
	}
}

func TestRenderSession_shortNameNotTruncated(t *testing.T) {
	s := sidebarWithNodes([]SidebarNode{
		{Session: "short"},
	})
	text := s.renderSession(s.nodes[0], false, false, 40)
	if strings.Contains(stripANSI(text), "…") {
		t.Error("unexpected ellipsis for short name")
	}
}

// --- Right-alignment ---

func TestAlignedRow_indicatorsAtRightEdge(t *testing.T) {
	row := alignedRow("name", "* ↑2", 20)
	plain := stripANSI(row)
	runes := []rune(plain)
	if len(runes) != 20 {
		t.Errorf("expected total width 20, got %d: %q", len(runes), plain)
	}
	if !strings.HasSuffix(plain, "* ↑2") {
		t.Errorf("indicators not at right edge: %q", plain)
	}
}

func TestAlignedRow_noIndicators(t *testing.T) {
	row := alignedRow("session-a", "", 20)
	plain := stripANSI(row)
	// with no indicators the row is just the name (no trailing spaces required)
	if !strings.HasPrefix(plain, "session-a") {
		t.Errorf("unexpected content: %q", plain)
	}
}

func TestAlignedRow_minimumOnePadSpace(t *testing.T) {
	// name + indicators exactly fill availWidth — must still have 1 pad space
	row := alignedRow("name", "ind", 7) // "name"(4) + "ind"(3) = 7, no room
	plain := stripANSI(row)
	if !strings.Contains(plain, " ") {
		t.Errorf("expected at least one space between name and indicators: %q", plain)
	}
}

// --- GotoTop / GotoBottom ---

func TestGotoTop_SetsToFirstAndClampsViewport(t *testing.T) {
	s := sidebarWithNodes(makeNodes(10))
	s.cursor = 8
	s.offset = 5
	s.GotoTop(10)
	if s.cursor != 0 {
		t.Errorf("expected cursor=0, got %d", s.cursor)
	}
	if s.offset != 0 {
		t.Errorf("expected offset=0, got %d", s.offset)
	}
}

func TestGotoBottom_SetsToLastNode(t *testing.T) {
	s := sidebarWithNodes(makeNodes(5))
	s.cursor = 0
	s.GotoBottom(10)
	if s.cursor != 4 {
		t.Errorf("expected cursor=4, got %d", s.cursor)
	}
}

func TestSidebarGotoBottom_EmptyNodes_NoPanic(t *testing.T) {
	s := SidebarModel{}
	s.GotoBottom(10) // must not panic, cursor stays at 0
	if s.cursor != 0 {
		t.Errorf("expected cursor=0, got %d", s.cursor)
	}
}

// --- SessionCount ---

func TestSessionCount_CountsAllNodes(t *testing.T) {
	nodes := []SidebarNode{
		{Session: "a"},
		{Session: "b"},
	}
	s := sidebarWithNodes(nodes)
	if s.SessionCount() != 2 {
		t.Errorf("expected 2, got %d", s.SessionCount())
	}
}

func TestSessionCount_Empty(t *testing.T) {
	s := SidebarModel{}
	if s.SessionCount() != 0 {
		t.Errorf("expected 0, got %d", s.SessionCount())
	}
}

// --- rebuildNodes sorting ---

// TestRebuildNodes_ZeroResultSearch is a regression test for the bug where a
// search with no matches still showed all sessions. When queryResult.Sessions
// is non-nil but empty (active search, zero matches), rebuildNodes must
// produce an empty node list, not the full unfiltered list.
func TestRebuildNodes_ZeroResultSearch(t *testing.T) {
	s := SidebarModel{
		sessions: makeSessions("alpha", "beta", "gamma"),
		states:   map[string]db.ToolState{},
		// Non-nil empty slice = active search with no matches.
		queryResult: query.Result{Sessions: []query.SessionMatch{}},
	}
	s.rebuildNodes()
	if len(s.nodes) != 0 {
		t.Errorf("expected 0 nodes for a zero-result search, got %d", len(s.nodes))
	}
}

func TestRender_NoResultsHint(t *testing.T) {
	s := SidebarModel{
		sessions:    makeSessions("alpha", "beta"),
		states:      map[string]db.ToolState{},
		queryResult: query.Result{Sessions: []query.SessionMatch{}},
	}
	s.rebuildNodes()
	out := s.Render(40, 10, false, "", "")
	if !strings.Contains(stripANSI(out), "no results") {
		t.Errorf("expected 'no results' hint when search returns zero matches, got: %q", stripANSI(out))
	}
}

func TestRebuildNodes_NoAlerts_AlphabeticalOrder(t *testing.T) {
	s := SidebarModel{
		sessions: makeSessions("charlie", "alpha", "beta"),
		states:   map[string]db.ToolState{},
	}
	s.rebuildNodes()
	var got []string
	for _, n := range s.nodes {
		got = append(got, n.Session)
	}
	want := []string{"alpha", "beta", "charlie"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("expected %v, got %v", want, got)
	}
}

func TestRebuildNodes_SessionWithStateSortsFirst(t *testing.T) {
	s := SidebarModel{
		sessions: makeSessions("alpha", "beta"),
		states: map[string]db.ToolState{
			"beta:0.0": {Target: "beta:0.0", Value: db.StateWorking},
		},
	}
	s.rebuildNodes()
	if len(s.nodes) == 0 || s.nodes[0].Session != "beta" {
		t.Errorf("expected beta (has state) first, got %v", s.nodes[0].Session)
	}
}

func TestRebuildNodes_HigherPriorityStateSortsFirst(t *testing.T) {
	// error > working, so vem-main (error) must sort before dm-main (working)
	s := SidebarModel{
		sessions: makeSessions("dm-main", "vem-main"),
		states: map[string]db.ToolState{
			"dm-main:0.0":  {Target: "dm-main:0.0", Value: db.StateWorking},
			"vem-main:0.0": {Target: "vem-main:0.0", Value: db.StateError},
		},
	}
	s.rebuildNodes()
	if len(s.nodes) < 2 {
		t.Fatalf("expected 2 nodes, got %d", len(s.nodes))
	}
	if s.nodes[0].Session != "vem-main" {
		t.Errorf("expected vem-main (error) first, got %q", s.nodes[0].Session)
	}
}

// --- Alert filter ---

func TestRebuildNodes_LastSeenSort(t *testing.T) {
	now := time.Now()
	older := now.Add(-10 * time.Minute)
	s := SidebarModel{
		sessions: []session.Session{
			{DisplayName: "alpha", IsLive: true, Activity: older},
			{DisplayName: "beta", IsLive: true, Activity: now},
		},
		states: map[string]db.ToolState{},
		cfg:    config.Config{Sidebar: config.SidebarConfig{Sort: []string{"last_seen", "priority", "alphabetical"}}},
	}
	s.rebuildNodes()
	var got []string
	for _, n := range s.nodes {
		got = append(got, n.Session)
	}
	// beta is more recent → should appear first
	if len(got) < 2 || got[0] != "beta" {
		t.Errorf("expected beta (more recent) first, got %v", got)
	}
}

func TestRebuildNodes_LastSeenSort_ThenAlpha(t *testing.T) {
	now := time.Now()
	s := SidebarModel{
		sessions: []session.Session{
			{DisplayName: "charlie", IsLive: true, Activity: now},
			{DisplayName: "alpha", IsLive: true, Activity: now},
			{DisplayName: "beta", IsLive: true, Activity: now},
		},
		states: map[string]db.ToolState{},
		// all same activity time → falls through to alpha
		cfg: config.Config{Sidebar: config.SidebarConfig{Sort: []string{"last_seen", "priority", "alphabetical"}}},
	}
	s.rebuildNodes()
	var got []string
	for _, n := range s.nodes {
		got = append(got, n.Session)
	}
	want := []string{"alpha", "beta", "charlie"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("expected %v (alpha tiebreak), got %v", want, got)
	}
}

func TestToggleAlertFilter_FilterOnHidesSessionsWithoutAlerts(t *testing.T) {
	s := SidebarModel{
		sessions: makeSessions("alpha", "beta"),
		states: map[string]db.ToolState{
			"beta:0.0": {Target: "beta:0.0", Value: db.StateWorking},
		},
		cfg: config.Config{Sidebar: config.SidebarConfig{}},
	}
	s.SetFilter(FilterPriority, 10)
	if s.ActiveFilter() != FilterPriority {
		t.Error("expected filter to be FilterPriority")
	}
	for _, n := range s.nodes {
		if n.Session == "alpha" {
			t.Error("expected alpha (no alert) to be hidden")
		}
	}
}

func TestSetFilter_AllFiltersToggle(t *testing.T) {
	s := SidebarModel{
		sessions: makeSessions("alpha", "beta"),
		states:   map[string]db.ToolState{},
		cfg:      config.Config{Sidebar: config.SidebarConfig{}},
		filter:   FilterTmux,
	}
	// Move cursor to "beta" (index 1 after rebuild).
	s.rebuildNodes()
	s.cursor = 1 // "beta"

	// Switch to FilterAll — cursor should follow "beta", prevSession saved as "beta".
	s.SetFilter(FilterAll, 10)
	if s.ActiveFilter() != FilterAll {
		t.Fatalf("expected FilterAll, got %q", s.ActiveFilter())
	}
	// Move to "alpha" in the all-sessions view.
	s.cursor = 0

	// Toggle back — cursor should be restored to "beta".
	s.SetFilter(FilterAll, 10)
	if s.ActiveFilter() != FilterTmux {
		t.Errorf("expected toggle back to FilterTmux, got %q", s.ActiveFilter())
	}
	if len(s.nodes) > 0 && s.nodes[s.cursor].Session != "beta" {
		t.Errorf("expected cursor restored to beta, got %q", s.nodes[s.cursor].Session)
	}

	// After toggling back, pressing FilterAll again should go to FilterAll.
	s.SetFilter(FilterAll, 10)
	if s.ActiveFilter() != FilterAll {
		t.Errorf("expected FilterAll after re-press, got %q", s.ActiveFilter())
	}
}

func TestToggleAlertFilter_ToggleOffRestoresAllSessions(t *testing.T) {
	s := SidebarModel{
		sessions: makeSessions("alpha", "beta"),
		states: map[string]db.ToolState{
			"beta:0.0": {Target: "beta:0.0", Value: db.StateWorking},
		},
		cfg: config.Config{Sidebar: config.SidebarConfig{}},
	}
	s.SetFilter(FilterPriority, 10) // on
	s.SetFilter(FilterPriority, 10) // off (toggles back to FilterTmux)
	if s.ActiveFilter() == FilterPriority {
		t.Error("expected filter to toggle off")
	}
	if len(s.nodes) != 2 {
		t.Errorf("expected 2 sessions after toggle off, got %d", len(s.nodes))
	}
}

func TestAlertFilterActive_ReportsCorrectState(t *testing.T) {
	s := SidebarModel{
		sessions: makeSessions("a"),
		states:   map[string]db.ToolState{},
		cfg:      config.Config{Sidebar: config.SidebarConfig{}},
	}
	if s.ActiveFilter() == FilterPriority {
		t.Error("expected not-FilterPriority before SetFilter")
	}
	s.SetFilter(FilterPriority, 10)
	if s.ActiveFilter() != FilterPriority {
		t.Error("expected FilterPriority after SetFilter")
	}
}

func TestToggleAlertFilter_NoAlertedWindowFallback_CursorClamped(t *testing.T) {
	s := SidebarModel{
		sessions: makeSessions("sess"),
		states:   map[string]db.ToolState{},
		cfg:      config.Config{Sidebar: config.SidebarConfig{}},
	}
	s.cursor = 5
	s.SetFilter(FilterPriority, 10)
	if s.cursor != 0 {
		t.Errorf("expected cursor clamped to 0 on empty node list, got %d", s.cursor)
	}
}

// --- FirstStateSession ---

func TestFirstStateSession_ReturnsFirstWithState(t *testing.T) {
	s := SidebarModel{
		sessions: []session.Session{
			{DisplayName: "alpha", IsLive: true},
			{DisplayName: "beta", IsLive: true},
			{DisplayName: "gamma", IsLive: true},
		},
		states: map[string]db.ToolState{
			"gamma": {Target: "gamma", Value: db.StateWaiting},
		},
		cfg: config.Config{Sidebar: config.SidebarConfig{Sort: []string{"alphabetical"}}},
	}
	s.rebuildNodes()
	got := s.FirstStateSession()
	if got != "gamma" {
		t.Errorf("FirstStateSession() = %q, want %q", got, "gamma")
	}
}

func TestFirstStateSession_PrioritySort(t *testing.T) {
	// With priority sort, the session with the highest-priority state comes first.
	s := SidebarModel{
		sessions: []session.Session{
			{DisplayName: "alpha", IsLive: true},
			{DisplayName: "beta", IsLive: true},
		},
		states: map[string]db.ToolState{
			"alpha": {Target: "alpha", Value: db.StateIdle},
			"beta":  {Target: "beta", Value: db.StateError},
		},
		cfg: config.Config{Sidebar: config.SidebarConfig{Sort: []string{"priority", "alphabetical"}}},
	}
	s.rebuildNodes()
	got := s.FirstStateSession()
	if got != "beta" {
		t.Errorf("FirstStateSession() = %q, want \"beta\" (error > idle)", got)
	}
}

func TestFirstStateSession_NoneReturnsEmpty(t *testing.T) {
	s := SidebarModel{
		sessions: makeSessions("alpha", "beta"),
		states:   map[string]db.ToolState{},
		cfg:      config.Config{Sidebar: config.SidebarConfig{}},
	}
	s.rebuildNodes()
	got := s.FirstStateSession()
	if got != "" {
		t.Errorf("FirstStateSession() = %q, want empty string", got)
	}
}

// --- FocusNode ---

func TestFocusNode_SessionLevel(t *testing.T) {
	s := SidebarModel{
		sessions: []session.Session{
			{DisplayName: "alpha", IsLive: true},
			{DisplayName: "beta", IsLive: true},
		},
		states: map[string]db.ToolState{},
		cfg:    config.Config{Sidebar: config.SidebarConfig{Sort: []string{"alphabetical"}}},
	}
	s.rebuildNodes()
	s.FocusNode("beta", 20)
	node := s.Selected()
	if node == nil || node.Session != "beta" {
		t.Errorf("expected session node beta, got %+v", node)
	}
}

func TestFocusNode_NoMatch_LeavesCursorAt0(t *testing.T) {
	s := SidebarModel{
		sessions: makeSessions("alpha"),
		states:   map[string]db.ToolState{},
	}
	s.rebuildNodes()
	s.FocusNode("nonexistent", 20)
	if s.cursor != 0 {
		t.Errorf("expected cursor=0, got %d", s.cursor)
	}
}

// --- visibleSessions filter tests ---

func TestVisibleSessions_FilterTmux(t *testing.T) {
	s := SidebarModel{
		sessions: []session.Session{
			{DisplayName: "live", IsLive: true},
			{DisplayName: "cfg-only", IsLive: false, IsConfig: true},
		},
		filter: FilterTmux,
	}
	s.rebuildNodes()
	if len(s.nodes) != 1 || s.nodes[0].Session != "live" {
		t.Errorf("FilterTmux should show only live sessions, got %v", s.nodes)
	}
}

func TestVisibleSessions_FilterConfig(t *testing.T) {
	s := SidebarModel{
		sessions: []session.Session{
			{DisplayName: "live", IsLive: true},
			{DisplayName: "cfg-only", IsLive: false, IsConfig: true},
			{DisplayName: "both", IsLive: true, IsConfig: true},
		},
		filter: FilterConfig,
	}
	s.rebuildNodes()
	if len(s.nodes) != 2 {
		t.Errorf("FilterConfig should show IsConfig sessions, got %d", len(s.nodes))
	}
}

func TestVisibleSessions_FilterAll(t *testing.T) {
	s := SidebarModel{
		sessions: []session.Session{
			{DisplayName: "live", IsLive: true},
			{DisplayName: "cfg-only", IsLive: false, IsConfig: true},
		},
		filter: FilterAll,
	}
	s.rebuildNodes()
	if len(s.nodes) != 2 {
		t.Errorf("FilterAll should show all sessions, got %d", len(s.nodes))
	}
}

func TestVisibleSessions_FilterPriority_HidesNoAlert(t *testing.T) {
	s := SidebarModel{
		sessions: []session.Session{
			{DisplayName: "alerted", IsLive: true},
			{DisplayName: "clean", IsLive: true},
		},
		states: map[string]db.ToolState{
			"alerted": {Target: "alerted", Value: db.StateError},
		},
		filter: FilterPriority,
		cfg:    config.Config{},
	}
	s.rebuildNodes()
	if len(s.nodes) != 1 || s.nodes[0].Session != "alerted" {
		t.Errorf("FilterPriority should show only alerted sessions, got %v", s.nodes)
	}
}

func TestFilterPriority_IncludesFlagged(t *testing.T) {
	s := SidebarModel{
		sessions: []session.Session{
			{DisplayName: "flagged", IsLive: true},
			{DisplayName: "clean", IsLive: true},
		},
		states: map[string]db.ToolState{
			"flagged": {Target: "flagged", Value: db.StateFlagged},
		},
		filter: FilterPriority,
		cfg:    config.Config{},
	}
	s.rebuildNodes()
	if len(s.nodes) != 1 || s.nodes[0].Session != "flagged" {
		t.Errorf("FilterPriority must include Flagged sessions, got %v", s.nodes)
	}
}

func TestFilterPriority_ExcludesDone(t *testing.T) {
	s := SidebarModel{
		sessions: []session.Session{
			{DisplayName: "done-sess", IsLive: true},
			{DisplayName: "clean", IsLive: true},
		},
		states: map[string]db.ToolState{
			"done-sess": {Target: "done-sess", Value: db.StateDone},
		},
		filter: FilterPriority,
		cfg:    config.Config{},
	}
	s.rebuildNodes()
	for _, n := range s.nodes {
		if n.Session == "done-sess" {
			t.Error("FilterPriority must NOT include Done sessions")
		}
	}
}

func TestFilterPriority_IncludesWorkingWhenFlagSet(t *testing.T) {
	s := SidebarModel{
		sessions: []session.Session{
			{DisplayName: "working-sess", IsLive: true},
			{DisplayName: "clean", IsLive: true},
		},
		states: map[string]db.ToolState{
			"working-sess": {Target: "working-sess", Value: db.StateWorking},
		},
		filter: FilterPriority,
		cfg:    config.Config{Tui: config.TuiConfig{AttentionFilterIncludeWorking: true}},
	}
	s.rebuildNodes()
	if len(s.nodes) != 1 || s.nodes[0].Session != "working-sess" {
		t.Errorf("FilterPriority with AttentionFilterIncludeWorking must include Working sessions, got %v", s.nodes)
	}
}

func TestFilterPriority_ExcludesWorkingByDefault(t *testing.T) {
	s := SidebarModel{
		sessions: []session.Session{
			{DisplayName: "working-sess", IsLive: true},
		},
		states: map[string]db.ToolState{
			"working-sess": {Target: "working-sess", Value: db.StateWorking},
		},
		filter: FilterPriority,
		cfg:    config.Config{},
	}
	s.rebuildNodes()
	for _, n := range s.nodes {
		if n.Session == "working-sess" {
			t.Error("FilterPriority must NOT include Working sessions by default")
		}
	}
}

func TestVisibleSessions_FilterWorktree_NoRoot(t *testing.T) {
	s := SidebarModel{
		sessions: []session.Session{
			{DisplayName: "no-git", IsLive: true},
		},
		filter:  FilterWorktree,
		gitInfo: map[string]git.Info{},
	}
	s.nodes = []SidebarNode{{Session: "no-git"}} // pre-set cursor target
	s.rebuildNodes()
	if len(s.nodes) != 0 {
		t.Error("expected empty list when no worktree root resolvable")
	}
	out := stripANSI(s.Render(40, 10, false, "", ""))
	if !strings.Contains(out, "no sessions in this worktree") {
		t.Errorf("expected hint message in render output, got: %q", out)
	}
}

func TestVisibleSessions_FilterWorktree_MatchesByRoot(t *testing.T) {
	s := SidebarModel{
		sessions: []session.Session{
			{DisplayName: "sess-a", IsLive: true},
			{DisplayName: "sess-b", IsLive: true},
			{DisplayName: "other", IsLive: true},
		},
		filter: FilterWorktree,
		gitInfo: map[string]git.Info{
			"sess-a": {RepoRoot: "/repo/worktrees/a"},
			"sess-b": {RepoRoot: "/repo/worktrees/b"},
			"other":  {RepoRoot: "/elsewhere/c"},
		},
	}
	// Cursor on sess-a; its root = /repo/worktrees
	s.nodes = []SidebarNode{{Session: "sess-a"}}
	s.rebuildNodes()
	// Both sess-a and sess-b have root /repo/worktrees, other has /elsewhere
	if len(s.nodes) != 2 {
		t.Errorf("expected 2 sessions sharing worktree root, got %d: %v", len(s.nodes), s.nodes)
	}
}

func TestRenderSession_ShowsIcon(t *testing.T) {
	initStyles(Theme{
		IconTmuxSession: "⊞",
		IconCfgSession:  "⚙︎",
		ColorSession:    lipgloss.Color("#89b4fa"),
		ColorBorder:     lipgloss.Color("#313244"),
		ColorSelected:   lipgloss.Color("#2a2a4a"),
		ColorFgMuted:    lipgloss.Color("#9399b2"),
	}, config.ProcessesConfig{}, nil)

	s := SidebarModel{
		sessions: []session.Session{
			{DisplayName: "myapp", IsLive: true, IsConfig: false},
		},
		filter: FilterTmux,
	}
	s.rebuildNodes()
	row := s.renderSession(s.nodes[0], false, false, 40)
	plain := stripANSI(row)
	if !strings.Contains(plain, "⊞") {
		t.Errorf("expected Tmux icon ⊞ in row, got: %q", plain)
	}
}

func TestRenderSession_ShowsConfigIcon(t *testing.T) {
	initStyles(Theme{
		IconTmuxSession: "⊞",
		IconCfgSession:  "⚙︎",
		ColorSession:    lipgloss.Color("#89b4fa"),
		ColorBorder:     lipgloss.Color("#313244"),
		ColorSelected:   lipgloss.Color("#2a2a4a"),
		ColorFgMuted:    lipgloss.Color("#9399b2"),
	}, config.ProcessesConfig{}, nil)

	cfg := session.ConfigEntry{Name: "dotf-main", Group: "dotf", Path: "/foo", Icon: "★"}
	s := SidebarModel{
		sessions: []session.Session{
			{DisplayName: "dotf-main", IsLive: false, IsConfig: true, Config: &cfg},
		},
		filter: FilterConfig,
	}
	s.rebuildNodes()
	row := s.renderSession(s.nodes[0], false, false, 40)
	plain := stripANSI(row)
	if !strings.Contains(plain, "★") {
		t.Errorf("expected config icon ★ in row, got: %q", plain)
	}
}

func TestRenderSession_LiveConfigShowsTmuxIcon(t *testing.T) {
	initStyles(Theme{
		IconTmuxSession: "⊞",
		IconCfgSession:  "⚙︎",
		ColorSession:    lipgloss.Color("#89b4fa"),
		ColorBorder:     lipgloss.Color("#313244"),
		ColorSelected:   lipgloss.Color("#2a2a4a"),
		ColorFgMuted:    lipgloss.Color("#9399b2"),
	}, config.ProcessesConfig{}, nil)

	cfg := session.ConfigEntry{Name: "dotf-main", Group: "dotf", Path: "/foo"}
	s := SidebarModel{
		sessions: []session.Session{
			{DisplayName: "dotf-main", IsLive: true, IsConfig: true, Config: &cfg},
		},
		filter: FilterAll,
	}
	s.rebuildNodes()
	row := s.renderSession(s.nodes[0], false, false, 40)
	plain := stripANSI(row)
	if !strings.Contains(plain, "⊞") {
		t.Errorf("expected Tmux icon ⊞ for live config session, got: %q", plain)
	}
	if strings.Contains(plain, "⚙︎") {
		t.Errorf("expected no cfg icon ⚙︎ for live config session, got: %q", plain)
	}
}

func TestRenderSession_ShowsLastSeen(t *testing.T) {
	initStyles(Theme{
		IconTmuxSession: "⊞",
		IconCfgSession:  "⚙︎",
	}, config.ProcessesConfig{}, nil)
	activity := time.Now().Add(-5 * time.Minute)
	s := SidebarModel{
		sessions: []session.Session{
			{DisplayName: "myses", IsLive: true, Activity: activity},
		},
		states:  map[string]db.ToolState{},
		gitInfo: map[string]git.Info{},
		cfg: config.Config{
			Sidebar: config.SidebarConfig{ShowLastSeen: true},
		},
	}
	s.rebuildNodes()
	row := s.renderSession(s.nodes[0], false, false, 40)
	plain := stripANSI(row)
	if !strings.Contains(plain, " 5m") {
		t.Errorf("expected last-seen ' 5m' in row, got: %q", plain)
	}
}

func TestRenderSession_HidesLastSeenWhenDisabled(t *testing.T) {
	initStyles(Theme{
		IconTmuxSession: "⊞",
		IconCfgSession:  "⚙︎",
	}, config.ProcessesConfig{}, nil)
	activity := time.Now().Add(-5 * time.Minute)
	s := SidebarModel{
		sessions: []session.Session{
			{DisplayName: "xyz", IsLive: true, Activity: activity},
		},
		states:  map[string]db.ToolState{},
		gitInfo: map[string]git.Info{},
		cfg: config.Config{
			Sidebar: config.SidebarConfig{ShowLastSeen: false},
		},
	}
	s.rebuildNodes()
	row := s.renderSession(s.nodes[0], false, false, 40)
	plain := stripANSI(row)
	if strings.Contains(plain, "m") || strings.Contains(plain, "s") || strings.Contains(plain, "h") || strings.Contains(plain, "d") {
		t.Errorf("expected no age indicator when ShowLastSeen=false, got: %q", plain)
	}
}

func TestFilterShortcutBar_FitsWidth(t *testing.T) {
	initStyles(Theme{
		ColorSession: lipgloss.Color("#89b4fa"),
		ColorFgMuted: lipgloss.Color("#9399b2"),
	}, config.ProcessesConfig{}, nil)
	bar := filterShortcutBar(FilterTmux, 50)
	plain := stripANSI(bar)
	if !strings.Contains(plain, "[t]") {
		t.Error("expected [t] in shortcut bar")
	}
	if !strings.Contains(plain, "[a]") {
		t.Error("expected [a] in shortcut bar")
	}
	if !strings.Contains(plain, "[!]") {
		t.Error("expected [!] in shortcut bar at width 50")
	}
}

func TestFilterShortcutBar_TrimmedWhenNarrow(t *testing.T) {
	initStyles(Theme{
		ColorSession: lipgloss.Color("#89b4fa"),
		ColorFgMuted: lipgloss.Color("#9399b2"),
	}, config.ProcessesConfig{}, nil)
	bar := filterShortcutBar(FilterTmux, 12)
	plain := stripANSI(bar)
	if !strings.Contains(plain, "[t]") {
		t.Error("expected [t] to survive trimming")
	}
	if strings.Contains(plain, "[!]") {
		t.Errorf("expected [!] to be trimmed at width 12, got: %q", plain)
	}
}

// --- formatAge ---

func TestFormatAge(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		desc string
		ago  time.Duration
		want string
	}{
		{"0 seconds", 0 * time.Second, "now"},
		{"5 seconds", 5 * time.Second, "now"},
		{"14 seconds", 14 * time.Second, "now"},
		{"15 seconds", 15 * time.Second, "<1m"},
		{"59 seconds", 59 * time.Second, "<1m"},
		{"1 minute", 1 * time.Minute, " 1m"},
		{"9 minutes", 9 * time.Minute, " 9m"},
		{"10 minutes", 10 * time.Minute, "10m"},
		{"59 minutes", 59 * time.Minute, "59m"},
		{"1 hour", 1 * time.Hour, " 1h"},
		{"9 hours", 9 * time.Hour, " 9h"},
		{"10 hours", 10 * time.Hour, "10h"},
		{"23 hours", 23 * time.Hour, "23h"},
		{"1 day", 24 * time.Hour, " 1d"},
		{"9 days", 9 * 24 * time.Hour, " 9d"},
		{"10 days", 10 * 24 * time.Hour, "10d"},
		{"99 days", 99 * 24 * time.Hour, "99d"},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			got := formatAge(now.Add(-tc.ago), now)
			if got != tc.want {
				t.Errorf("formatAge(%v ago): want %q, got %q", tc.ago, tc.want, got)
			}
		})
	}
}

func TestStateValue_Priority_InTUI(t *testing.T) {
	// Verify the priority ordering used by sorting and filtering in the TUI.
	// The canonical implementation lives in db.StateValue.Priority().
	if db.StateWaiting.Priority() <= db.StateError.Priority() {
		t.Error("Waiting must have higher priority than Error")
	}
	if db.StateError.Priority() <= db.StateDone.Priority() {
		t.Error("Error must have higher priority than Done")
	}
	if db.StateDone.Priority() <= db.StateFlagged.Priority() {
		t.Error("Done must have higher priority than Flagged")
	}
	if db.StateFlagged.Priority() <= db.StateWorking.Priority() {
		t.Error("Flagged must have higher priority than Working")
	}
}

func TestSidebarViewport(t *testing.T) {
	tests := []struct {
		name        string
		cursor      int
		offset      int
		visRows     int
		nodeCount   int
		wantOffset  int
		wantContent int
		wantAbove   bool
		wantBelow   bool
	}{
		{"all fit", 0, 0, 10, 5, 0, 10, false, false},
		{"scroll below", 0, 0, 5, 10, 0, 4, false, true},
		{"cursor before offset", 2, 5, 10, 10, 2, 9, true, false},
		{"cursor past viewport", 8, 0, 5, 10, 4, 3, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOffset, gotContent, gotAbove, gotBelow := sidebarViewport(tt.cursor, tt.offset, tt.visRows, tt.nodeCount)
			if gotOffset != tt.wantOffset {
				t.Errorf("offset: got %d, want %d", gotOffset, tt.wantOffset)
			}
			if gotContent != tt.wantContent {
				t.Errorf("contentRows: got %d, want %d", gotContent, tt.wantContent)
			}
			if gotAbove != tt.wantAbove {
				t.Errorf("hasAbove: got %v, want %v", gotAbove, tt.wantAbove)
			}
			if gotBelow != tt.wantBelow {
				t.Errorf("hasBelow: got %v, want %v", gotBelow, tt.wantBelow)
			}
		})
	}
}

func TestSetActiveSession_StoresName(t *testing.T) {
	s := SidebarModel{}
	s.SetActiveSession("myapp")
	if s.activeSession != "myapp" {
		t.Errorf("expected activeSession=myapp, got %q", s.activeSession)
	}
}

func TestSetActiveSession_EmptyClears(t *testing.T) {
	s := SidebarModel{activeSession: "myapp"}
	s.SetActiveSession("")
	if s.activeSession != "" {
		t.Errorf("expected empty activeSession, got %q", s.activeSession)
	}
}

func TestRenderSession_ActiveSessionShowsIndicator(t *testing.T) {
	initStyles(Theme{
		IconTmuxSession: "⊞",
		IconCfgSession:  "⚙︎",
		ColorSession:    lipgloss.Color("#89b4fa"),
		ColorBorder:     lipgloss.Color("#313244"),
		ColorSelected:   lipgloss.Color("#2a2a4a"),
		ColorFgMuted:    lipgloss.Color("#9399b2"),
	}, config.ProcessesConfig{}, nil)

	s := SidebarModel{
		sessions: []session.Session{
			{DisplayName: "myapp", IsLive: true},
		},
		states:        map[string]db.ToolState{},
		gitInfo:       map[string]git.Info{},
		cfg:           config.Config{Sidebar: config.SidebarConfig{ActiveSessionIcon: "►"}},
		activeSession: "myapp",
	}
	s.rebuildNodes()
	row := s.renderSession(s.nodes[0], false, false, 40)
	plain := stripANSI(row)
	if !strings.Contains(plain, "►") {
		t.Errorf("expected ► indicator for active session, got: %q", plain)
	}
}

func TestRenderSession_NonActiveSessionNoIndicator(t *testing.T) {
	initStyles(Theme{
		IconTmuxSession: "⊞",
		IconCfgSession:  "⚙︎",
		ColorSession:    lipgloss.Color("#89b4fa"),
		ColorBorder:     lipgloss.Color("#313244"),
		ColorSelected:   lipgloss.Color("#2a2a4a"),
		ColorFgMuted:    lipgloss.Color("#9399b2"),
	}, config.ProcessesConfig{}, nil)

	s := SidebarModel{
		sessions: []session.Session{
			{DisplayName: "myapp", IsLive: true},
		},
		states:        map[string]db.ToolState{},
		gitInfo:       map[string]git.Info{},
		cfg:           config.Config{Sidebar: config.SidebarConfig{ActiveSessionIcon: "►"}},
		activeSession: "other",
	}
	s.rebuildNodes()
	row := s.renderSession(s.nodes[0], false, false, 40)
	plain := stripANSI(row)
	if strings.Contains(plain, "►") {
		t.Errorf("unexpected ► indicator for non-active session, got: %q", plain)
	}
}

func TestRenderSession_ActiveSessionCustomIcon(t *testing.T) {
	initStyles(Theme{
		IconTmuxSession: "⊞",
		ColorSession:    lipgloss.Color("#89b4fa"),
	}, config.ProcessesConfig{}, nil)

	s := SidebarModel{
		sessions: []session.Session{
			{DisplayName: "myapp", IsLive: true},
		},
		states:        map[string]db.ToolState{},
		gitInfo:       map[string]git.Info{},
		cfg:           config.Config{Sidebar: config.SidebarConfig{ActiveSessionIcon: "@"}},
		activeSession: "myapp",
	}
	s.rebuildNodes()
	row := s.renderSession(s.nodes[0], false, false, 40)
	plain := stripANSI(row)
	if !strings.Contains(plain, "@") {
		t.Errorf("expected @ indicator for active session with custom icon, got: %q", plain)
	}
}

func TestRenderSession_ActiveSessionEmptyIconFallsBackToDefault(t *testing.T) {
	initStyles(Theme{
		IconTmuxSession: "⊞",
		ColorSession:    lipgloss.Color("#89b4fa"),
	}, config.ProcessesConfig{}, nil)

	s := SidebarModel{
		sessions: []session.Session{
			{DisplayName: "myapp", IsLive: true},
		},
		states:        map[string]db.ToolState{},
		gitInfo:       map[string]git.Info{},
		cfg:           config.Config{Sidebar: config.SidebarConfig{ActiveSessionIcon: ""}},
		activeSession: "myapp",
	}
	s.rebuildNodes()
	row := s.renderSession(s.nodes[0], false, false, 40)
	plain := stripANSI(row)
	if !strings.Contains(plain, "►") {
		t.Errorf("expected ► fallback for empty ActiveSessionIcon, got: %q", plain)
	}
}
