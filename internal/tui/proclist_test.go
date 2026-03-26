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

func TestMoveUp_Depth0_MovesToPrevHeader(t *testing.T) {
    m := modelAt(buildNodes(), 3) // cursor on second pane header
    m.MoveUp()
    if m.cursor != 0 {
        t.Errorf("expected cursor 0 (first pane header), got %d", m.cursor)
    }
}

func TestMoveUp_Depth0_AtFirstHeader_NoMove(t *testing.T) {
    m := modelAt(buildNodes(), 0) // already at first pane header
    m.MoveUp()
    if m.cursor != 0 {
        t.Errorf("expected cursor to stay at 0, got %d", m.cursor)
    }
}

func TestMoveUp_Depth1_DoesNotCrossPaneHeader(t *testing.T) {
    // cursor at procB (index 4) — MoveUp should not cross pane header at index 3
    m := modelAt(buildNodes(), 4)
    m.MoveUp()
    // No sibling at depth 1 before the pane header, so cursor stays.
    if m.cursor != 4 {
        t.Errorf("expected cursor to stay at 4, got %d", m.cursor)
    }
}

func TestMoveUp_Depth2_DoesNotCrossParentProcess(t *testing.T) {
    // cursor at procB-child (index 5) — MoveUp should not cross procB (depth 1)
    m := modelAt(buildNodes(), 5)
    m.MoveUp()
    // No sibling at depth 2 before procB within this scope, so cursor stays.
    if m.cursor != 5 {
        t.Errorf("expected cursor to stay at 5, got %d", m.cursor)
    }
}

func TestMoveUp_Depth2_MovesToPrevSiblingAtSameDepth(t *testing.T) {
    // Add a second depth-2 child so there is a peer to move to.
    nodes := buildNodes()
    nodes = append(nodes[:3], append([]ProcListNode{
        {Proc: proc.Process{PID: 10, Name: "procA-child2"}, Depth: 2},
    }, nodes[3:]...)...)
    // nodes: [0]=pane0, [1]=procA, [2]=procA-child, [3]=procA-child2, [4]=pane1, ...
    m := modelAt(nodes, 3) // cursor on procA-child2
    m.MoveUp()
    if m.cursor != 2 {
        t.Errorf("expected cursor 2 (procA-child), got %d", m.cursor)
    }
}

// ---------- MoveDown tests ----------

func TestMoveDown_Depth0_MovesToNextHeader(t *testing.T) {
    m := modelAt(buildNodes(), 0)
    m.MoveDown()
    if m.cursor != 3 {
        t.Errorf("expected cursor 3 (second pane header), got %d", m.cursor)
    }
}

func TestMoveDown_Depth0_AtLastHeader_NoMove(t *testing.T) {
    m := modelAt(buildNodes(), 3)
    m.MoveDown()
    if m.cursor != 3 {
        t.Errorf("expected cursor to stay at 3, got %d", m.cursor)
    }
}

func TestMoveDown_Depth1_DoesNotCrossPaneHeader(t *testing.T) {
    // cursor at procA (index 1) — MoveDown should not cross pane header at index 3
    m := modelAt(buildNodes(), 1)
    m.MoveDown()
    // No depth-1 peer after procA before the next header, so cursor stays.
    if m.cursor != 1 {
        t.Errorf("expected cursor to stay at 1, got %d", m.cursor)
    }
}

func TestMoveDown_Depth2_DoesNotCrossParentProcess(t *testing.T) {
    // cursor at procA-child (index 2) — MoveDown should not cross pane header (index 3)
    m := modelAt(buildNodes(), 2)
    m.MoveDown()
    if m.cursor != 2 {
        t.Errorf("expected cursor to stay at 2, got %d", m.cursor)
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
