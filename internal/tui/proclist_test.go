package tui

import (
    "testing"

    "github.com/rtalex/demux/internal/proc"
    "github.com/rtalex/demux/internal/tmux"
)

// buildNodes constructs a ProcListNode slice for use in tests.
// Layout: two panes, each with one depth-1 process and one depth-2 subprocess.
//
//    [0] pane header  (depth 0)
//    [1] proc A       (depth 1)
//    [2] proc A.child (depth 2)
//    [3] pane header  (depth 0)
//    [4] proc B       (depth 1)
//    [5] proc B.child (depth 2)
func buildNodes() []ProcListNode {
    return []ProcListNode{
        {IsPaneHeader: true, Pane: tmux.Pane{PaneIndex: 0}, Depth: 0},
        {Proc: proc.Process{PID: 1, Name: "procA"}, Depth: 1},
        {Proc: proc.Process{PID: 2, Name: "procA-child"}, Depth: 2},
        {IsPaneHeader: true, Pane: tmux.Pane{PaneIndex: 1}, Depth: 0},
        {Proc: proc.Process{PID: 3, Name: "procB"}, Depth: 1},
        {Proc: proc.Process{PID: 4, Name: "procB-child"}, Depth: 2},
    }
}

func modelAt(nodes []ProcListNode, cursor int) ProcListModel {
    return ProcListModel{nodes: nodes, cursor: cursor}
}

// ---------- MoveUp tests ----------

func TestMoveUp_MovesLinearlyUp(t *testing.T) {
    m := modelAt(buildNodes(), 3) // cursor on second pane header
    m.MoveUp()
    if m.cursor != 2 {
        t.Errorf("expected cursor 2, got %d", m.cursor)
    }
}

func TestMoveUp_AtFirstNode_NoMove(t *testing.T) {
    m := modelAt(buildNodes(), 0)
    m.MoveUp()
    if m.cursor != 0 {
        t.Errorf("expected cursor to stay at 0, got %d", m.cursor)
    }
}

func TestMoveUp_CrossesPaneHeaderLinearly(t *testing.T) {
    // cursor at procB (index 4) — linear MoveUp goes to pane header at index 3
    m := modelAt(buildNodes(), 4)
    m.MoveUp()
    if m.cursor != 3 {
        t.Errorf("expected cursor 3, got %d", m.cursor)
    }
}

func TestMoveUp_CrossesDepthBoundaryLinearly(t *testing.T) {
    // cursor at procB-child (index 5) — linear MoveUp goes to procB (index 4)
    m := modelAt(buildNodes(), 5)
    m.MoveUp()
    if m.cursor != 4 {
        t.Errorf("expected cursor 4, got %d", m.cursor)
    }
}

// ---------- MoveDown tests ----------

func TestMoveDown_MovesLinearlyDown(t *testing.T) {
    m := modelAt(buildNodes(), 0)
    m.MoveDown()
    if m.cursor != 1 {
        t.Errorf("expected cursor 1, got %d", m.cursor)
    }
}

func TestMoveDown_AtLastNode_NoMove(t *testing.T) {
    nodes := buildNodes()
    m := modelAt(nodes, len(nodes)-1)
    m.MoveDown()
    if m.cursor != len(nodes)-1 {
        t.Errorf("expected cursor to stay at %d, got %d", len(nodes)-1, m.cursor)
    }
}

func TestMoveDown_CrossesPaneHeaderLinearly(t *testing.T) {
    // cursor at procA-child (index 2) — linear MoveDown goes to pane header at index 3
    m := modelAt(buildNodes(), 2)
    m.MoveDown()
    if m.cursor != 3 {
        t.Errorf("expected cursor 3, got %d", m.cursor)
    }
}

// ---------- TabNext tests ----------

func TestTabNext_Depth0_WrapsAcrossHeaders(t *testing.T) {
    m := modelAt(buildNodes(), 0)
    m.TabNext()
    if m.cursor != 3 {
        t.Errorf("TabNext from first header: expected 3, got %d", m.cursor)
    }
    m.TabNext()
    if m.cursor != 0 {
        t.Errorf("TabNext from last header should wrap to 0, got %d", m.cursor)
    }
}

func TestTabNext_Depth1_WrapsWithinPane(t *testing.T) {
    nodes := buildNodes()
    // Insert a second depth-1 process into pane 0.
    nodes = append(nodes[:2], append([]ProcListNode{
        {Proc: proc.Process{PID: 11, Name: "procA2"}, Depth: 1},
    }, nodes[2:]...)...)
    // nodes: [0]=pane0, [1]=procA, [2]=procA2, [3]=procA-child, [4]=pane1, ...
    m := modelAt(nodes, 1)
    m.TabNext()
    if m.cursor != 2 {
        t.Errorf("TabNext depth-1: expected 2, got %d", m.cursor)
    }
    m.TabNext()
    if m.cursor != 1 {
        t.Errorf("TabNext depth-1 wrap: expected 1, got %d", m.cursor)
    }
}

func TestTabNext_EmptyNodes_NoOp(t *testing.T) {
    m := ProcListModel{}
    m.TabNext() // should not panic
}

func TestTabNext_SingleNode_StaysInPlace(t *testing.T) {
    m := modelAt([]ProcListNode{{IsPaneHeader: true, Pane: tmux.Pane{PaneIndex: 0}}}, 0)
    m.TabNext()
    if m.cursor != 0 {
        t.Errorf("expected cursor to stay at 0 with single node, got %d", m.cursor)
    }
}

// buildIdleNodes creates a node list where pane 0 has an idle placeholder.
//
//    [0] pane header  (depth 0)
//    [1] idle         (depth 1, IsIdle=true)
//    [2] pane header  (depth 0)
//    [3] proc B       (depth 1)
func buildIdleNodes() []ProcListNode {
    return []ProcListNode{
        {IsPaneHeader: true, Pane: tmux.Pane{PaneIndex: 0}, Depth: 0},
        {IsIdle: true, Depth: 1},
        {IsPaneHeader: true, Pane: tmux.Pane{PaneIndex: 1}, Depth: 0},
        {Proc: proc.Process{PID: 3, Name: "procB"}, Depth: 1},
    }
}

// ---------- isSelectable ----------

func TestIsSelectable_FalseForIdleNode(t *testing.T) {
    if isSelectable(ProcListNode{IsIdle: true}) {
        t.Error("idle node should not be selectable")
    }
}

func TestIsSelectable_TrueForProcNode(t *testing.T) {
    if !isSelectable(ProcListNode{Proc: proc.Process{PID: 1}}) {
        t.Error("process node should be selectable")
    }
}

func TestIsSelectable_TrueForPaneHeader(t *testing.T) {
    if !isSelectable(ProcListNode{IsPaneHeader: true}) {
        t.Error("pane header should be selectable")
    }
}

// ---------- Idle node skipping in MoveUp/MoveDown ----------

func TestMoveDown_SkipsIdleNode(t *testing.T) {
    nodes := buildIdleNodes()
    m := modelAt(nodes, 0) // cursor on first pane header
    m.MoveDown()
    // idle at [1] should be skipped; next selectable is pane header at [2]
    if m.cursor != 2 {
        t.Errorf("expected cursor=2 (skip idle), got %d", m.cursor)
    }
}

func TestMoveUp_SkipsIdleNode(t *testing.T) {
    nodes := buildIdleNodes()
    m := modelAt(nodes, 2) // cursor on second pane header
    m.MoveUp()
    // going up from [2]: [1] is idle (skip), [0] is pane header (selectable)
    if m.cursor != 0 {
        t.Errorf("expected cursor=0 (skip idle), got %d", m.cursor)
    }
}

func TestMoveDown_StopsAtLastSelectableNode(t *testing.T) {
    // when last node is idle, MoveDown should not move past the last selectable
    nodes := []ProcListNode{
        {IsPaneHeader: true, Pane: tmux.Pane{PaneIndex: 0}},
        {Proc: proc.Process{PID: 1, Name: "procA"}, Depth: 1},
        {IsIdle: true, Depth: 1},
    }
    m := modelAt(nodes, 1) // cursor on procA
    m.MoveDown()
    // idle at [2] is not selectable — cursor stays at [1]
    if m.cursor != 1 {
        t.Errorf("expected cursor=1 (no selectable node below), got %d", m.cursor)
    }
}

// ---------- GotoTop / GotoBottom ----------

func TestGotoTop_SetsCursorAndOffsetToZero(t *testing.T) {
    m := modelAt(buildNodes(), 5)
    m.offset = 3
    m.GotoTop()
    if m.cursor != 0 {
        t.Errorf("expected cursor=0, got %d", m.cursor)
    }
    if m.offset != 0 {
        t.Errorf("expected offset=0, got %d", m.offset)
    }
}

func TestGotoBottom_MovesToLastSelectableNode(t *testing.T) {
    nodes := buildNodes()
    m := modelAt(nodes, 0)
    m.GotoBottom()
    if m.cursor != len(nodes)-1 {
        t.Errorf("expected cursor=%d, got %d", len(nodes)-1, m.cursor)
    }
}

func TestGotoBottom_SkipsTrailingIdleNode(t *testing.T) {
    nodes := []ProcListNode{
        {IsPaneHeader: true, Pane: tmux.Pane{PaneIndex: 0}},
        {Proc: proc.Process{PID: 1, Name: "procA"}, Depth: 1},
        {IsIdle: true, Depth: 1},
    }
    m := modelAt(nodes, 0)
    m.GotoBottom()
    // procA at [1] is the last selectable; idle at [2] must be skipped
    if m.cursor != 1 {
        t.Errorf("expected cursor=1, got %d", m.cursor)
    }
}

func TestGotoBottom_EmptyNodes_NoPanic(t *testing.T) {
    m := ProcListModel{}
    m.GotoBottom() // should not panic
}

// ---------- SelectedNode ----------

func TestSelectedNode_ReturnsNilForIdleNode(t *testing.T) {
    nodes := []ProcListNode{
        {IsPaneHeader: true, Pane: tmux.Pane{PaneIndex: 0}},
        {IsIdle: true, Depth: 1},
    }
    m := modelAt(nodes, 1) // cursor on idle node
    if m.SelectedNode() != nil {
        t.Error("expected nil for idle node")
    }
}

func TestSelectedNode_ReturnsProcNode(t *testing.T) {
    nodes := buildNodes()
    m := modelAt(nodes, 1) // cursor on procA
    n := m.SelectedNode()
    if n == nil {
        t.Fatal("expected non-nil node")
    }
    if n.Proc.PID != 1 {
        t.Errorf("expected PID=1, got %d", n.Proc.PID)
    }
}

func TestSelectedNode_ReturnsNilForEmptyModel(t *testing.T) {
    m := ProcListModel{}
    if m.SelectedNode() != nil {
        t.Error("expected nil for empty model")
    }
}

// ---------- nodeRows ----------

func TestNodeRows_PaneHeaderIsOneRow(t *testing.T) {
    if nodeRows(ProcListNode{IsPaneHeader: true}) != 1 {
        t.Error("expected 1 row for pane header")
    }
}

func TestNodeRows_IdleIsOneRow(t *testing.T) {
    if nodeRows(ProcListNode{IsIdle: true}) != 1 {
        t.Error("expected 1 row for idle placeholder")
    }
}

func TestNodeRows_ProcIsTwoRows(t *testing.T) {
    if nodeRows(ProcListNode{Proc: proc.Process{PID: 1}}) != 2 {
        t.Error("expected 2 rows for process node")
    }
}

// ---------- clampOffset ----------

func TestClampOffset_EmptyNodes(t *testing.T) {
    m := ProcListModel{}
    m.clampOffset(10) // must not panic
    if m.offset != 0 {
        t.Errorf("expected offset=0 for empty nodes, got %d", m.offset)
    }
}

func TestClampOffset_CursorFitsInViewport_NoChange(t *testing.T) {
    // cursor=1, only 2 nodes, large maxRows — offset should stay 0
    nodes := buildNodes()
    m := modelAt(nodes, 1)
    m.offset = 0
    m.clampOffset(20)
    if m.offset != 0 {
        t.Errorf("expected offset=0 when cursor fits, got %d", m.offset)
    }
}

func TestClampOffset_AdvancesOffsetWhenCursorRowsExceedAvailable(t *testing.T) {
    // nodes: pane header (1 row) + 4 processes (2 rows each) = 9 rows to last cursor
    // maxRows=6 → available=4; cursor at last process (index 5) won't fit without scrolling
    nodes := buildNodes()
    m := modelAt(nodes, 5)
    m.offset = 0
    m.clampOffset(6)
    if m.offset == 0 {
        t.Error("expected offset to advance when cursor is far below viewport")
    }
    // verify cursor is within available rows from new offset
    available := 6 - 2
    rows := 0
    for i := m.offset; i <= m.cursor; i++ {
        rows += nodeRows(nodes[i])
    }
    if rows > available {
        t.Errorf("cursor rows %d still exceed available %d after clamp", rows, available)
    }
}

func TestClampOffset_CursorAboveOffset_ClampsUp(t *testing.T) {
    nodes := buildNodes()
    m := modelAt(nodes, 1)
    m.offset = 3 // cursor is above viewport
    m.clampOffset(10)
    if m.offset != 1 {
        t.Errorf("expected offset=cursor=1 when cursor above viewport, got %d", m.offset)
    }
}

// ---------- aggStats ----------

func TestAggStats_LeafNode_ReturnsSelf(t *testing.T) {
    tree := map[int32][]proc.Process{}
    cpu, mem := aggStats(1, proc.Process{PID: 1, CPU: 2.5, MemRSS: 1024}, tree)
    if cpu != 2.5 {
        t.Errorf("expected cpu=2.5, got %.2f", cpu)
    }
    if mem != 1024 {
        t.Errorf("expected mem=1024, got %d", mem)
    }
}

func TestAggStats_WithChildren_IncludesDescendants(t *testing.T) {
    parent := proc.Process{PID: 10, CPU: 1.0, MemRSS: 100}
    child1 := proc.Process{PID: 11, CPU: 0.5, MemRSS: 50}
    child2 := proc.Process{PID: 12, CPU: 0.3, MemRSS: 30}
    grandchild := proc.Process{PID: 13, CPU: 0.2, MemRSS: 20}
    tree := map[int32][]proc.Process{
        10: {child1, child2},
        11: {grandchild},
    }
    cpu, mem := aggStats(10, parent, tree)
    if cpu != 2.0 {
        t.Errorf("expected cpu=2.0, got %.2f", cpu)
    }
    if mem != 200 {
        t.Errorf("expected mem=200, got %d", mem)
    }
}
