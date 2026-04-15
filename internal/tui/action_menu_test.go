package tui

import (
	"testing"

	"github.com/rtalexk/demux/internal/proc"
	"github.com/rtalexk/demux/internal/tmux"
)

func TestBuildActionItems_AlwaysPresent(t *testing.T) {
	node := ProcListNode{
		Proc: proc.Process{PID: 42, Name: "node", Cmdline: "node server.js"},
		Pane: tmux.Pane{CWD: "/home/user/app", PaneID: "%1"},
	}
	items := buildActionItems(node)
	kinds := make(map[ActionKind]bool)
	for _, it := range items {
		kinds[it.Kind] = true
	}
	for _, required := range []ActionKind{ActionKill, ActionRestart, ActionViewLogs} {
		if !kinds[required] {
			t.Errorf("expected action %v in items", required)
		}
	}
}

func TestBuildActionItems_ContextualPort(t *testing.T) {
	node := ProcListNode{
		Proc: proc.Process{PID: 1, Name: "node", Cmdline: "node server.js"},
		Pane: tmux.Pane{PaneID: "%1"},
		Port: 3000,
	}
	items := buildActionItems(node)
	found := false
	for _, it := range items {
		if it.Kind == ActionOpenBrowser {
			found = true
		}
	}
	if !found {
		t.Error("expected ActionOpenBrowser when Port > 0")
	}
}

func TestBuildActionItems_NoPortNoOpenBrowser(t *testing.T) {
	node := ProcListNode{
		Proc: proc.Process{PID: 1, Name: "go", Cmdline: "go build"},
		Pane: tmux.Pane{PaneID: "%1"},
		Port: 0,
	}
	items := buildActionItems(node)
	for _, it := range items {
		if it.Kind == ActionOpenBrowser {
			t.Error("ActionOpenBrowser should not appear when Port == 0")
		}
	}
}


func TestActionMenuModel_Navigation(t *testing.T) {
	m := ActionMenuModel{
		items:  []ActionItem{{Kind: ActionKill, Label: "Kill"}, {Kind: ActionRestart, Label: "Restart"}, {Kind: ActionViewLogs, Label: "Logs"}},
		cursor: 0,
	}
	m.MoveDown()
	if m.cursor != 1 {
		t.Errorf("want cursor=1 after MoveDown, got %d", m.cursor)
	}
	m.MoveUp()
	if m.cursor != 0 {
		t.Errorf("want cursor=0 after MoveUp, got %d", m.cursor)
	}
	// MoveUp at top does not wrap
	m.MoveUp()
	if m.cursor != 0 {
		t.Errorf("want cursor=0 (no wrap up), got %d", m.cursor)
	}
	// MoveDown at bottom does not wrap
	m.cursor = 2
	m.MoveDown()
	if m.cursor != 2 {
		t.Errorf("want cursor=2 (no wrap down), got %d", m.cursor)
	}
}

func TestActionMenuModel_SelectedItem(t *testing.T) {
	m := ActionMenuModel{
		items:  []ActionItem{{Kind: ActionKill, Label: "Kill"}, {Kind: ActionRestart, Label: "Restart"}},
		cursor: 1,
	}
	got := m.Selected()
	if got == nil || got.Kind != ActionRestart {
		t.Errorf("want ActionRestart, got %v", got)
	}
}

