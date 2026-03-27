package tui

import (
    "strings"
    "testing"

    "github.com/rtalex/demux/internal/config"
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
        map[string]git.Info{}, config.Config{},
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
    // pre-expand node (PID 301) before calling SetWindowData
    m.collapsedPIDs = map[int32]bool{301: false}
    m.SetWindowData(
        []tmux.Pane{pane}, "test", 0,
        procs, map[int32]string{},
        map[string]git.Info{}, config.Config{},
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
        map[string]git.Info{}, config.Config{},
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
	}
	line := m.renderProc(node, false)
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
	}
	line := m.renderProc(node, false)
	plain := stripANSI(line)
	if !strings.Contains(plain, "▼") {
		t.Errorf("expected ▼ prefix for expanded node with children, got: %s", plain)
	}
	if strings.Contains(plain, "(2.5%)") {
		t.Errorf("expanded node should not show agg stats, got: %s", plain)
	}
}

func TestRenderProc_NoChildren_NoTriangle(t *testing.T) {
	m := ProcListModel{}
	node := ProcListNode{
		Proc:        proc.Process{PID: 3, Name: "nvim"},
		Depth:       1,
		HasChildren: false,
	}
	line := m.renderProc(node, false)
	plain := stripANSI(line)
	if strings.Contains(plain, "▶") || strings.Contains(plain, "▼") {
		t.Errorf("no triangle expected for node without children, got: %s", plain)
	}
}
