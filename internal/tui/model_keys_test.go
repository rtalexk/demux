package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rtalexk/demux/internal/config"
	"github.com/rtalexk/demux/internal/db"
	"github.com/rtalexk/demux/internal/proc"
	"github.com/rtalexk/demux/internal/tmux"
)

func newTestModel(t *testing.T) Model {
	t.Helper()
	return New(config.Config{}, nil)
}

func TestHighestPriorityPaneTarget_ReturnsHighestPriorityPane(t *testing.T) {
	states := []db.ToolState{
		{Target: db.Target{Type: db.TargetTypePane, ID: "%1", SessionID: "$1"}, Value: db.StateWorking},
		{Target: db.Target{Type: db.TargetTypePane, ID: "%2", SessionID: "$1"}, Value: db.StateError}, // highest priority
		{Target: db.Target{Type: db.TargetTypePane, ID: "%3", SessionID: "$1"}, Value: db.StateFlagged},
	}
	got := highestPriorityPaneTarget("$1", states)
	if got != "%2" {
		t.Errorf("want %%2 (error=highest), got %q", got)
	}
}

func TestHighestPriorityPaneTarget_IncludesDone(t *testing.T) {
	// Done has priority=2; Working has priority=0; Done wins.
	states := []db.ToolState{
		{Target: db.Target{Type: db.TargetTypePane, ID: "%1", SessionID: "$1"}, Value: db.StateDone},
		{Target: db.Target{Type: db.TargetTypePane, ID: "%2", SessionID: "$1"}, Value: db.StateWorking},
	}
	got := highestPriorityPaneTarget("$1", states)
	if got != "%1" {
		t.Errorf("want %%1 (done beats working), got %q", got)
	}
}

func TestHighestPriorityPaneTarget_ReturnsEmptyWhenOnlyIdle(t *testing.T) {
	states := []db.ToolState{
		{Target: db.Target{Type: db.TargetTypePane, ID: "%1", SessionID: "$1"}, Value: db.StateIdle},
		{Target: db.Target{Type: db.TargetTypePane, ID: "%2", SessionID: "$1"}, Value: db.StateIdle},
	}
	got := highestPriorityPaneTarget("$1", states)
	if got != "" {
		t.Errorf("want empty (only idle states), got %q", got)
	}
}

func TestHighestPriorityPaneTarget_IgnoresOtherSessions(t *testing.T) {
	states := []db.ToolState{
		{Target: db.Target{Type: db.TargetTypePane, ID: "%9", SessionID: "$9"}, Value: db.StateWaiting},
		{Target: db.Target{Type: db.TargetTypePane, ID: "%1", SessionID: "$1"}, Value: db.StateWorking},
	}
	got := highestPriorityPaneTarget("$1", states)
	if got != "%1" {
		t.Errorf("want %%1 (other session ignored), got %q", got)
	}
}

func TestHighestPriorityPaneTarget_IgnoresSessionTargets(t *testing.T) {
	states := []db.ToolState{
		{Target: db.Target{Type: db.TargetTypeSession, ID: "$1", SessionID: "$1"}, Value: db.StateError},
		{Target: db.Target{Type: db.TargetTypePane, ID: "%1", SessionID: "$1"}, Value: db.StateWorking},
	}
	got := highestPriorityPaneTarget("$1", states)
	// session target should be skipped; only pane target matters
	if got != "%1" {
		t.Errorf("want %%1 (session target ignored), got %q", got)
	}
}

func TestHighestPriorityPaneTarget_ReturnsEmptyForEmptyStates(t *testing.T) {
	got := highestPriorityPaneTarget("$1", nil)
	if got != "" {
		t.Errorf("want empty, got %q", got)
	}
}

func TestClearCurrentState_ClearsByTargetID(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	target := db.Target{Type: db.TargetTypePane, ID: "%5", SessionID: "$1", WindowID: "@1", PaneID: "%5"}
	_ = d.StateSet(target, "claude", db.StateWorking, "", db.SourceTool, false, nil)

	m := Model{db: d}
	cmd := m.clearCurrentState(target)
	cmd()

	st, _ := d.StateByID(target)
	if st != nil {
		t.Errorf("state should be cleared by target ID, got: %+v", st)
	}
}

func TestClearCurrentState_NoopForMissingTarget(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	target := db.Target{Type: db.TargetTypePane, ID: "%99", SessionID: "$1", WindowID: "@1", PaneID: "%99"}

	m := Model{db: d}
	cmd := m.clearCurrentState(target)
	// Should not panic or error even when no state exists.
	result := cmd()
	if _, ok := result.(statesMsg); !ok {
		t.Errorf("expected statesMsg, got %T", result)
	}
}

func TestResolveFilterKey(t *testing.T) {
	tests := []struct {
		keyStr string
		want   SidebarFilter
		wantOk bool
	}{
		{"t", FilterTmux, true},
		{"a", FilterAll, true},
		{"c", FilterConfig, true},
		{"w", FilterWorktree, true},
		{"!", FilterPriority, true},
		{"x", "", false},
		{"q", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.keyStr, func(t *testing.T) {
			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.keyStr)}
			got, ok := resolveFilterKey(msg)
			if ok != tt.wantOk {
				t.Errorf("ok: got %v, want %v", ok, tt.wantOk)
			}
			if ok && got != tt.want {
				t.Errorf("filter: got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProcListStateIdentity_EmptyPaneIDReturnsNil(t *testing.T) {
	d, _ := db.Open(":memory:")
	m := New(config.Default(), d)
	m.procList.nodes = []ProcListNode{
		{Pane: tmux.Pane{}}, // empty PaneID
	}
	m.procList.cursor = 0
	if got := m.procListStateIdentity(); got != nil {
		t.Errorf("expected nil for empty PaneID, got %+v", got)
	}
}

func TestProcListStateIdentity_EmptyWindowIDReturnsNil(t *testing.T) {
	d, _ := db.Open(":memory:")
	m := New(config.Default(), d)
	m.procList.nodes = []ProcListNode{
		{IsWindowHeader: true, Pane: tmux.Pane{}}, // empty WindowID
	}
	m.procList.cursor = 0
	if got := m.procListStateIdentity(); got != nil {
		t.Errorf("expected nil for empty WindowID, got %+v", got)
	}
}

func TestHandleKey_ActionMenu_OpensOnA_InProcList(t *testing.T) {
	m := newTestModel(t)
	m.focus = panelProcList
	m.procList.nodes = []ProcListNode{
		{IsPaneHeader: true, Pane: tmux.Pane{PaneID: "%1", Session: "s"}},
		{Proc: proc.Process{PID: 99, Name: "node", Cmdline: "node server.js"}, Depth: 1, Pane: tmux.Pane{PaneID: "%1"}},
	}
	m.procList.cursor = 1

	result, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	got := result.(Model)
	if !got.showActionMenu {
		t.Error("expected showActionMenu=true after pressing 'a' in proclist")
	}
}

func TestHandleKey_ActionMenu_DoesNotOpenOnA_InSidebar(t *testing.T) {
	m := newTestModel(t)
	m.focus = panelSidebar

	result, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	got := result.(Model)
	if got.showActionMenu {
		t.Error("expected showActionMenu=false when 'a' pressed in sidebar focus")
	}
}
