package tui

import (
    "testing"

    "github.com/rtalex/demux/internal/tmux"
)

func makeTestProcListWithNodes(nodes []ProcListNode) ProcListModel {
    m := ProcListModel{}
    m.nodes = nodes
    return m
}

func TestSelectedPane_OnPaneHeader(t *testing.T) {
    pane := tmux.Pane{Session: "main", WindowIndex: 0, PaneIndex: 1}
    nodes := []ProcListNode{
        {IsPaneHeader: true, Pane: pane},
    }
    m := makeTestProcListWithNodes(nodes)
    m.cursor = 0
    got := m.SelectedPane()
    if got == nil {
        t.Fatal("expected non-nil pane")
    }
    if got.PaneIndex != 1 {
        t.Errorf("PaneIndex: got %d, want 1", got.PaneIndex)
    }
}

func TestSelectedPane_OnProcessNode(t *testing.T) {
    pane := tmux.Pane{Session: "main", WindowIndex: 0, PaneIndex: 2}
    nodes := []ProcListNode{
        {IsPaneHeader: true, Pane: pane},
        {IsPaneHeader: false, Depth: 1, Pane: pane},
        {IsPaneHeader: false, Depth: 2, Pane: pane},
    }
    m := makeTestProcListWithNodes(nodes)

    // cursor on depth-1 process
    m.cursor = 1
    got := m.SelectedPane()
    if got == nil {
        t.Fatal("expected non-nil pane for depth-1 node")
    }
    if got.PaneIndex != 2 {
        t.Errorf("PaneIndex: got %d, want 2", got.PaneIndex)
    }

    // cursor on depth-2 subprocess
    m.cursor = 2
    got = m.SelectedPane()
    if got == nil {
        t.Fatal("expected non-nil pane for depth-2 node")
    }
}

func TestSelectedPane_EmptyList(t *testing.T) {
    m := makeTestProcListWithNodes(nil)
    if m.SelectedPane() != nil {
        t.Error("expected nil for empty node list")
    }
}
