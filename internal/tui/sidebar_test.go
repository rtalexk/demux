package tui

import (
    "strings"
    "testing"
    "time"

    "github.com/rtalex/demux/internal/config"
    "github.com/rtalex/demux/internal/db"
    "github.com/rtalex/demux/internal/tmux"
)

// makeNodes builds a flat slice of session-only SidebarNodes for viewport tests.
func makeNodes(n int) []SidebarNode {
    nodes := make([]SidebarNode, n)
    for i := range nodes {
        nodes[i] = SidebarNode{Session: strings.Repeat("s", i+1), IsSession: true}
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
    rendered := s.Render(40, visibleRows+2, false, "")
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
    rendered := s.Render(40, visibleRows+2, false, "")
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
        {Session: "a-very-long-session-name-that-exceeds-width", IsSession: true},
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
        {Session: "short", IsSession: true},
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

func TestSessionCount_CountsOnlySessionNodes(t *testing.T) {
    nodes := []SidebarNode{
        {Session: "a", IsSession: true},
        {Session: "a", WindowIndex: 0},
        {Session: "a", WindowIndex: 1},
        {Session: "b", IsSession: true},
        {Session: "b", WindowIndex: 0},
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
            "sess:0": {Target: "sess:0", CreatedAt: t1},
            "sess:1": {Target: "sess:1", CreatedAt: t2},
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

func TestRebuildNodes_NoAlerts_AlphabeticalOrder(t *testing.T) {
    s := SidebarModel{
        sessions: makeSessions("charlie", "alpha", "beta"),
        alerts:   map[string]db.Alert{},
    }
    s.rebuildNodes()
    var got []string
    for _, n := range s.nodes {
        if n.IsSession {
            got = append(got, n.Session)
        }
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
            "beta:0": {Target: "beta:0", CreatedAt: t1},
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
            "alpha:0": {Target: "alpha:0", CreatedAt: t1},
            "beta:0":  {Target: "beta:0", CreatedAt: t2},
        },
    }
    s.rebuildNodes()
    if len(s.nodes) == 0 || s.nodes[0].Session != "beta" {
        t.Errorf("expected beta (newer alert) first, got %v", s.nodes[0].Session)
    }
}

func TestRebuildNodes_WindowWithAlertSortsFirst(t *testing.T) {
    t1 := time.Now()
    s := SidebarModel{
        sessions: map[string]map[int][]tmux.Pane{
            "sess": {0: nil, 1: nil, 2: nil},
        },
        alerts: map[string]db.Alert{
            "sess:2": {Target: "sess:2", CreatedAt: t1},
        },
    }
    s.rebuildNodes()
    // Find window nodes for "sess"
    var winIdxs []int
    for _, n := range s.nodes {
        if !n.IsSession && n.Session == "sess" {
            winIdxs = append(winIdxs, n.WindowIndex)
        }
    }
    if len(winIdxs) == 0 || winIdxs[0] != 2 {
        t.Errorf("expected window 2 (has alert) first, got %v", winIdxs)
    }
}

// --- Alert filter ---

func TestToggleAlertFilter_FilterOnHidesSessionsWithoutAlerts(t *testing.T) {
    t1 := time.Now()
    s := SidebarModel{
        sessions: makeSessions("alpha", "beta"),
        alerts: map[string]db.Alert{
            "beta:0": {Target: "beta:0", CreatedAt: t1},
        },
        cfg: config.Config{AlertFilterWindows: "all"},
    }
    active := s.ToggleAlertFilter()
    if !active {
        t.Error("expected ToggleAlertFilter to return true (filter now active)")
    }
    for _, n := range s.nodes {
        if n.IsSession && n.Session == "alpha" {
            t.Error("alpha (no alerts) should be hidden when filter is active")
        }
    }
    hasBeta := false
    for _, n := range s.nodes {
        if n.IsSession && n.Session == "beta" {
            hasBeta = true
        }
    }
    if !hasBeta {
        t.Error("beta (has alerts) should be visible when filter is active")
    }
}

func TestToggleAlertFilter_AllWindows_ShowsAllWindowsOfAlertedSession(t *testing.T) {
    t1 := time.Now()
    s := SidebarModel{
        sessions: map[string]map[int][]tmux.Pane{
            "sess": {0: nil, 1: nil},
        },
        alerts: map[string]db.Alert{
            "sess:1": {Target: "sess:1", CreatedAt: t1},
        },
        cfg: config.Config{AlertFilterWindows: "all"},
    }
    s.ToggleAlertFilter()
    var winIdxs []int
    for _, n := range s.nodes {
        if !n.IsSession {
            winIdxs = append(winIdxs, n.WindowIndex)
        }
    }
    if len(winIdxs) != 2 {
        t.Errorf("expected both windows visible with AlertFilterWindows=all, got %v", winIdxs)
    }
}

func TestToggleAlertFilter_AlertsOnly_HidesWindowsWithoutAlert(t *testing.T) {
    t1 := time.Now()
    s := SidebarModel{
        sessions: map[string]map[int][]tmux.Pane{
            "sess": {0: nil, 1: nil},
        },
        alerts: map[string]db.Alert{
            "sess:1": {Target: "sess:1", CreatedAt: t1},
        },
        cfg: config.Config{AlertFilterWindows: "alerts_only"},
    }
    s.ToggleAlertFilter()
    var winIdxs []int
    for _, n := range s.nodes {
        if !n.IsSession {
            winIdxs = append(winIdxs, n.WindowIndex)
        }
    }
    if len(winIdxs) != 1 || winIdxs[0] != 1 {
        t.Errorf("expected only window 1 (has alert) visible with AlertFilterWindows=alerts_only, got %v", winIdxs)
    }
}

func TestToggleAlertFilter_ToggleOffRestoresAllSessions(t *testing.T) {
    t1 := time.Now()
    s := SidebarModel{
        sessions: makeSessions("alpha", "beta"),
        alerts: map[string]db.Alert{
            "beta:0": {Target: "beta:0", CreatedAt: t1},
        },
        cfg: config.Config{AlertFilterWindows: "all"},
    }
    s.ToggleAlertFilter() // on
    active := s.ToggleAlertFilter() // off
    if active {
        t.Error("expected ToggleAlertFilter to return false (filter now inactive)")
    }
    var sessions []string
    for _, n := range s.nodes {
        if n.IsSession {
            sessions = append(sessions, n.Session)
        }
    }
    if len(sessions) != 2 {
        t.Errorf("expected both sessions after toggle off, got %v", sessions)
    }
}

func TestAlertFilterActive_ReportsCorrectState(t *testing.T) {
    s := SidebarModel{
        sessions: makeSessions("a"),
        alerts:   map[string]db.Alert{},
        cfg:      config.Config{AlertFilterWindows: "all"},
    }
    if s.AlertFilterActive() {
        t.Error("expected AlertFilterActive=false before toggle")
    }
    s.ToggleAlertFilter()
    if !s.AlertFilterActive() {
        t.Error("expected AlertFilterActive=true after toggle")
    }
}
