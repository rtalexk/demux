package tui

import (
    "strings"
    "testing"
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

// --- Render: scroll hints ---

func renderInner(s SidebarModel, visibleRows int) string {
    // height = visibleRows + 2 (border), width = 40
    rendered := s.Render(40, visibleRows+2, false)
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

// hints must not add extra rows (total content <= visibleRows)
func TestRender_hintsDoNotExceedVisibleRows(t *testing.T) {
    s := sidebarWithNodes(makeNodes(10))
    s.offset = 3
    s.cursor = 3
    visibleRows := 4
    rendered := s.Render(40, visibleRows+2, false)
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
