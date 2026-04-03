package tui

import (
	"testing"

	"github.com/rtalexk/demux/internal/tmux"
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
	if got.PaneIndex != 2 {
		t.Errorf("PaneIndex: got %d, want 2", got.PaneIndex)
	}
}

func TestSelectedPane_OnWindowHeader(t *testing.T) {
	win0Pane := tmux.Pane{Session: "s", WindowIndex: 0, PaneIndex: 0}
	win1Pane := tmux.Pane{Session: "s", WindowIndex: 1, PaneIndex: 0}
	nodes := []ProcListNode{
		{IsWindowHeader: true, Pane: win0Pane},
		{IsPaneHeader: true, Pane: win0Pane},
		{IsWindowHeader: true, Pane: win1Pane},
		{IsPaneHeader: true, Pane: win1Pane},
	}
	m := makeTestProcListWithNodes(nodes)

	// cursor on Win 1 header — must return Win 1, not Win 0
	m.cursor = 2
	got := m.SelectedPane()
	if got == nil {
		t.Fatal("expected non-nil pane for window header node")
	}
	if got.WindowIndex != 1 {
		t.Errorf("WindowIndex: got %d, want 1", got.WindowIndex)
	}
}

func TestSelectedPane_EmptyList(t *testing.T) {
	m := makeTestProcListWithNodes(nil)
	if m.SelectedPane() != nil {
		t.Error("expected nil for empty node list")
	}
}

func TestSelectedPane_StaleCursor(t *testing.T) {
	pane := tmux.Pane{Session: "main", WindowIndex: 0, PaneIndex: 1}
	nodes := []ProcListNode{
		{IsPaneHeader: true, Pane: pane},
	}
	m := makeTestProcListWithNodes(nodes)
	m.cursor = 5 // stale: beyond slice bounds
	got := m.SelectedPane()
	if got == nil {
		t.Fatal("expected non-nil pane when cursor is stale but pane header exists")
	}
	if got.PaneIndex != 1 {
		t.Errorf("PaneIndex: got %d, want 1", got.PaneIndex)
	}
}
