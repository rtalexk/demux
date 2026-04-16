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

func TestActionMenu_NavigateDown(t *testing.T) {
	m := newTestModel(t)
	m.showActionMenu = true
	m.actionMenu = ActionMenuModel{
		items:  []ActionItem{{Kind: ActionKill, Label: "Kill"}, {Kind: ActionRestart, Label: "Restart"}},
		cursor: 0,
	}
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	got := result.(Model)
	if got.actionMenu.cursor != 1 {
		t.Errorf("want cursor=1 after j, got %d", got.actionMenu.cursor)
	}
}

func TestActionMenu_NavigateUp(t *testing.T) {
	m := newTestModel(t)
	m.showActionMenu = true
	m.actionMenu = ActionMenuModel{
		items:  []ActionItem{{Kind: ActionKill, Label: "Kill"}, {Kind: ActionRestart, Label: "Restart"}},
		cursor: 1,
	}
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	got := result.(Model)
	if got.actionMenu.cursor != 0 {
		t.Errorf("want cursor=0 after k, got %d", got.actionMenu.cursor)
	}
}

func TestActionMenu_EscCloses(t *testing.T) {
	m := newTestModel(t)
	m.showActionMenu = true
	m.actionMenu = ActionMenuModel{items: []ActionItem{{Kind: ActionKill, Label: "Kill"}}}
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := result.(Model)
	if got.showActionMenu {
		t.Error("expected showActionMenu=false after Esc")
	}
}

func TestActionMenu_QCloses(t *testing.T) {
	m := newTestModel(t)
	m.showActionMenu = true
	m.actionMenu = ActionMenuModel{items: []ActionItem{{Kind: ActionKill, Label: "Kill"}}}
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	got := result.(Model)
	if got.showActionMenu {
		t.Error("expected showActionMenu=false after q")
	}
}

func TestActionMenu_ShortcutX_TriggersKill(t *testing.T) {
	m := newTestModel(t)
	m.showActionMenu = true
	m.actionMenu = ActionMenuModel{
		items: []ActionItem{
			{Kind: ActionKill, Label: "Kill", Shortcut: "x"},
			{Kind: ActionRestart, Label: "Restart", Shortcut: "r"},
		},
		cursor: 1, // cursor starts on Restart
		target: ProcListNode{Proc: proc.Process{PID: 42, Name: "node"}, Pane: tmux.Pane{PaneID: "%1"}},
	}
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	got := result.(Model)
	// x should jump cursor to Kill (index 0) and open kill confirm sub-popup
	if got.actionMenu.subPopup != SubPopupKillConfirm {
		t.Errorf("expected SubPopupKillConfirm after 'x', got %v", got.actionMenu.subPopup)
	}
	if got.actionMenu.cursor != 0 {
		t.Errorf("expected cursor=0 (Kill item) after 'x', got %d", got.actionMenu.cursor)
	}
}

func TestProcList_X_OpensKillConfirm(t *testing.T) {
	m := newTestModel(t)
	m.focus = panelProcList
	m.procList.nodes = []ProcListNode{
		{IsPaneHeader: true, Pane: tmux.Pane{PaneID: "%1", Session: "s"}},
		{Proc: proc.Process{PID: 77, Name: "node", Cmdline: "node server.js"}, Depth: 1, Pane: tmux.Pane{PaneID: "%1"}},
	}
	m.procList.cursor = 1

	result, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	got := result.(Model)
	if !got.showConfirm {
		t.Error("expected showConfirm=true after 'x' on process node")
	}
	if got.confirmCmd == nil {
		t.Error("expected confirmCmd to be set")
	}
}

func TestProcList_X_NoopOnPaneHeader(t *testing.T) {
	m := newTestModel(t)
	m.focus = panelProcList
	m.procList.nodes = []ProcListNode{
		{IsPaneHeader: true, Pane: tmux.Pane{PaneID: "%1", Session: "s"}},
	}
	m.procList.cursor = 0

	result, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	got := result.(Model)
	if got.showConfirm {
		t.Error("expected showConfirm=false when 'x' pressed on pane header (no process)")
	}
}

func TestActionMenu_ShortcutForMissingAction_IsNoop(t *testing.T) {
	m := newTestModel(t)
	m.showActionMenu = true
	// Menu has only Kill — pressing 'c' (CopyCommand, not in list) should be a no-op.
	m.actionMenu = ActionMenuModel{
		items:  []ActionItem{{Kind: ActionKill, Label: "Kill", Shortcut: "k"}},
		cursor: 0,
	}
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	got := result.(Model)
	if !got.showActionMenu {
		t.Error("expected menu to remain open after no-op shortcut")
	}
	if got.actionMenu.cursor != 0 {
		t.Errorf("expected cursor unchanged at 0, got %d", got.actionMenu.cursor)
	}
}

func TestOpenActionMenu_RecoversSuppressedCWD(t *testing.T) {
	m := New(config.Default(), nil)
	// Simulate a process node whose CWD was suppressed in displayPane (matches window CWD).
	m.panes = []tmux.Pane{
		{PaneID: "%1", Session: "s", CWD: "/real/cwd"},
	}
	m.procList.nodes = []ProcListNode{
		{
			Proc: proc.Process{PID: 42, Name: "node"},
			Pane: tmux.Pane{PaneID: "%1", Session: "s", CWD: ""}, // suppressed
		},
	}
	m.procList.cursor = 0
	m.focus = panelProcList

	got := m.openActionMenu()
	if got.actionMenu.target.Pane.CWD != "/real/cwd" {
		t.Errorf("expected target CWD=/real/cwd, got %q", got.actionMenu.target.Pane.CWD)
	}
}

func TestOpenActionMenu_NoopOnPaneHeader(t *testing.T) {
	m := New(config.Default(), nil)
	m.procList.nodes = []ProcListNode{
		{IsPaneHeader: true, Pane: tmux.Pane{PaneID: "%1", Session: "s"}},
	}
	m.procList.cursor = 0
	m.focus = panelProcList

	got := m.openActionMenu()
	if got.showActionMenu {
		t.Error("expected showActionMenu=false for pane header node")
	}
}

func TestOpenActionMenu_NoopOnWindowHeader(t *testing.T) {
	m := New(config.Default(), nil)
	m.procList.nodes = []ProcListNode{
		{IsWindowHeader: true, Pane: tmux.Pane{PaneID: "%1"}},
	}
	m.procList.cursor = 0
	m.focus = panelProcList

	got := m.openActionMenu()
	if got.showActionMenu {
		t.Error("expected showActionMenu=false for window header node")
	}
}

func TestExecuteActionMenuItem_Restart_ErrorReturnsProcActionMsg(t *testing.T) {
	m := newTestModel(t)
	m.showActionMenu = true
	m.actionMenu = ActionMenuModel{
		items:  []ActionItem{{Kind: ActionRestart, Label: "Restart", Shortcut: "r"}},
		cursor: 0,
		target: ProcListNode{
			Proc: proc.Process{PID: 42, Name: "node"},
			Pane: tmux.Pane{PaneID: "DEFINITELY-INVALID-PANE-ID-demux-test-xyz"},
		},
	}

	_, cmd := m.executeActionMenuItem()
	if cmd == nil {
		t.Fatal("expected non-nil cmd from restart action")
	}
	msg := cmd()
	if msg == nil {
		t.Fatal("expected procActionMsg on tmux failure, got nil")
	}
	if _, ok := msg.(procActionMsg); !ok {
		t.Errorf("expected procActionMsg, got %T: %v", msg, msg)
	}
}

func TestActionMenu_SubPopup_EscReturnsToMenu(t *testing.T) {
	m := newTestModel(t)
	m.showActionMenu = true
	m.actionMenu = ActionMenuModel{
		items:    []ActionItem{{Kind: ActionKill, Label: "Kill"}},
		subPopup: SubPopupKillConfirm,
	}
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := result.(Model)
	if !got.showActionMenu {
		t.Error("expected showActionMenu=true (action menu stays open after sub-popup close)")
	}
	if got.actionMenu.subPopup != SubPopupNone {
		t.Errorf("expected subPopup=None after Esc, got %v", got.actionMenu.subPopup)
	}
}
