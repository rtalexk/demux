package tui

import (
    "strings"
    "testing"
    "time"

    "github.com/rtalex/demux/internal/config"
    "github.com/rtalex/demux/internal/db"
    "github.com/rtalex/demux/internal/query"
    "github.com/rtalex/demux/internal/tmux"
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

// --- newestSessionAlert ---

func TestNewestSessionAlert_NoAlerts(t *testing.T) {
    s := SidebarModel{alerts: map[string]db.Alert{}}
    if !s.newestSessionAlert("sess").IsZero() {
        t.Error("expected zero time when no alerts for session")
    }
}

func TestNewestSessionAlert_IgnoresDifferentSession(t *testing.T) {
    t1 := time.Now()
    s := SidebarModel{
        alerts: map[string]db.Alert{
            "other:0": {Target: "other:0", CreatedAt: t1},
        },
    }
    if !s.newestSessionAlert("sess").IsZero() {
        t.Error("expected zero time — alert belongs to a different session")
    }
}

func TestNewestSessionAlert_ReturnsNewestAmongWindows(t *testing.T) {
    t1 := time.Now().Add(-10 * time.Second)
    t2 := time.Now()
    s := SidebarModel{
        alerts: map[string]db.Alert{
            "sess:0.0": {Target: "sess:0.0", CreatedAt: t1},
            "sess:1.0": {Target: "sess:1.0", CreatedAt: t2},
        },
    }
    got := s.newestSessionAlert("sess")
    if !got.Equal(t2) {
        t.Errorf("expected newest alert time %v, got %v", t2, got)
    }
}

func TestNewestSessionAlert_MatchesSessionLevelTarget(t *testing.T) {
    t1 := time.Now()
    s := SidebarModel{
        alerts: map[string]db.Alert{
            "sess": {Target: "sess", CreatedAt: t1},
        },
    }
    got := s.newestSessionAlert("sess")
    if !got.Equal(t1) {
        t.Errorf("expected %v, got %v", t1, got)
    }
}

// --- rebuildNodes sorting ---

// makeSessions builds the sessions map used by SidebarModel from a list of names.
// Each session gets a single window at index 0 with no panes (sufficient for sort tests).
func makeSessions(names ...string) map[string]map[int][]tmux.Pane {
    m := make(map[string]map[int][]tmux.Pane, len(names))
    for _, n := range names {
        m[n] = map[int][]tmux.Pane{0: nil}
    }
    return m
}

// TestRebuildNodes_ZeroResultSearch is a regression test for the bug where a
// search with no matches still showed all sessions. When queryResult.Sessions
// is non-nil but empty (active search, zero matches), rebuildNodes must
// produce an empty node list, not the full unfiltered list.
func TestRebuildNodes_ZeroResultSearch(t *testing.T) {
    s := SidebarModel{
        sessions: makeSessions("alpha", "beta", "gamma"),
        alerts:   map[string]db.Alert{},
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
        alerts:      map[string]db.Alert{},
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
        alerts:   map[string]db.Alert{},
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

func TestRebuildNodes_SessionWithAlertSortsFirst(t *testing.T) {
    t1 := time.Now()
    s := SidebarModel{
        sessions: makeSessions("alpha", "beta"),
        alerts: map[string]db.Alert{
            "beta:0.0": {Target: "beta:0.0", CreatedAt: t1},
        },
    }
    s.rebuildNodes()
    if len(s.nodes) == 0 || s.nodes[0].Session != "beta" {
        t.Errorf("expected beta (has alert) first, got %v", s.nodes[0].Session)
    }
}

func TestRebuildNodes_NewestAlertSessionSortsFirst(t *testing.T) {
    t1 := time.Now().Add(-time.Minute)
    t2 := time.Now()
    s := SidebarModel{
        sessions: makeSessions("alpha", "beta"),
        alerts: map[string]db.Alert{
            "alpha:0.0": {Target: "alpha:0.0", CreatedAt: t1},
            "beta:0.0":  {Target: "beta:0.0", CreatedAt: t2},
        },
    }
    s.rebuildNodes()
    if len(s.nodes) == 0 || s.nodes[0].Session != "beta" {
        t.Errorf("expected beta (newer alert) first, got %v", s.nodes[0].Session)
    }
}

// --- Alert filter ---

func TestRebuildNodes_LastSeenSort(t *testing.T) {
    now := time.Now()
    older := now.Add(-10 * time.Minute)
    s := SidebarModel{
        sessions: makeSessions("alpha", "beta"),
        alerts:   map[string]db.Alert{},
        sessionActivity: map[string]time.Time{
            "alpha": older,
            "beta":  now,
        },
        cfg: config.Config{Sidebar: config.SidebarConfig{Sort: []string{"last_seen", "priority", "alphabetical"}}},
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
        sessions: makeSessions("charlie", "alpha", "beta"),
        alerts:   map[string]db.Alert{},
        // all same activity time → falls through to alpha
        sessionActivity: map[string]time.Time{
            "charlie": now,
            "alpha":   now,
            "beta":    now,
        },
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
    t1 := time.Now()
    s := SidebarModel{
        sessions: makeSessions("alpha", "beta"),
        alerts: map[string]db.Alert{
            "beta:0.0": {Target: "beta:0.0", CreatedAt: t1},
        },
        cfg: config.Config{Sidebar: config.SidebarConfig{AlertFilter: "all"}},
    }
    active := s.ToggleAlertFilter(10)
    if !active {
        t.Error("expected ToggleAlertFilter to return true (filter now active)")
    }
    for _, n := range s.nodes {
        if n.Session == "alpha" {
            t.Error("alpha (no alerts) should be hidden when filter is active")
        }
    }
    hasBeta := false
    for _, n := range s.nodes {
        if n.Session == "beta" {
            hasBeta = true
        }
    }
    if !hasBeta {
        t.Error("beta (has alerts) should be visible when filter is active")
    }
}

func TestToggleAlertFilter_ToggleOffRestoresAllSessions(t *testing.T) {
    t1 := time.Now()
    s := SidebarModel{
        sessions: makeSessions("alpha", "beta"),
        alerts: map[string]db.Alert{
            "beta:0.0": {Target: "beta:0.0", CreatedAt: t1},
        },
        cfg: config.Config{Sidebar: config.SidebarConfig{AlertFilter: "all"}},
    }
    s.ToggleAlertFilter(10) // on
    active := s.ToggleAlertFilter(10) // off
    if active {
        t.Error("expected ToggleAlertFilter to return false (filter now inactive)")
    }
    if len(s.nodes) != 2 {
        t.Errorf("expected both sessions after toggle off, got %d nodes", len(s.nodes))
    }
}

func TestAlertFilterActive_ReportsCorrectState(t *testing.T) {
    s := SidebarModel{
        sessions: makeSessions("a"),
        alerts:   map[string]db.Alert{},
        cfg:      config.Config{Sidebar: config.SidebarConfig{AlertFilter: "all"}},
    }
    if s.AlertFilterActive() {
        t.Error("expected AlertFilterActive=false before toggle")
    }
    s.ToggleAlertFilter(10)
    if !s.AlertFilterActive() {
        t.Error("expected AlertFilterActive=true after toggle")
    }
}

func TestToggleAlertFilter_NoAlertedWindowFallback_CursorClamped(t *testing.T) {
    // Filter on with no alerts: cursor should clamp to last valid node.
    s := SidebarModel{
        sessions: makeSessions("sess"),
        alerts:   map[string]db.Alert{},
        cfg:      config.Config{Sidebar: config.SidebarConfig{AlertFilter: "all"}},
    }
    // With no alerts, filter on hides all sessions → nodes is empty.
    s.cursor = 5 // out of range
    s.ToggleAlertFilter(10)
    if s.cursor != 0 {
        t.Errorf("expected cursor clamped to 0 on empty node list, got %d", s.cursor)
    }
}

// --- FocusNode ---

func TestFocusNode_SessionLevel(t *testing.T) {
    s := SidebarModel{
        sessions: map[string]map[int][]tmux.Pane{
            "alpha": {0: nil},
            "beta":  {0: nil, 1: nil},
        },
        alerts: map[string]db.Alert{},
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
        alerts:   map[string]db.Alert{},
    }
    s.rebuildNodes()
    s.FocusNode("nonexistent", 20)
    if s.cursor != 0 {
        t.Errorf("expected cursor=0, got %d", s.cursor)
    }
}

// --- FocusFirstAlertSession ---

func TestFocusFirstAlertSession_MovesToAlertedSession(t *testing.T) {
    t1 := time.Now()
    s := SidebarModel{
        sessions: makeSessions("alpha", "beta"),
        alerts: map[string]db.Alert{
            "beta:0.0": {Target: "beta:0.0", Level: "warn", CreatedAt: t1},
        },
        cfg: config.Config{Sidebar: config.SidebarConfig{Sort: []string{"alphabetical"}}},
    }
    s.rebuildNodes()
    s.FocusFirstAlertSession(20)
    node := s.Selected()
    if node == nil || node.Session != "beta" {
        t.Errorf("expected session node beta, got %+v", node)
    }
}

func TestFocusFirstAlertSession_NoAlerts_LeavesCursorAt0(t *testing.T) {
    s := SidebarModel{
        sessions: makeSessions("alpha", "beta"),
        alerts:   map[string]db.Alert{},
    }
    s.rebuildNodes()
    s.FocusFirstAlertSession(20)
    if s.cursor != 0 {
        t.Errorf("expected cursor=0, got %d", s.cursor)
    }
}

// --- FocusFirstAlertSession returns bool ---

func TestFocusFirstAlertSession_ReturnsTrue_WhenFound(t *testing.T) {
    t1 := time.Now()
    s := SidebarModel{
        sessions: makeSessions("alpha"),
        alerts: map[string]db.Alert{
            "alpha:0.0": {Target: "alpha:0.0", Level: "warn", CreatedAt: t1},
        },
        cfg: config.Config{Sidebar: config.SidebarConfig{Sort: []string{"alphabetical"}}},
    }
    s.rebuildNodes()
    found := s.FocusFirstAlertSession(20)
    if !found {
        t.Error("expected found=true")
    }
}

func TestFocusFirstAlertSession_ReturnsFalse_WhenNotFound(t *testing.T) {
    s := SidebarModel{
        sessions: makeSessions("alpha"),
        alerts:   map[string]db.Alert{},
    }
    s.rebuildNodes()
    found := s.FocusFirstAlertSession(20)
    if found {
        t.Error("expected found=false when no alerted sessions")
    }
}

func makeSidebarWithAlerts(alerts []db.Alert) SidebarModel {
    s := SidebarModel{}
    s.alerts = make(map[string]db.Alert, len(alerts))
    for _, a := range alerts {
        s.alerts[a.Target] = a
    }
    return s
}

func TestBestAlertTargetInSession_NoAlerts(t *testing.T) {
    s := makeSidebarWithAlerts(nil)
    if got := s.BestAlertTargetInSession("work", "severity"); got != "" {
        t.Errorf("expected empty, got %q", got)
    }
}

func TestBestAlertTargetInSession_SingleAlert(t *testing.T) {
    now := time.Now()
    s := makeSidebarWithAlerts([]db.Alert{
        {Target: "work:1.0", Level: db.LevelWarn, CreatedAt: now},
    })
    if got := s.BestAlertTargetInSession("work", "severity"); got != "work:1.0" {
        t.Errorf("expected \"work:1.0\", got %q", got)
    }
}

func TestBestAlertTargetInSession_SeverityPriority(t *testing.T) {
    now := time.Now()
    s := makeSidebarWithAlerts([]db.Alert{
        {Target: "work:1.0", Level: db.LevelWarn,  CreatedAt: now.Add(-time.Minute)},
        {Target: "work:2.0", Level: db.LevelError, CreatedAt: now.Add(-2 * time.Minute)},
        {Target: "work:3.0", Level: db.LevelInfo,  CreatedAt: now},
    })
    if got := s.BestAlertTargetInSession("work", "severity"); got != "work:2.0" {
        t.Errorf("expected \"work:2.0\" (highest severity), got %q", got)
    }
}

func TestBestAlertTargetInSession_SeverityTiebreaker(t *testing.T) {
    base := time.Now()
    s := makeSidebarWithAlerts([]db.Alert{
        {Target: "work:1.0", Level: db.LevelError, CreatedAt: base.Add(-time.Minute)},
        {Target: "work:2.0", Level: db.LevelError, CreatedAt: base},
    })
    // equal severity — newest wins
    if got := s.BestAlertTargetInSession("work", "severity"); got != "work:2.0" {
        t.Errorf("expected \"work:2.0\" (newer tiebreaker), got %q", got)
    }
}

func TestBestAlertTargetInSession_NewestPriority(t *testing.T) {
    base := time.Now()
    s := makeSidebarWithAlerts([]db.Alert{
        {Target: "work:1.0", Level: db.LevelError, CreatedAt: base.Add(-time.Minute)},
        {Target: "work:2.0", Level: db.LevelInfo,  CreatedAt: base},
    })
    if got := s.BestAlertTargetInSession("work", "newest"); got != "work:2.0" {
        t.Errorf("expected \"work:2.0\" (newest), got %q", got)
    }
}

func TestBestAlertTargetInSession_OldestPriority(t *testing.T) {
    base := time.Now()
    s := makeSidebarWithAlerts([]db.Alert{
        {Target: "work:1.0", Level: db.LevelInfo,  CreatedAt: base.Add(-time.Minute)},
        {Target: "work:2.0", Level: db.LevelError, CreatedAt: base},
    })
    if got := s.BestAlertTargetInSession("work", "oldest"); got != "work:1.0" {
        t.Errorf("expected \"work:1.0\" (oldest), got %q", got)
    }
}

func TestBestAlertTargetInSession_OtherSessionIgnored(t *testing.T) {
    now := time.Now()
    s := makeSidebarWithAlerts([]db.Alert{
        {Target: "other:1.0", Level: db.LevelError, CreatedAt: now},
        {Target: "work:2.0",  Level: db.LevelInfo,  CreatedAt: now.Add(-time.Minute)},
    })
    if got := s.BestAlertTargetInSession("work", "severity"); got != "work:2.0" {
        t.Errorf("expected only work session alert, got %q", got)
    }
}

func TestBestAlertTargetInSession_UnknownPriorityFallsBackToSeverity(t *testing.T) {
    base := time.Now()
    s := makeSidebarWithAlerts([]db.Alert{
        {Target: "work:1.0", Level: db.LevelWarn,  CreatedAt: base.Add(-time.Minute)},
        {Target: "work:2.0", Level: db.LevelError, CreatedAt: base.Add(-2 * time.Minute)},
    })
    if got := s.BestAlertTargetInSession("work", "bogus"); got != "work:2.0" {
        t.Errorf("expected severity fallback to return \"work:2.0\", got %q", got)
    }
}

func TestBestAlertTargetInSession_DefaultPriority(t *testing.T) {
    now := time.Now()
    s := makeSidebarWithAlerts([]db.Alert{
        {Target: "work:1.0", Level: db.LevelError, CreatedAt: now},
        {Target: "work:2.0", Level: db.LevelWarn,  CreatedAt: now.Add(-time.Minute)},
    })
    // "default" must return "" regardless of what alerts exist
    if got := s.BestAlertTargetInSession("work", "default"); got != "" {
        t.Errorf("expected \"\" for default priority, got %q", got)
    }
}
