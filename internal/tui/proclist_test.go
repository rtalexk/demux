package tui

import (
    "strings"
    "testing"

    "github.com/rtalex/demux/internal/config"
    "github.com/rtalex/demux/internal/db"
    "github.com/rtalex/demux/internal/git"
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

func TestNodeRows_WindowHeader_IsOneRow(t *testing.T) {
    if nodeRows(ProcListNode{IsWindowHeader: true}) != 1 {
        t.Error("expected window header to occupy 1 row")
    }
}

func TestIsSelectable_WindowHeader_IsTrue(t *testing.T) {
    if !isSelectable(ProcListNode{IsWindowHeader: true}) {
        t.Error("expected window header to be selectable")
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
    // nodes: pane header (1 row) + 4 processes (2 rows each) = 10 rows total
    // maxRows=6 → cursor at last process (index 5) won't fit without scrolling
    nodes := buildNodes()
    m := modelAt(nodes, 5)
    m.offset = 0
    m.clampOffset(6)
    if m.offset == 0 {
        t.Error("expected offset to advance when cursor is far below viewport")
    }
    // verify cursor is visible according to the same hint logic used by Render
    if !procCursorVisible(nodes, m.cursor, m.offset, 6) {
        t.Errorf("cursor not visible after clamp: offset=%d cursor=%d", m.offset, m.cursor)
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

// ---------- procCursorVisible ----------

// buildHeaderNodes returns n pane-header nodes (1 row each) for simple
// viewport arithmetic in scroll tests.
func buildHeaderNodes(n int) []ProcListNode {
    nodes := make([]ProcListNode, n)
    for i := range nodes {
        nodes[i] = ProcListNode{IsPaneHeader: true}
    }
    return nodes
}

// TestProcCursorVisible_SecondToLastTriggersHasBelow is the regression test for
// the bug where the first pass broke early at the cursor node and never checked
// whether nodes after the cursor would cause a ▼ hint — making the cursor
// appear visible when Render would actually hide it behind the hint row.
func TestProcCursorVisible_SecondToLastTriggersHasBelow(t *testing.T) {
    // 9 single-row nodes, viewport fits 8. cursor=7 (second to last).
    // Node 8 overflows → ▼ hint → effective content = 7 → cursor=7 is NOT visible.
    nodes := buildHeaderNodes(9)
    if procCursorVisible(nodes, 7, 0, 8) {
        t.Error("cursor=7 (second to last of 9) should NOT be visible: node 8 causes ▼ hint that cuts it off")
    }
}

func TestProcCursorVisible_LastNodeNoHasBelow(t *testing.T) {
    // cursor at the very last node — no ▼ hint, so it should be visible
    // even though it fills the viewport exactly.
    nodes := buildHeaderNodes(8)
    if !procCursorVisible(nodes, 7, 0, 8) {
        t.Error("cursor at last node should be visible when all nodes fit exactly")
    }
}

func TestProcCursorVisible_AllFitNoScroll(t *testing.T) {
    // Entire list fits in viewport with no hints needed.
    nodes := buildHeaderNodes(5)
    if !procCursorVisible(nodes, 4, 0, 10) {
        t.Error("cursor should be visible when list easily fits in viewport")
    }
}

// ---------- Render downward safety clamp ----------

// TestRender_SafetyClampWhenViewportShrinks is the regression test for the bug
// where moving the cursor to a process node caused the detail pane to expand,
// shrinking procH. The stale p.offset (valid for the larger viewport) left the
// cursor outside the visible area. Render must re-clamp read-only.
func TestRender_SafetyClampWhenViewportShrinks(t *testing.T) {
    nodes := buildNodes() // 6 nodes, 10 rows total
    m := modelAt(nodes, 5)
    // Simulate offset that was valid for a tall viewport but is now stale.
    m.offset = 0

    // height=6 → maxRows=4 (tiny viewport, cursor=5 can't fit at offset=0)
    rendered := m.Render(40, 6, true, "")
    plain := stripANSI(rendered)

    // cursor is procB-child (PID 4); its name must appear in the rendered output
    if !strings.Contains(plain, "procB-child") {
        t.Errorf("cursor node 'procB-child' should be visible after read-only safety clamp; got:\n%s", plain)
    }
}

// ---------- aggStats ----------

func TestAggStats_LeafNode_ReturnsSelf(t *testing.T) {
    tree := map[int32][]proc.Process{}
    cpu, mem := aggStats(proc.Process{PID: 1, CPU: 2.5, MemRSS: 1024}, tree)
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
    cpu, mem := aggStats(parent, tree)
    if cpu != 2.0 {
        t.Errorf("expected cpu=2.0, got %.2f", cpu)
    }
    if mem != 200 {
        t.Errorf("expected mem=200, got %d", mem)
    }
}

// ---------- SetWindowData collapse defaults ----------

func buildTestPane(panePID int32) tmux.Pane {
    return tmux.Pane{
        Session:     "test",
        WindowIndex: 0,
        PaneIndex:   0,
        PanePID:     panePID,
    }
}

func TestSetWindowData_CollapseHidesChildren(t *testing.T) {
    shell := proc.Process{PID: 200, PPID: 0, Name: "zsh"}
    parent := proc.Process{PID: 201, PPID: 200, Name: "node"}
    child := proc.Process{PID: 202, PPID: 201, Name: "esbuild"}
    procs := []proc.Process{shell, parent, child}

    pane := buildTestPane(200)
    var m ProcListModel
    m.SetWindowData(
        []tmux.Pane{pane}, "test", 0,
        procs, map[int32]string{},
        map[string]git.Info{}, nil, config.Config{},
    )

    // esbuild (depth-2 child of node) should NOT appear — parent collapsed by default
    for _, n := range m.nodes {
        if n.Proc.PID == 202 {
            t.Error("esbuild (depth-2 child) should be hidden when parent is collapsed by default")
        }
    }

    // node (PID 201) should have HasChildren=true and Collapsed=true
    var nodeProc *ProcListNode
    for i := range m.nodes {
        if m.nodes[i].Proc.PID == 201 {
            nodeProc = &m.nodes[i]
        }
    }
    if nodeProc == nil {
        t.Fatal("node proc (PID 201) not found in node list")
    }
    if !nodeProc.HasChildren {
        t.Error("expected HasChildren=true for node with child esbuild")
    }
    if !nodeProc.Collapsed {
        t.Error("expected Collapsed=true by default")
    }
}

func TestSetWindowData_ExpandedShowsChildren(t *testing.T) {
    shell := proc.Process{PID: 300, PPID: 0, Name: "zsh"}
    parent := proc.Process{PID: 301, PPID: 300, Name: "node"}
    child := proc.Process{PID: 302, PPID: 301, Name: "esbuild"}
    procs := []proc.Process{shell, parent, child}

    pane := buildTestPane(300)
    var m ProcListModel
    // First call establishes the window and defaults PID 301 to collapsed.
    m.SetWindowData(
        []tmux.Pane{pane}, "test", 0,
        procs, map[int32]string{},
        map[string]git.Info{}, nil, config.Config{},
    )
    // Simulate a toggle — expand PID 301.
    m.collapsedPIDs[301] = false
    // Second call for the same window (windowChanged=false) — map is preserved.
    m.SetWindowData(
        []tmux.Pane{pane}, "test", 0,
        procs, map[int32]string{},
        map[string]git.Info{}, nil, config.Config{},
    )

    found := false
    for _, n := range m.nodes {
        if n.Proc.PID == 302 {
            found = true
        }
    }
    if !found {
        t.Error("esbuild (depth-2) should be visible when parent is expanded")
    }
}

func TestSetWindowData_AggStats_SetOnCollapsedNode(t *testing.T) {
    shell := proc.Process{PID: 400, PPID: 0, Name: "zsh"}
    parent := proc.Process{PID: 401, PPID: 400, Name: "node", CPU: 1.0, MemRSS: 100}
    child := proc.Process{PID: 402, PPID: 401, Name: "esbuild", CPU: 0.5, MemRSS: 50}
    procs := []proc.Process{shell, parent, child}

    pane := buildTestPane(400)
    var m ProcListModel
    m.SetWindowData(
        []tmux.Pane{pane}, "test", 0,
        procs, map[int32]string{},
        map[string]git.Info{}, nil, config.Config{},
    )

    var nodeProc *ProcListNode
    for i := range m.nodes {
        if m.nodes[i].Proc.PID == 401 {
            nodeProc = &m.nodes[i]
        }
    }
    if nodeProc == nil {
        t.Fatal("node proc (PID 401) not found")
    }
    if !nodeProc.HasChildren {
        t.Error("expected HasChildren=true for node with child esbuild")
    }
    if !nodeProc.Collapsed {
        t.Error("expected Collapsed=true by default")
    }
    if nodeProc.AggCPU != 1.5 {
        t.Errorf("expected AggCPU=1.5, got %.2f", nodeProc.AggCPU)
    }
    if nodeProc.AggMemRSS != 150 {
        t.Errorf("expected AggMemRSS=150, got %d", nodeProc.AggMemRSS)
    }
}

// ---------- ToggleCollapse ----------

func TestToggleCollapse_Depth1WithChildren_TogglesAndReturnsTrue(t *testing.T) {
    m := ProcListModel{
        collapsedPIDs: map[int32]bool{10: true},
        nodes: []ProcListNode{
            {IsPaneHeader: true, Pane: tmux.Pane{PaneIndex: 0}},
            {Proc: proc.Process{PID: 10}, Depth: 1, HasChildren: true, Collapsed: true},
        },
        cursor: 1,
    }
    toggled := m.ToggleCollapse()
    if !toggled {
        t.Error("expected ToggleCollapse to return true")
    }
    if m.collapsedPIDs[10] != false {
        t.Error("expected PID 10 to be expanded after toggle")
    }
}

func TestToggleCollapse_Depth1NoChildren_ReturnsFalse(t *testing.T) {
    m := ProcListModel{
        collapsedPIDs: map[int32]bool{},
        nodes: []ProcListNode{
            {IsPaneHeader: true, Pane: tmux.Pane{PaneIndex: 0}},
            {Proc: proc.Process{PID: 20}, Depth: 1, HasChildren: false},
        },
        cursor: 1,
    }
    toggled := m.ToggleCollapse()
    if toggled {
        t.Error("expected ToggleCollapse to return false for node without children")
    }
}

func TestToggleCollapse_PaneHeader_ReturnsFalse(t *testing.T) {
    m := ProcListModel{
        collapsedPIDs: map[int32]bool{},
        nodes: []ProcListNode{
            {IsPaneHeader: true, Pane: tmux.Pane{PaneIndex: 0}},
        },
        cursor: 0,
    }
    if m.ToggleCollapse() {
        t.Error("expected false for pane header")
    }
}

func TestToggleCollapse_ExpandedToCollapsed(t *testing.T) {
    m := ProcListModel{
        collapsedPIDs: map[int32]bool{30: false},
        nodes: []ProcListNode{
            {IsPaneHeader: true, Pane: tmux.Pane{PaneIndex: 0}},
            {Proc: proc.Process{PID: 30}, Depth: 1, HasChildren: true, Collapsed: false},
        },
        cursor: 1,
    }
    m.ToggleCollapse()
    if m.collapsedPIDs[30] != true {
        t.Error("expected PID 30 to be collapsed after toggle")
    }
}

func TestToggleCollapse_EmptyNodes_ReturnsFalse(t *testing.T) {
    m := ProcListModel{}
    if m.ToggleCollapse() {
        t.Error("expected false for empty model")
    }
}

func TestToggleCollapse_IdleNode_ReturnsFalse(t *testing.T) {
    m := ProcListModel{
        collapsedPIDs: map[int32]bool{},
        nodes: []ProcListNode{
            {IsPaneHeader: true, Pane: tmux.Pane{PaneIndex: 0}},
            {IsIdle: true, Depth: 1},
        },
        cursor: 1,
    }
    if m.ToggleCollapse() {
        t.Error("expected false for idle node")
    }
}

// ---------- Expand ----------

func TestExpand_CollapsedNode_ExpandsAndReturnsTrue(t *testing.T) {
    m := ProcListModel{
        collapsedPIDs: map[int32]bool{10: true},
        nodes: []ProcListNode{
            {IsPaneHeader: true, Pane: tmux.Pane{PaneIndex: 0}},
            {Proc: proc.Process{PID: 10, Name: "proc"}, Depth: 1, HasChildren: true},
        },
        cursor: 1,
    }
    if !m.Expand() {
        t.Error("expected Expand to return true for collapsed node")
    }
    if m.collapsedPIDs[10] {
        t.Error("expected PID 10 to be expanded after Expand()")
    }
}

func TestExpand_AlreadyExpanded_ReturnsFalse(t *testing.T) {
    m := ProcListModel{
        collapsedPIDs: map[int32]bool{10: false},
        nodes: []ProcListNode{
            {IsPaneHeader: true, Pane: tmux.Pane{PaneIndex: 0}},
            {Proc: proc.Process{PID: 10, Name: "proc"}, Depth: 1, HasChildren: true},
        },
        cursor: 1,
    }
    if m.Expand() {
        t.Error("expected Expand to return false when node is already expanded")
    }
}

func TestExpand_PaneHeader_ReturnsFalse(t *testing.T) {
    m := ProcListModel{
        collapsedPIDs: map[int32]bool{},
        nodes:         []ProcListNode{{IsPaneHeader: true, Pane: tmux.Pane{PaneIndex: 0}}},
        cursor:        0,
    }
    if m.Expand() {
        t.Error("expected false for pane header")
    }
}

func TestExpand_NoChildren_ReturnsFalse(t *testing.T) {
    m := ProcListModel{
        collapsedPIDs: map[int32]bool{},
        nodes: []ProcListNode{
            {Proc: proc.Process{PID: 5, Name: "proc"}, Depth: 1, HasChildren: false},
        },
        cursor: 0,
    }
    if m.Expand() {
        t.Error("expected false for node without children")
    }
}

// ---------- Collapse ----------

func TestCollapse_ExpandedNode_CollapsesAndReturnsTrue(t *testing.T) {
    m := ProcListModel{
        collapsedPIDs: map[int32]bool{20: false},
        nodes: []ProcListNode{
            {IsPaneHeader: true, Pane: tmux.Pane{PaneIndex: 0}},
            {Proc: proc.Process{PID: 20, Name: "proc"}, Depth: 1, HasChildren: true},
        },
        cursor: 1,
    }
    if !m.Collapse() {
        t.Error("expected Collapse to return true for expanded node")
    }
    if !m.collapsedPIDs[20] {
        t.Error("expected PID 20 to be collapsed after Collapse()")
    }
}

func TestCollapse_AlreadyCollapsed_ReturnsFalse(t *testing.T) {
    m := ProcListModel{
        collapsedPIDs: map[int32]bool{20: true},
        nodes: []ProcListNode{
            {IsPaneHeader: true, Pane: tmux.Pane{PaneIndex: 0}},
            {Proc: proc.Process{PID: 20, Name: "proc"}, Depth: 1, HasChildren: true},
        },
        cursor: 1,
    }
    if m.Collapse() {
        t.Error("expected Collapse to return false when node is already collapsed")
    }
}

func TestCollapse_PaneHeader_ReturnsFalse(t *testing.T) {
    m := ProcListModel{
        collapsedPIDs: map[int32]bool{},
        nodes:         []ProcListNode{{IsPaneHeader: true, Pane: tmux.Pane{PaneIndex: 0}}},
        cursor:        0,
    }
    if m.Collapse() {
        t.Error("expected false for pane header")
    }
}

func TestCollapse_Depth2Node_CollapsesParentAndMovesCursor(t *testing.T) {
    m := ProcListModel{
        collapsedPIDs: map[int32]bool{20: false},
        nodes: []ProcListNode{
            {IsPaneHeader: true, Pane: tmux.Pane{PaneIndex: 0}},
            {Proc: proc.Process{PID: 20, Name: "parent"}, Depth: 1, HasChildren: true},
            {Proc: proc.Process{PID: 21, Name: "child"}, Depth: 2},
        },
        cursor: 2, // depth-2 child
    }
    if !m.Collapse() {
        t.Error("expected Collapse to return true for depth-2 node")
    }
    if !m.collapsedPIDs[20] {
        t.Error("expected parent PID 20 to be collapsed")
    }
    if m.cursor != 1 {
        t.Errorf("expected cursor to move to parent (1), got %d", m.cursor)
    }
}

func TestCollapse_Depth2Node_ParentAlreadyCollapsed_ReturnsFalse(t *testing.T) {
    m := ProcListModel{
        collapsedPIDs: map[int32]bool{20: true},
        nodes: []ProcListNode{
            {IsPaneHeader: true, Pane: tmux.Pane{PaneIndex: 0}},
            {Proc: proc.Process{PID: 20, Name: "parent"}, Depth: 1, HasChildren: true},
            {Proc: proc.Process{PID: 21, Name: "child"}, Depth: 2},
        },
        cursor: 2,
    }
    if m.Collapse() {
        t.Error("expected false when parent is already collapsed")
    }
}

// ---------- ExpandAll ----------

func TestExpandAll_CollapsedNodes_ExpandsAndReturnsTrue(t *testing.T) {
    m := ProcListModel{
        collapsedPIDs: map[int32]bool{10: true, 20: true},
        nodes: []ProcListNode{
            {IsPaneHeader: true, Pane: tmux.Pane{PaneIndex: 0}},
            {Proc: proc.Process{PID: 10, Name: "procA"}, Depth: 1, HasChildren: true},
            {IsPaneHeader: true, Pane: tmux.Pane{PaneIndex: 1}},
            {Proc: proc.Process{PID: 20, Name: "procB"}, Depth: 1, HasChildren: true},
        },
    }
    if !m.ExpandAll() {
        t.Error("expected ExpandAll to return true when nodes are collapsed")
    }
    if m.collapsedPIDs[10] || m.collapsedPIDs[20] {
        t.Error("expected all nodes to be expanded after ExpandAll()")
    }
}

func TestExpandAll_AllAlreadyExpanded_ReturnsFalse(t *testing.T) {
    m := ProcListModel{
        collapsedPIDs: map[int32]bool{10: false, 20: false},
        nodes: []ProcListNode{
            {IsPaneHeader: true, Pane: tmux.Pane{PaneIndex: 0}},
            {Proc: proc.Process{PID: 10, Name: "procA"}, Depth: 1, HasChildren: true},
            {IsPaneHeader: true, Pane: tmux.Pane{PaneIndex: 1}},
            {Proc: proc.Process{PID: 20, Name: "procB"}, Depth: 1, HasChildren: true},
        },
    }
    if m.ExpandAll() {
        t.Error("expected ExpandAll to return false when all are already expanded")
    }
}

func TestExpandAll_NoCollapsibleNodes_ReturnsFalse(t *testing.T) {
    m := ProcListModel{
        collapsedPIDs: map[int32]bool{},
        nodes:         []ProcListNode{{IsPaneHeader: true, Pane: tmux.Pane{PaneIndex: 0}}},
    }
    if m.ExpandAll() {
        t.Error("expected ExpandAll to return false with no collapsible nodes")
    }
}

// ---------- CollapseAll ----------

func TestCollapseAll_ExpandedNodes_CollapsesAndReturnsTrue(t *testing.T) {
    m := ProcListModel{
        collapsedPIDs: map[int32]bool{10: false, 20: false},
        nodes: []ProcListNode{
            {IsPaneHeader: true, Pane: tmux.Pane{PaneIndex: 0}},
            {Proc: proc.Process{PID: 10, Name: "procA"}, Depth: 1, HasChildren: true},
            {IsPaneHeader: true, Pane: tmux.Pane{PaneIndex: 1}},
            {Proc: proc.Process{PID: 20, Name: "procB"}, Depth: 1, HasChildren: true},
        },
    }
    if !m.CollapseAll() {
        t.Error("expected CollapseAll to return true when nodes are expanded")
    }
    if !m.collapsedPIDs[10] || !m.collapsedPIDs[20] {
        t.Error("expected all nodes to be collapsed after CollapseAll()")
    }
}

func TestCollapseAll_AllAlreadyCollapsed_ReturnsFalse(t *testing.T) {
    m := ProcListModel{
        collapsedPIDs: map[int32]bool{10: true, 20: true},
        nodes: []ProcListNode{
            {IsPaneHeader: true, Pane: tmux.Pane{PaneIndex: 0}},
            {Proc: proc.Process{PID: 10, Name: "procA"}, Depth: 1, HasChildren: true},
            {IsPaneHeader: true, Pane: tmux.Pane{PaneIndex: 1}},
            {Proc: proc.Process{PID: 20, Name: "procB"}, Depth: 1, HasChildren: true},
        },
    }
    if m.CollapseAll() {
        t.Error("expected CollapseAll to return false when all are already collapsed")
    }
}

// ---------- clampOffset cursor bounds ----------

// Regression: CollapseAll shrinks p.nodes; clampOffset must clamp p.cursor to
// prevent out-of-bounds panics on subsequent MoveUp/MoveDown calls.
func TestClampOffset_CursorBeyondNodes_ClampsToBounds(t *testing.T) {
    m := ProcListModel{
        nodes: []ProcListNode{
            {IsPaneHeader: true, Pane: tmux.Pane{PaneIndex: 0}},
            {Proc: proc.Process{PID: 10, Name: "procA"}, Depth: 1, HasChildren: true},
        },
        cursor: 15, // out of bounds after a CollapseAll rebuild
        offset: 15,
    }
    m.clampOffset(10)
    if m.cursor >= len(m.nodes) {
        t.Errorf("cursor %d still out of bounds (len=%d)", m.cursor, len(m.nodes))
    }
    if m.offset < 0 || m.offset >= len(m.nodes) {
        t.Errorf("offset %d out of bounds (len=%d)", m.offset, len(m.nodes))
    }
    // MoveUp must not panic
    m.MoveUp()
    // MoveDown must not panic
    m.MoveDown()
}

// ---------- renderProc collapse rendering ----------

func TestRenderProc_CollapsedWithChildren_ShowsRightTriangleAndAggStats(t *testing.T) {
    m := ProcListModel{}
    node := ProcListNode{
        Proc:        proc.Process{PID: 1, Name: "node", CPU: 1.0, MemRSS: 100 * 1024 * 1024},
        Depth:       1,
        HasChildren: true,
        Collapsed:   true,
        AggCPU:      2.5,
        AggMemRSS:   250 * 1024 * 1024,
        TreePrefix:  "  └─ ",
        StatPrefix:  "     ",
    }
    line := m.renderProc(node, false, 80)
    plain := stripANSI(line)
    if !strings.Contains(plain, "▶") {
        t.Errorf("expected ▶ prefix for collapsed node, got: %s", plain)
    }
    if !strings.Contains(plain, "(2.5%)") {
        t.Errorf("expected aggregated cpu (2.5%%) in stats, got: %s", plain)
    }
    if !strings.Contains(plain, "(250.0MB)") {
        t.Errorf("expected aggregated mem (250.0MB) in stats, got: %s", plain)
    }
    if !strings.Contains(plain, "└─") {
        t.Errorf("expected tree connector └─ in output, got: %s", plain)
    }
}

func TestRenderProc_ExpandedWithChildren_ShowsDownTriangle_NoAggStats(t *testing.T) {
    m := ProcListModel{}
    node := ProcListNode{
        Proc:        proc.Process{PID: 2, Name: "node", CPU: 1.0, MemRSS: 100 * 1024 * 1024},
        Depth:       1,
        HasChildren: true,
        Collapsed:   false,
        AggCPU:      2.5,
        AggMemRSS:   250 * 1024 * 1024,
        TreePrefix:  "  └─ ",
        StatPrefix:  "     ",
    }
    line := m.renderProc(node, false, 80)
    plain := stripANSI(line)
    if !strings.Contains(plain, "▼") {
        t.Errorf("expected ▼ prefix for expanded node with children, got: %s", plain)
    }
    if strings.Contains(plain, "(2.5%)") {
        t.Errorf("expanded node should not show agg stats, got: %s", plain)
    }
    if !strings.Contains(plain, "└─") {
        t.Errorf("expected tree connector └─ in output, got: %s", plain)
    }
}

func TestRenderProc_NoChildren_NoTriangle(t *testing.T) {
    m := ProcListModel{}
    node := ProcListNode{
        Proc:        proc.Process{PID: 3, Name: "nvim"},
        Depth:       1,
        HasChildren: false,
        TreePrefix:  "  └─ ",
        StatPrefix:  "     ",
    }
    line := m.renderProc(node, false, 80)
    plain := stripANSI(line)
    if strings.Contains(plain, "▶") || strings.Contains(plain, "▼") {
        t.Errorf("no triangle expected for node without children, got: %s", plain)
    }
    if !strings.Contains(plain, "└─") {
        t.Errorf("expected tree connector └─ in output, got: %s", plain)
    }
}

// ---------- assignTreePrefixes ----------

func TestAssignTreePrefixes_Depth0_Unchanged(t *testing.T) {
    nodes := []ProcListNode{
        {IsPaneHeader: true, Depth: 0},
    }
    assignTreePrefixes(nodes)
    if nodes[0].TreePrefix != "" || nodes[0].StatPrefix != "" {
        t.Error("pane header (depth 0) should have empty prefixes")
    }
}

func TestAssignTreePrefixes_SingleDepth1_GetsLastConnector(t *testing.T) {
    nodes := []ProcListNode{
        {IsPaneHeader: true, Depth: 0},
        {Proc: proc.Process{PID: 1}, Depth: 1},
    }
    assignTreePrefixes(nodes)
    if nodes[1].TreePrefix != "  └─ " {
        t.Errorf("single depth-1 should get └─, got %q", nodes[1].TreePrefix)
    }
    if nodes[1].StatPrefix != "     " {
        t.Errorf("single depth-1 StatPrefix should be 5 spaces, got %q", nodes[1].StatPrefix)
    }
}

func TestAssignTreePrefixes_TwoDepth1_CorrectConnectors(t *testing.T) {
    nodes := []ProcListNode{
        {IsPaneHeader: true, Depth: 0},
        {Proc: proc.Process{PID: 1}, Depth: 1},
        {Proc: proc.Process{PID: 2}, Depth: 1},
    }
    assignTreePrefixes(nodes)
    if nodes[1].TreePrefix != "  ├─ " {
        t.Errorf("first of two depth-1 should get ├─, got %q", nodes[1].TreePrefix)
    }
    if nodes[1].StatPrefix != "  │  " {
        t.Errorf("first of two depth-1 StatPrefix should be '  │  ', got %q", nodes[1].StatPrefix)
    }
    if nodes[2].TreePrefix != "  └─ " {
        t.Errorf("last depth-1 should get └─, got %q", nodes[2].TreePrefix)
    }
    if nodes[2].StatPrefix != "     " {
        t.Errorf("last depth-1 StatPrefix should be 5 spaces, got %q", nodes[2].StatPrefix)
    }
}

func TestAssignTreePrefixes_Depth2UnderNonLastParent(t *testing.T) {
    // pane0, procA (non-last), procA-child (last under procA), procB (last)
    nodes := []ProcListNode{
        {IsPaneHeader: true, Depth: 0},
        {Proc: proc.Process{PID: 1}, Depth: 1}, // non-last
        {Proc: proc.Process{PID: 2}, Depth: 2}, // last under PID 1
        {Proc: proc.Process{PID: 3}, Depth: 1}, // last
    }
    assignTreePrefixes(nodes)
    // procA-child: parent (depth-1, PID 1) is non-last → ancestor cont = "│  "
    // procA-child is the only child → isLast → "└─ "
    if nodes[2].TreePrefix != "  │  └─ " {
        t.Errorf("depth-2 under non-last parent should get '  │  └─ ', got %q", nodes[2].TreePrefix)
    }
    if nodes[2].StatPrefix != "  │     " {
        t.Errorf("depth-2 StatPrefix under non-last parent should be '  │     ', got %q", nodes[2].StatPrefix)
    }
}

func TestAssignTreePrefixes_Depth2UnderLastParent(t *testing.T) {
    // pane0, procA (last), procA-child1 (non-last), procA-child2 (last)
    nodes := []ProcListNode{
        {IsPaneHeader: true, Depth: 0},
        {Proc: proc.Process{PID: 1}, Depth: 1}, // last depth-1
        {Proc: proc.Process{PID: 2}, Depth: 2}, // non-last
        {Proc: proc.Process{PID: 3}, Depth: 2}, // last
    }
    assignTreePrefixes(nodes)
    // parent (PID 1) is last → ancestor cont = "   " (3 spaces)
    if nodes[2].TreePrefix != "     ├─ " {
        t.Errorf("non-last depth-2 under last parent, got %q", nodes[2].TreePrefix)
    }
    if nodes[2].StatPrefix != "     │  " {
        t.Errorf("non-last depth-2 StatPrefix under last parent, got %q", nodes[2].StatPrefix)
    }
    if nodes[3].TreePrefix != "     └─ " {
        t.Errorf("last depth-2 under last parent, got %q", nodes[3].TreePrefix)
    }
    if nodes[3].StatPrefix != "        " {
        t.Errorf("last depth-2 StatPrefix (8 spaces), got %q", nodes[3].StatPrefix)
    }
}

func TestAssignTreePrefixes_CrossPaneBoundary_ResetsSiblingCount(t *testing.T) {
    // depth-1 in pane 1 should be treated as last (only child in its pane)
    nodes := []ProcListNode{
        {IsPaneHeader: true, Depth: 0},
        {Proc: proc.Process{PID: 1}, Depth: 1},
        {IsPaneHeader: true, Depth: 0},
        {Proc: proc.Process{PID: 2}, Depth: 1},
    }
    assignTreePrefixes(nodes)
    // PID 1 is last in pane 0 (next node at same depth is in another pane, separated by depth-0)
    if nodes[1].TreePrefix != "  └─ " {
        t.Errorf("depth-1 in pane 0 should get └─, got %q", nodes[1].TreePrefix)
    }
    // PID 2 is last in pane 1
    if nodes[3].TreePrefix != "  └─ " {
        t.Errorf("depth-1 in pane 1 should get └─, got %q", nodes[3].TreePrefix)
    }
}

// ---------- renderPaneHeader right-align tests ----------

func paneNode(paneIndex int, cwd string) ProcListNode {
    return ProcListNode{
        IsPaneHeader: true,
        Pane:         tmux.Pane{PaneIndex: paneIndex, CWD: cwd},
    }
}

func TestRenderPaneHeader_RightAlign_PathAppearsAtRight(t *testing.T) {
    p := ProcListModel{
        cfg: config.Config{PanePathRightAlign: true},
    }
    node := paneNode(0, "/home/user/project")
    innerW := 40
    line := p.renderPaneHeader(node, false, innerW)
    plain := stripANSI(line)
    runes := []rune(plain)
    if len(runes) != innerW {
        t.Errorf("expected line width %d, got %d: %q", innerW, len(runes), plain)
    }
    if !strings.HasSuffix(strings.TrimRight(plain, " "), "/home/user/project") {
        t.Errorf("expected path at right edge, got: %q", plain)
    }
}

func TestRenderPaneHeader_RightAlign_FillCharsPresent(t *testing.T) {
    p := ProcListModel{
        cfg: config.Config{PanePathRightAlign: true},
    }
    node := paneNode(1, "/tmp")
    innerW := 40
    line := p.renderPaneHeader(node, false, innerW)
    plain := stripANSI(line)
    if !strings.Contains(plain, "─") {
        t.Errorf("expected fill chars (─) in right-aligned line, got: %q", plain)
    }
}

func TestRenderPaneHeader_LeftAlign_NoFillChars(t *testing.T) {
    p := ProcListModel{
        cfg: config.Config{PanePathRightAlign: false},
    }
    node := paneNode(0, "/tmp")
    innerW := 40
    line := p.renderPaneHeader(node, false, innerW)
    plain := stripANSI(line)
    if strings.Contains(plain, "─") {
        t.Errorf("expected no fill chars in left-aligned line, got: %q", plain)
    }
}

func TestRenderPaneHeader_RightAlign_NoCWD_NoFill(t *testing.T) {
    p := ProcListModel{
        cfg: config.Config{PanePathRightAlign: true},
    }
    node := paneNode(0, "")
    innerW := 40
    line := p.renderPaneHeader(node, false, innerW)
    plain := stripANSI(line)
    if strings.Contains(plain, "─") {
        t.Errorf("expected no fill when CWD is empty, got: %q", plain)
    }
}

func TestRenderPaneHeader_RightAlign_Selected_ContainsLabelAndPath(t *testing.T) {
    p := ProcListModel{
        cfg: config.Config{PanePathRightAlign: true},
    }
    node := paneNode(0, "/home/user/project")
    innerW := 40
    line := p.renderPaneHeader(node, true, innerW)
    plain := stripANSI(line)
    if !strings.Contains(plain, "pane 0") {
        t.Errorf("expected label in selected right-align line, got: %q", plain)
    }
    if !strings.Contains(plain, "/home/user/project") {
        t.Errorf("expected path in selected right-align line, got: %q", plain)
    }
    // selected rows use space padding, not separator chars
    if strings.Contains(plain, "─") {
        t.Errorf("selected right-align line should not contain separator chars, got: %q", plain)
    }
}

// ---------- windowAlertFromMap ----------

func TestWindowAlertFromMap_ReturnsNilForEmptyMap(t *testing.T) {
    if windowAlertFromMap(nil, "s", 0) != nil {
        t.Error("expected nil for nil map")
    }
    if windowAlertFromMap(map[string]db.Alert{}, "s", 0) != nil {
        t.Error("expected nil for empty map")
    }
}

func TestWindowAlertFromMap_ReturnsWindowLevelAlert(t *testing.T) {
    a := db.Alert{Target: "s:0", Level: db.LevelInfo}
    m := map[string]db.Alert{"s:0": a}
    got := windowAlertFromMap(m, "s", 0)
    if got == nil || got.Target != "s:0" {
        t.Errorf("expected alert for s:0, got %v", got)
    }
}

func TestWindowAlertFromMap_ReturnsPaneLevelAlert(t *testing.T) {
    a := db.Alert{Target: "s:0.1", Level: db.LevelInfo}
    m := map[string]db.Alert{"s:0.1": a}
    got := windowAlertFromMap(m, "s", 0)
    if got == nil || got.Target != "s:0.1" {
        t.Errorf("expected alert for pane s:0.1, got %v", got)
    }
}

func TestWindowAlertFromMap_ReturnsHighestSeverity(t *testing.T) {
    low := db.Alert{Target: "s:1.0", Level: db.LevelInfo}
    high := db.Alert{Target: "s:1.1", Level: db.LevelError}
    m := map[string]db.Alert{"s:1.0": low, "s:1.1": high}
    got := windowAlertFromMap(m, "s", 1)
    if got == nil || got.Level != db.LevelError {
        t.Errorf("expected LevelError, got %v", got)
    }
}

func TestWindowAlertFromMap_IgnoresOtherWindows(t *testing.T) {
    a := db.Alert{Target: "s:2", Level: db.LevelInfo}
    m := map[string]db.Alert{"s:2": a}
    if windowAlertFromMap(m, "s", 0) != nil {
        t.Error("expected nil — window 2 alert should not match window 0")
    }
}

// ---------- SetSessionData ----------

func buildSessionPanes() []tmux.Pane {
    return []tmux.Pane{
        // window 0, pane 0 — shell PID 10
        {Session: "s", WindowIndex: 0, PaneIndex: 0, WindowName: "editor", CWD: "/proj", PanePID: 10},
        // window 0, pane 1 — shell PID 11, same CWD
        {Session: "s", WindowIndex: 0, PaneIndex: 1, WindowName: "editor", CWD: "/proj", PanePID: 11},
        // window 1, pane 0 — shell PID 20, different CWD
        {Session: "s", WindowIndex: 1, PaneIndex: 0, WindowName: "server", CWD: "/other", PanePID: 20},
    }
}

func TestSetSessionData_EmitsWindowHeaders(t *testing.T) {
    var m ProcListModel
    m.SetSessionData(buildSessionPanes(), "s",
        nil, map[int32]string{}, map[string]git.Info{}, nil, config.Config{},
    )
    if !m.inSessionMode {
        t.Error("expected inSessionMode=true after SetSessionData")
    }
    if len(m.nodes) == 0 {
        t.Fatal("expected nodes to be populated")
    }
    if !m.nodes[0].IsWindowHeader {
        t.Error("expected first node to be a window header")
    }
    if m.nodes[0].Pane.WindowIndex != 0 {
        t.Errorf("expected window index 0, got %d", m.nodes[0].Pane.WindowIndex)
    }
    if m.nodes[0].Pane.WindowName != "editor" {
        t.Errorf("expected window name 'editor', got %q", m.nodes[0].Pane.WindowName)
    }
}

func TestSetSessionData_WindowsOrderedByIndex(t *testing.T) {
    var m ProcListModel
    m.SetSessionData(buildSessionPanes(), "s",
        nil, map[int32]string{}, map[string]git.Info{}, nil, config.Config{},
    )
    var winHeaders []int
    for _, n := range m.nodes {
        if n.IsWindowHeader {
            winHeaders = append(winHeaders, n.Pane.WindowIndex)
        }
    }
    if len(winHeaders) != 2 {
        t.Fatalf("expected 2 window headers, got %d", len(winHeaders))
    }
    if winHeaders[0] != 0 || winHeaders[1] != 1 {
        t.Errorf("expected windows [0, 1], got %v", winHeaders)
    }
}

func TestSetSessionData_SuppressPaneCWDWhenMatchesWindowCWD(t *testing.T) {
    var m ProcListModel
    m.SetSessionData(buildSessionPanes(), "s",
        nil, map[int32]string{}, map[string]git.Info{}, nil, config.Config{},
    )
    // pane 0 and pane 1 of window 0 both have CWD "/proj" == window CWD — should be suppressed
    for _, n := range m.nodes {
        if n.IsPaneHeader && n.Pane.WindowIndex == 0 {
            if n.Pane.CWD != "" {
                t.Errorf("expected pane CWD suppressed for window 0 pane %d, got %q",
                    n.Pane.PaneIndex, n.Pane.CWD)
            }
        }
    }
}

func TestSetSessionData_ShowsPaneCWDWhenDivergent(t *testing.T) {
    panes := []tmux.Pane{
        {Session: "s", WindowIndex: 0, PaneIndex: 0, CWD: "/proj", PanePID: 10},
        {Session: "s", WindowIndex: 0, PaneIndex: 1, CWD: "/other", PanePID: 11}, // divergent
    }
    var m ProcListModel
    m.SetSessionData(panes, "s",
        nil, map[int32]string{}, map[string]git.Info{}, nil, config.Config{},
    )
    found := false
    for _, n := range m.nodes {
        if n.IsPaneHeader && n.Pane.PaneIndex == 1 {
            found = true
            if n.Pane.CWD != "/other" {
                t.Errorf("expected divergent pane CWD '/other', got %q", n.Pane.CWD)
            }
        }
    }
    if !found {
        t.Error("pane 1 header not found")
    }
}

func TestSetSessionData_SetsWindowAlert(t *testing.T) {
    a := db.Alert{Target: "s:0", Level: db.LevelInfo}
    alertMap := map[string]db.Alert{"s:0": a}
    var m ProcListModel
    m.SetSessionData(buildSessionPanes(), "s",
        nil, map[int32]string{}, map[string]git.Info{}, alertMap, config.Config{},
    )
    if m.nodes[0].Alert == nil {
        t.Error("expected window header to carry alert")
    }
}

func TestSetSessionData_ResetsCursorOnSessionChange(t *testing.T) {
    var m ProcListModel
    m.SetSessionData(buildSessionPanes(), "s",
        nil, map[int32]string{}, map[string]git.Info{}, nil, config.Config{},
    )
    m.cursor = 3
    m.offset = 2
    // change to different session — should reset cursor and offset
    m.SetSessionData(buildSessionPanes(), "other",
        nil, map[int32]string{}, map[string]git.Info{}, nil, config.Config{},
    )
    if m.cursor != 0 || m.offset != 0 {
        t.Errorf("expected cursor=0 offset=0 after session change, got cursor=%d offset=%d",
            m.cursor, m.offset)
    }
}

func TestSetSessionData_SetWindowDataClearsSessionMode(t *testing.T) {
    pane := buildTestPane(100)
    var m ProcListModel
    m.SetSessionData(buildSessionPanes(), "s",
        nil, map[int32]string{}, map[string]git.Info{}, nil, config.Config{},
    )
    if !m.inSessionMode {
        t.Fatal("expected inSessionMode=true")
    }
    m.SetWindowData([]tmux.Pane{pane}, "test", 0,
        nil, map[int32]string{}, map[string]git.Info{}, nil, config.Config{},
    )
    if m.inSessionMode {
        t.Error("expected inSessionMode=false after SetWindowData")
    }
}

// ---------- renderWindowHeader smoke test ----------

func TestRender_SessionMode_ContainsWindowHeader(t *testing.T) {
    panes := []tmux.Pane{
        {Session: "s", WindowIndex: 0, PaneIndex: 0, WindowName: "edit", CWD: "/proj", PanePID: 500},
        {Session: "s", WindowIndex: 1, PaneIndex: 0, WindowName: "run", CWD: "/proj", PanePID: 501},
    }
    var m ProcListModel
    m.SetSessionData(panes, "s",
        nil, map[int32]string{}, map[string]git.Info{}, nil, config.Config{},
    )
    out := m.Render(80, 20, false, "procs")
    plain := stripANSI(out)
    if !strings.Contains(plain, "Win 1") {
        t.Errorf("expected 'Win 1' in output, got:\n%s", plain)
    }
}

func TestRender_SessionMode_PaneHeaderIndented(t *testing.T) {
    panes := []tmux.Pane{
        {Session: "s", WindowIndex: 0, PaneIndex: 0, CWD: "/proj", PanePID: 600},
    }
    var m ProcListModel
    m.SetSessionData(panes, "s",
        nil, map[int32]string{}, map[string]git.Info{}, nil, config.Config{},
    )
    out := m.Render(80, 20, false, "procs")
    plain := stripANSI(out)
    // pane 0 should appear indented with 4 leading spaces
    if !strings.Contains(plain, "    pane 0") {
        t.Errorf("expected '    pane 0' (4-space indent) in output, got:\n%s", plain)
    }
}
