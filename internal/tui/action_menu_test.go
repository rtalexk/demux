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
	for _, required := range []ActionKind{ActionKill, ActionRestart, ActionViewLogs, ActionCopyPID} {
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

func TestBuildActionItems_NoCmdlineNoCopyCommand(t *testing.T) {
	node := ProcListNode{
		Proc: proc.Process{PID: 1, Name: "bash", Cmdline: ""},
		Pane: tmux.Pane{PaneID: "%1"},
	}
	items := buildActionItems(node)
	for _, it := range items {
		if it.Kind == ActionCopyCommand {
			t.Error("ActionCopyCommand should not appear when Cmdline is empty")
		}
	}
}

func TestBuildActionItems_NoCWDNoCopyCWD(t *testing.T) {
	node := ProcListNode{
		Proc: proc.Process{PID: 1, Name: "bash", Cmdline: "bash"},
		Pane: tmux.Pane{PaneID: "%1", CWD: ""},
	}
	items := buildActionItems(node)
	for _, it := range items {
		if it.Kind == ActionCopyCWD {
			t.Error("ActionCopyCWD should not appear when CWD is empty")
		}
	}
}

func TestActionMenuModel_Navigation(t *testing.T) {
	m := ActionMenuModel{
		items:  []ActionItem{{ActionKill, "Kill"}, {ActionRestart, "Restart"}, {ActionViewLogs, "Logs"}},
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
		items:  []ActionItem{{ActionKill, "Kill"}, {ActionRestart, "Restart"}},
		cursor: 1,
	}
	got := m.Selected()
	if got == nil || got.Kind != ActionRestart {
		t.Errorf("want ActionRestart, got %v", got)
	}
}

func TestActionMenuModel_SubScroll(t *testing.T) {
	m := ActionMenuModel{
		subLines: []string{"a", "b", "c", "d"},
	}
	m.SubScrollDown()
	if m.subScrollOff != 1 {
		t.Errorf("want subScrollOff=1, got %d", m.subScrollOff)
	}
	m.SubScrollUp()
	if m.subScrollOff != 0 {
		t.Errorf("want subScrollOff=0 after SubScrollUp, got %d", m.subScrollOff)
	}
	// clamp at top
	m.SubScrollUp()
	if m.subScrollOff != 0 {
		t.Errorf("want subScrollOff=0 (no wrap), got %d", m.subScrollOff)
	}
	// clamp at bottom
	m.subScrollOff = 3
	m.SubScrollDown()
	if m.subScrollOff != 3 {
		t.Errorf("want subScrollOff=3 (clamped at last), got %d", m.subScrollOff)
	}
}
