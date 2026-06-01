package db

import (
	"testing"
)

func TestStateValue_Priority(t *testing.T) {
	tests := []struct {
		value StateValue
		want  int
	}{
		{0, 0},            // implicit zero / unknown
		{StateWorking, 0}, // in-progress but not urgent
		{StateIdle, -1},
		{StateFlagged, 1},
		{StateDone, 2},
		{StateError, 3},
		{StateWaiting, 4}, // highest — needs human input
	}
	for _, tt := range tests {
		t.Run(tt.value.String(), func(t *testing.T) {
			if got := tt.value.Priority(); got != tt.want {
				t.Errorf("Priority() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestStateSet_BasicUpsert(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	target := Target{Type: TargetTypePane, ID: "%1", SessionID: "$0", WindowID: "@0", PaneID: "%1"}
	if err := d.StateSet(target, "claude", StateWorking, "running tests", SourceTool, false, nil); err != nil {
		t.Fatal(err)
	}
	st, err := d.StateByID(target)
	if err != nil {
		t.Fatal(err)
	}
	if st == nil {
		t.Fatal("expected state, got nil")
	}
	if st.Value != StateWorking || st.Tool != "claude" || st.Message != "running tests" {
		t.Errorf("unexpected state: %+v", st)
	}
}

func TestStateSet_LockWorking_DifferentTool(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	target := Target{Type: TargetTypePane, ID: "%1", SessionID: "$0", WindowID: "@0", PaneID: "%1"}
	d.StateSet(target, "claude", StateWorking, "", SourceTool, false, nil)
	err := d.StateSet(target, "make", StateWorking, "", SourceTool, false, nil)
	if err == nil {
		t.Fatal("expected locked error, got nil")
	}
	// original owner should be unchanged
	st, _ := d.StateByID(target)
	if st.Tool != "claude" {
		t.Errorf("lock violated: tool changed to %q", st.Tool)
	}
}

func TestStateSet_LockWorking_SameTool(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	target := Target{Type: TargetTypePane, ID: "%1", SessionID: "$0", WindowID: "@0", PaneID: "%1"}
	d.StateSet(target, "claude", StateWorking, "step 1", SourceTool, false, nil)
	if err := d.StateSet(target, "claude", StateWorking, "step 2", SourceTool, false, nil); err != nil {
		t.Errorf("same tool should not be locked: %v", err)
	}
}

func TestStateSet_LockFlagged_BlocksAllTools(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	target := Target{Type: TargetTypePane, ID: "%1", SessionID: "$0", WindowID: "@0", PaneID: "%1"}
	d.StateSet(target, "", StateFlagged, "come back", SourceUser, false, nil)
	// same tool name should still be blocked
	if err := d.StateSet(target, "claude", StateWorking, "", SourceTool, false, nil); err == nil {
		t.Fatal("flagged should block all tool writes")
	}
}

func TestStateSet_ForceOverridesLock(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	target := Target{Type: TargetTypePane, ID: "%1", SessionID: "$0", WindowID: "@0", PaneID: "%1"}
	// Flagged blocks tool writes normally.
	d.StateSet(target, "", StateFlagged, "come back", SourceUser, false, nil)
	if err := d.StateSet(target, "claude", StateWorking, "", SourceTool, true, nil); err != nil {
		t.Fatalf("force should override flagged lock: %v", err)
	}

	// Different-tool lock overridden by force.
	d.StateSet(target, "claude", StateWorking, "", SourceTool, false, nil)
	if err := d.StateSet(target, "make", StateWorking, "", SourceTool, true, nil); err != nil {
		t.Fatalf("force should override different-tool lock: %v", err)
	}
}

func TestStateSet_UserWriteAlwaysSucceeds(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	target := Target{Type: TargetTypePane, ID: "%1", SessionID: "$0", WindowID: "@0", PaneID: "%1"}
	d.StateSet(target, "claude", StateWorking, "", SourceTool, false, nil)
	if err := d.StateSet(target, "", StateFlagged, "note", SourceUser, false, nil); err != nil {
		t.Errorf("user write should always succeed: %v", err)
	}
}

func TestStateClear(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	target := Target{Type: TargetTypePane, ID: "%1", SessionID: "$0", WindowID: "@0", PaneID: "%1"}
	d.StateSet(target, "claude", StateError, "boom", SourceTool, false, nil)
	if err := d.StateClear(target); err != nil {
		t.Fatal(err)
	}
	st, _ := d.StateByID(target)
	if st != nil {
		t.Errorf("expected nil after clear, got: %+v", st)
	}
}

func TestStateDeleteIfResting(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	// done is cleared
	target1 := Target{Type: TargetTypePane, ID: "%1", SessionID: "$0", WindowID: "@0", PaneID: "%1"}
	d.StateSet(target1, "claude", StateDone, "finished", SourceTool, false, nil)
	d.StateDeleteIfResting(target1)
	st, _ := d.StateByID(target1)
	if st != nil {
		t.Errorf("done state should be cleared by StateDeleteIfResting, got: %+v", st)
	}

	// idle is cleared
	target2 := Target{Type: TargetTypePane, ID: "%2", SessionID: "$0", WindowID: "@0", PaneID: "%2"}
	d.StateSet(target2, "claude", StateIdle, "available", SourceTool, false, nil)
	d.StateDeleteIfResting(target2)
	st2, _ := d.StateByID(target2)
	if st2 != nil {
		t.Errorf("idle state should be cleared by StateDeleteIfResting, got: %+v", st2)
	}

	// working is not cleared
	target3 := Target{Type: TargetTypePane, ID: "%3", SessionID: "$0", WindowID: "@0", PaneID: "%3"}
	d.StateSet(target3, "claude", StateWorking, "running", SourceTool, false, nil)
	d.StateDeleteIfResting(target3)
	st3, _ := d.StateByID(target3)
	if st3 == nil {
		t.Error("working state should survive StateDeleteIfResting")
	}

	// error is not cleared
	target4 := Target{Type: TargetTypePane, ID: "%4", SessionID: "$0", WindowID: "@0", PaneID: "%4"}
	d.StateSet(target4, "claude", StateError, "boom", SourceTool, false, nil)
	d.StateDeleteIfResting(target4)
	st4, _ := d.StateByID(target4)
	if st4 == nil {
		t.Error("error state should survive StateDeleteIfResting")
	}
}

func TestStateList(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	t1 := Target{Type: TargetTypePane, ID: "%1", SessionID: "$0", WindowID: "@0", PaneID: "%1"}
	t2 := Target{Type: TargetTypePane, ID: "%2", SessionID: "$0", WindowID: "@0", PaneID: "%2"}
	d.StateSet(t1, "claude", StateWorking, "", SourceTool, false, nil)
	d.StateSet(t2, "make", StateError, "fail", SourceTool, false, nil)

	all, err := d.StateList(0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 states, got %d", len(all))
	}

	filtered, _ := d.StateList(StateError, "")
	if len(filtered) != 1 || filtered[0].Value != StateError {
		t.Errorf("filter by value failed: %+v", filtered)
	}
}

func TestStateIdle_ParseAndString(t *testing.T) {
	v, err := ParseStateValue("idle")
	if err != nil {
		t.Fatalf("ParseStateValue(\"idle\") error: %v", err)
	}
	if v != StateIdle {
		t.Errorf("expected StateIdle, got %v", v)
	}
	if v.String() != "idle" {
		t.Errorf("String() = %q, want \"idle\"", v.String())
	}
}

func TestStateIdle_SetAndRetrieve(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	target := Target{Type: TargetTypePane, ID: "%1", SessionID: "$0", WindowID: "@0", PaneID: "%1"}
	if err := d.StateSet(target, "claude", StateIdle, "available", SourceTool, false, nil); err != nil {
		t.Fatal(err)
	}
	st, _ := d.StateByID(target)
	if st == nil || st.Value != StateIdle {
		t.Errorf("expected idle state, got: %+v", st)
	}
}

func TestStateSet_IfState_NoopWhenMismatch(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	target := Target{Type: TargetTypePane, ID: "%1", SessionID: "$0", WindowID: "@0", PaneID: "%1"}
	// Set to done.
	d.StateSet(target, "claude", StateDone, "finished", SourceTool, false, nil)

	// Try to overwrite with working, but guard says "only if currently working".
	ifWorking := StateWorking
	err := d.StateSet(target, "claude", StateWorking, "running", SourceTool, false, &ifWorking)
	if err != nil {
		t.Fatalf("ifState mismatch should be a silent no-op, got: %v", err)
	}

	st, _ := d.StateByID(target)
	if st == nil || st.Value != StateDone {
		t.Errorf("state should remain done after mismatch, got: %+v", st)
	}
}

func TestStateSet_IfState_WritesWhenMatch(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	target := Target{Type: TargetTypePane, ID: "%1", SessionID: "$0", WindowID: "@0", PaneID: "%1"}
	d.StateSet(target, "claude", StateWorking, "running", SourceTool, false, nil)

	ifWorking := StateWorking
	if err := d.StateSet(target, "claude", StateDone, "finished", SourceTool, false, &ifWorking); err != nil {
		t.Fatalf("ifState match should write: %v", err)
	}
	st, _ := d.StateByID(target)
	if st == nil || st.Value != StateDone {
		t.Errorf("expected done state, got: %+v", st)
	}
}

func TestStateSet_IfState_NoopWhenNoRow(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	target := Target{Type: TargetTypePane, ID: "%1", SessionID: "$0", WindowID: "@0", PaneID: "%1"}
	// No row exists. Passing ifState=StateIdle (6) should be a no-op because
	// a missing row is treated as StateValue(0), not StateIdle.
	ifIdle := StateIdle
	err := d.StateSet(target, "claude", StateWorking, "running", SourceTool, false, &ifIdle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	st, _ := d.StateByID(target)
	if st != nil {
		t.Errorf("no row should exist: missing row != StateIdle, got: %+v", st)
	}
}

func TestStateGCOrphaned_DeletesByType(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	pane1 := Target{Type: TargetTypePane, ID: "%12", SessionID: "$0", WindowID: "@0", PaneID: "%12"}
	pane2 := Target{Type: TargetTypePane, ID: "%13", SessionID: "$0", WindowID: "@0", PaneID: "%13"}
	d.StateSet(pane1, "claude", StateWorking, "", SourceTool, false, nil)
	d.StateSet(pane2, "claude", StateWorking, "", SourceTool, false, nil)

	// %12 is live; %13 is gone.
	n, err := d.StateGCOrphaned(
		map[string]bool{"%12": true},
		map[string]bool{},
		map[string]bool{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("expected 1 deleted, got %d", n)
	}
	if st, _ := d.StateByID(pane1); st == nil {
		t.Error("live pane state should survive GC")
	}
	if st, _ := d.StateByID(pane2); st != nil {
		t.Error("orphaned pane state should be deleted")
	}
}

func TestStateGCOrphaned_DeletesWindows(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	win1 := Target{Type: TargetTypeWindow, ID: "@5", SessionID: "$0", WindowID: "@5"}
	win2 := Target{Type: TargetTypeWindow, ID: "@6", SessionID: "$0", WindowID: "@6"}
	d.StateSet(win1, "claude", StateWorking, "", SourceTool, false, nil)
	d.StateSet(win2, "claude", StateWorking, "", SourceTool, false, nil)

	// @5 is live; @6 is gone.
	n, err := d.StateGCOrphaned(
		map[string]bool{},
		map[string]bool{"@5": true},
		map[string]bool{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("expected 1 deleted, got %d", n)
	}
	if st, _ := d.StateByID(win1); st == nil {
		t.Error("live window state should survive GC")
	}
	if st, _ := d.StateByID(win2); st != nil {
		t.Error("orphaned window state should be deleted")
	}
}

func TestStateGCOrphaned_DeletesSessions(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	sess1 := Target{Type: TargetTypeSession, ID: "$1", SessionID: "$1"}
	sess2 := Target{Type: TargetTypeSession, ID: "$2", SessionID: "$2"}
	d.StateSet(sess1, "claude", StateWorking, "", SourceTool, false, nil)
	d.StateSet(sess2, "claude", StateWorking, "", SourceTool, false, nil)

	// $1 is live; $2 is gone.
	n, err := d.StateGCOrphaned(
		map[string]bool{},
		map[string]bool{},
		map[string]bool{"$1": true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("expected 1 deleted, got %d", n)
	}
	if st, _ := d.StateByID(sess1); st == nil {
		t.Error("live session state should survive GC")
	}
	if st, _ := d.StateByID(sess2); st != nil {
		t.Error("orphaned session state should be deleted")
	}
}

func TestStateGCOrphaned_EmptyLiveSetsDeleteAll(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	pane := Target{Type: TargetTypePane, ID: "%1", SessionID: "$0", WindowID: "@0", PaneID: "%1"}
	win := Target{Type: TargetTypeWindow, ID: "@1", SessionID: "$0", WindowID: "@1"}
	d.StateSet(pane, "claude", StateWorking, "", SourceTool, false, nil)
	d.StateSet(win, "claude", StateWorking, "", SourceTool, false, nil)

	// All live sets empty: every record should be deleted.
	n, err := d.StateGCOrphaned(
		map[string]bool{},
		map[string]bool{},
		map[string]bool{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("expected 2 deleted, got %d", n)
	}
	if st, _ := d.StateByID(pane); st != nil {
		t.Error("pane record should be deleted when live pane set is empty")
	}
	if st, _ := d.StateByID(win); st != nil {
		t.Error("window record should be deleted when live window set is empty")
	}
}

func TestStateDeleteBySession_DeletesAll(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	t1 := Target{Type: TargetTypePane, ID: "%5", SessionID: "$0", WindowID: "@0", PaneID: "%5"}
	t2 := Target{Type: TargetTypePane, ID: "%6", SessionID: "$0", WindowID: "@0", PaneID: "%6"}
	t3 := Target{Type: TargetTypePane, ID: "%7", SessionID: "$1", WindowID: "@1", PaneID: "%7"}

	d.StateSet(t1, "claude", StateWorking, "", SourceTool, false, nil)
	d.StateSet(t2, "make", StateDone, "", SourceTool, false, nil)
	d.StateSet(t3, "claude", StateDone, "", SourceTool, false, nil)

	// Delete all states for session $0
	n, err := d.StateDeleteBySession("$0")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("expected 2 deleted, got %d", n)
	}
	if st, _ := d.StateByID(t1); st != nil {
		t.Error("t1 should be deleted")
	}
	if st, _ := d.StateByID(t2); st != nil {
		t.Error("t2 should be deleted")
	}
	if st, _ := d.StateByID(t3); st == nil {
		t.Error("t3 (different session) should survive")
	}
}

func TestStateDeleteByWindow_CascadesPanesAndWindow(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	// Window @5 with two panes, plus an unrelated window @6 with one pane.
	win5 := Target{Type: TargetTypeWindow, ID: "@5", SessionID: "$0", WindowID: "@5"}
	p1 := Target{Type: TargetTypePane, ID: "%10", SessionID: "$0", WindowID: "@5", PaneID: "%10"}
	p2 := Target{Type: TargetTypePane, ID: "%11", SessionID: "$0", WindowID: "@5", PaneID: "%11"}
	win6 := Target{Type: TargetTypeWindow, ID: "@6", SessionID: "$0", WindowID: "@6"}
	p3 := Target{Type: TargetTypePane, ID: "%12", SessionID: "$0", WindowID: "@6", PaneID: "%12"}

	d.StateSet(win5, "claude", StateWorking, "", SourceTool, false, nil)
	d.StateSet(p1, "claude", StateWorking, "", SourceTool, false, nil)
	d.StateSet(p2, "make", StateDone, "", SourceTool, false, nil)
	d.StateSet(win6, "claude", StateWorking, "", SourceTool, false, nil)
	d.StateSet(p3, "claude", StateWorking, "", SourceTool, false, nil)

	n, err := d.StateDeleteByWindow("@5")
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("expected 3 rows deleted (window + 2 panes), got %d", n)
	}
	for _, gone := range []Target{win5, p1, p2} {
		if st, _ := d.StateByID(gone); st != nil {
			t.Errorf("expected %s deleted, still present", gone.ID)
		}
	}
	for _, kept := range []Target{win6, p3} {
		if st, _ := d.StateByID(kept); st == nil {
			t.Errorf("expected %s to survive, was deleted", kept.ID)
		}
	}
}

func TestStateSet_StoresPaneID(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	target := Target{Type: TargetTypePane, ID: "%1", SessionID: "$0", WindowID: "@0", PaneID: "%1"}
	if err := d.StateSet(target, "claude", StateWorking, "running", SourceTool, false, nil); err != nil {
		t.Fatal(err)
	}
	st, _ := d.StateByID(target)
	if st == nil {
		t.Fatal("expected state, got nil")
	}
	if st.Target.PaneID != "%1" {
		t.Errorf("expected PaneID %%1, got %q", st.Target.PaneID)
	}
}

func TestStateSet_PreservesSessionWindowPaneIDs(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	target := Target{Type: TargetTypePane, ID: "%1", SessionID: "$0", WindowID: "@0", PaneID: "%1"}
	if err := d.StateSet(target, "claude", StateWorking, "running", SourceTool, false, nil); err != nil {
		t.Fatal(err)
	}
	st, _ := d.StateByID(target)
	if st == nil {
		t.Fatal("expected state, got nil")
	}
	if st.Target.SessionID != "$0" {
		t.Errorf("expected SessionID $0, got %q", st.Target.SessionID)
	}
	if st.Target.WindowID != "@0" {
		t.Errorf("expected WindowID @0, got %q", st.Target.WindowID)
	}
	if st.Target.PaneID != "%1" {
		t.Errorf("expected PaneID %%1, got %q", st.Target.PaneID)
	}
}

func TestStateSet_EmptySessionWindowPaneIDs(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	// Session target has no WindowID or PaneID
	target := Target{Type: TargetTypeSession, ID: "$0", SessionID: "$0"}
	if err := d.StateSet(target, "claude", StateWorking, "running", SourceTool, false, nil); err != nil {
		t.Fatal(err)
	}
	st, _ := d.StateByID(target)
	if st == nil {
		t.Fatal("expected state, got nil")
	}
	if st.Target.WindowID != "" {
		t.Errorf("expected empty WindowID, got %q", st.Target.WindowID)
	}
	if st.Target.PaneID != "" {
		t.Errorf("expected empty PaneID, got %q", st.Target.PaneID)
	}
}

func TestStateRefreshParentIDs_Updates(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	target := Target{Type: TargetTypePane, ID: "%1", SessionID: "$0", WindowID: "@0", PaneID: "%1"}
	if err := d.StateSet(target, "claude", StateWorking, "", SourceTool, false, nil); err != nil {
		t.Fatal(err)
	}

	// Pane moved to a different session/window.
	if err := d.StateRefreshParentIDs("%1", "@1", "$1"); err != nil {
		t.Fatal(err)
	}

	st, err := d.StateByID(Target{Type: TargetTypePane, ID: "%1"})
	if err != nil {
		t.Fatal(err)
	}
	if st == nil {
		t.Fatal("expected state, got nil")
	}
	if st.Target.SessionID != "$1" {
		t.Errorf("expected SessionID $1, got %q", st.Target.SessionID)
	}
	if st.Target.WindowID != "@1" {
		t.Errorf("expected WindowID @1, got %q", st.Target.WindowID)
	}
}

func TestStateRefreshParentIDs_NoopWhenMatch(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	target := Target{Type: TargetTypePane, ID: "%1", SessionID: "$0", WindowID: "@0", PaneID: "%1"}
	if err := d.StateSet(target, "claude", StateWorking, "", SourceTool, false, nil); err != nil {
		t.Fatal(err)
	}

	// IDs already match; no update should occur.
	if err := d.StateRefreshParentIDs("%1", "@0", "$0"); err != nil {
		t.Fatal(err)
	}

	st, _ := d.StateByID(Target{Type: TargetTypePane, ID: "%1"})
	if st == nil {
		t.Fatal("expected state, got nil")
	}
	if st.Target.SessionID != "$0" || st.Target.WindowID != "@0" {
		t.Errorf("IDs should be unchanged, got session=%q window=%q", st.Target.SessionID, st.Target.WindowID)
	}
}

func TestStateRefreshParentIDs_NoopWhenNoRow(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	// No row exists for %99; should not error.
	if err := d.StateRefreshParentIDs("%99", "@0", "$0"); err != nil {
		t.Fatalf("expected no error for missing row, got: %v", err)
	}
}

func TestStateSet_ParentIDMissing_WritesThrough(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	// Write initial state with empty session_id (simulates a failed resolveParentIDs).
	bare := Target{Type: TargetTypePane, ID: "%1", SessionID: "", WindowID: "", PaneID: "%1"}
	if err := d.StateSet(bare, "claude", StateWorking, "step 1", SourceTool, false, nil); err != nil {
		t.Fatal(err)
	}

	// Second write has same value+tool but provides the session_id — must write through.
	withIDs := Target{Type: TargetTypePane, ID: "%1", SessionID: "$0", WindowID: "@0", PaneID: "%1"}
	if err := d.StateSet(withIDs, "claude", StateWorking, "step 1", SourceTool, false, nil); err != nil {
		t.Fatalf("parentIDMissing write-through should not error: %v", err)
	}

	st, _ := d.StateByID(Target{Type: TargetTypePane, ID: "%1"})
	if st == nil {
		t.Fatal("expected state, got nil")
	}
	if st.Target.SessionID != "$0" {
		t.Errorf("expected SessionID $0 after write-through, got %q", st.Target.SessionID)
	}
}
