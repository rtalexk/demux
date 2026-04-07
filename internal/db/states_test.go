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

	if err := d.StateSet("s:0:0", "claude", StateWorking, "running tests", SourceTool, false, nil); err != nil {
		t.Fatal(err)
	}
	st, err := d.StateByTarget("s:0:0")
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

	d.StateSet("s:0:0", "claude", StateWorking, "", SourceTool, false, nil)
	err := d.StateSet("s:0:0", "make", StateWorking, "", SourceTool, false, nil)
	if err == nil {
		t.Fatal("expected locked error, got nil")
	}
	// original owner should be unchanged
	st, _ := d.StateByTarget("s:0:0")
	if st.Tool != "claude" {
		t.Errorf("lock violated: tool changed to %q", st.Tool)
	}
}

func TestStateSet_LockWorking_SameTool(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	d.StateSet("s:0:0", "claude", StateWorking, "step 1", SourceTool, false, nil)
	if err := d.StateSet("s:0:0", "claude", StateWorking, "step 2", SourceTool, false, nil); err != nil {
		t.Errorf("same tool should not be locked: %v", err)
	}
}

func TestStateSet_LockFlagged_BlocksAllTools(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	d.StateSet("s:0:0", "", StateFlagged, "come back", SourceUser, false, nil)
	// same tool name should still be blocked
	if err := d.StateSet("s:0:0", "claude", StateWorking, "", SourceTool, false, nil); err == nil {
		t.Fatal("flagged should block all tool writes")
	}
}

func TestStateSet_ForceOverridesLock(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	// Flagged blocks tool writes normally.
	d.StateSet("s:0:0", "", StateFlagged, "come back", SourceUser, false, nil)
	if err := d.StateSet("s:0:0", "claude", StateWorking, "", SourceTool, true, nil); err != nil {
		t.Fatalf("force should override flagged lock: %v", err)
	}

	// Different-tool lock overridden by force.
	d.StateSet("s:0:0", "claude", StateWorking, "", SourceTool, false, nil)
	if err := d.StateSet("s:0:0", "make", StateWorking, "", SourceTool, true, nil); err != nil {
		t.Fatalf("force should override different-tool lock: %v", err)
	}
}

func TestStateSet_UserWriteAlwaysSucceeds(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	d.StateSet("s:0:0", "claude", StateWorking, "", SourceTool, false, nil)
	if err := d.StateSet("s:0:0", "", StateFlagged, "note", SourceUser, false, nil); err != nil {
		t.Errorf("user write should always succeed: %v", err)
	}
}

func TestStateClear(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	d.StateSet("s:0:0", "claude", StateError, "boom", SourceTool, false, nil)
	if err := d.StateClear("s:0:0"); err != nil {
		t.Fatal(err)
	}
	st, _ := d.StateByTarget("s:0:0")
	if st != nil {
		t.Errorf("expected nil after clear, got: %+v", st)
	}
}

func TestStateDeleteIfDone(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	d.StateSet("s:0:0", "claude", StateDone, "finished", SourceTool, false, nil)
	if err := d.StateDeleteIfDone("s:0:0"); err != nil {
		t.Fatal(err)
	}
	st, _ := d.StateByTarget("s:0:0")
	if st != nil {
		t.Errorf("expected done state deleted, got: %+v", st)
	}

	// non-done states must not be deleted
	d.StateSet("s:1:0", "claude", StateError, "boom", SourceTool, false, nil)
	d.StateDeleteIfDone("s:1:0")
	st2, _ := d.StateByTarget("s:1:0")
	if st2 == nil {
		t.Error("non-done state should not be deleted by StateDeleteIfDone")
	}
}

func TestStateDeleteIfResting(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	// done is cleared
	d.StateSet("s:0:0", "claude", StateDone, "finished", SourceTool, false, nil)
	d.StateDeleteIfResting("s:0:0")
	st, _ := d.StateByTarget("s:0:0")
	if st != nil {
		t.Errorf("done state should be cleared by StateDeleteIfResting, got: %+v", st)
	}

	// idle is cleared
	d.StateSet("s:1:0", "claude", StateIdle, "available", SourceTool, false, nil)
	d.StateDeleteIfResting("s:1:0")
	st2, _ := d.StateByTarget("s:1:0")
	if st2 != nil {
		t.Errorf("idle state should be cleared by StateDeleteIfResting, got: %+v", st2)
	}

	// working is not cleared
	d.StateSet("s:2:0", "claude", StateWorking, "running", SourceTool, false, nil)
	d.StateDeleteIfResting("s:2:0")
	st3, _ := d.StateByTarget("s:2:0")
	if st3 == nil {
		t.Error("working state should survive StateDeleteIfResting")
	}

	// error is not cleared
	d.StateSet("s:3:0", "claude", StateError, "boom", SourceTool, false, nil)
	d.StateDeleteIfResting("s:3:0")
	st4, _ := d.StateByTarget("s:3:0")
	if st4 == nil {
		t.Error("error state should survive StateDeleteIfResting")
	}
}

func TestStateList(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	d.StateSet("s:0:0", "claude", StateWorking, "", SourceTool, false, nil)
	d.StateSet("s:1:0", "make", StateError, "fail", SourceTool, false, nil)

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

func TestStateDeleteBySession(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	// populate: two targets for session "myapp", one for another session
	d.StateSet("myapp:0", "claude", StateWorking, "", SourceTool, false, nil)
	d.StateSet("myapp:1", "make", StateError, "fail", SourceTool, false, nil)
	d.StateSet("other:0", "claude", StateDone, "ok", SourceTool, false, nil)

	n, err := d.StateDeleteBySession("myapp")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("expected 2 deleted, got %d", n)
	}

	// myapp targets must be gone
	st, _ := d.StateByTarget("myapp:0")
	if st != nil {
		t.Error("myapp:0 should be deleted")
	}
	st2, _ := d.StateByTarget("myapp:1")
	if st2 != nil {
		t.Error("myapp:1 should be deleted")
	}

	// unrelated session must survive
	st3, _ := d.StateByTarget("other:0")
	if st3 == nil {
		t.Error("other:0 should not be deleted")
	}
}

func TestStateDeleteBySession_BareTarget(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	// target that is exactly the session name (no colon suffix)
	d.StateSet("myapp", "claude", StateWorking, "", SourceTool, false, nil)

	n, err := d.StateDeleteBySession("myapp")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("expected 1 deleted, got %d", n)
	}
	st, _ := d.StateByTarget("myapp")
	if st != nil {
		t.Error("bare target should be deleted")
	}
}

func TestStateDeleteBySession_NoneExist(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	n, err := d.StateDeleteBySession("ghost")
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("expected 0 deleted, got %d", n)
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

	if err := d.StateSet("s:0:0", "claude", StateIdle, "available", SourceTool, false, nil); err != nil {
		t.Fatal(err)
	}
	st, _ := d.StateByTarget("s:0:0")
	if st == nil || st.Value != StateIdle {
		t.Errorf("expected idle state, got: %+v", st)
	}
}

func TestStateSet_IfState_NoopWhenMismatch(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	// Set to done.
	d.StateSet("s:0:0", "claude", StateDone, "finished", SourceTool, false, nil)

	// Try to overwrite with working, but guard says "only if currently working".
	ifWorking := StateWorking
	err := d.StateSet("s:0:0", "claude", StateWorking, "running", SourceTool, false, &ifWorking)
	if err != nil {
		t.Fatalf("ifState mismatch should be a silent no-op, got: %v", err)
	}

	st, _ := d.StateByTarget("s:0:0")
	if st == nil || st.Value != StateDone {
		t.Errorf("state should remain done after mismatch, got: %+v", st)
	}
}

func TestStateSet_IfState_WritesWhenMatch(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	d.StateSet("s:0:0", "claude", StateWorking, "running", SourceTool, false, nil)

	ifWorking := StateWorking
	if err := d.StateSet("s:0:0", "claude", StateDone, "finished", SourceTool, false, &ifWorking); err != nil {
		t.Fatalf("ifState match should write: %v", err)
	}
	st, _ := d.StateByTarget("s:0:0")
	if st == nil || st.Value != StateDone {
		t.Errorf("expected done state, got: %+v", st)
	}
}

func TestStateSet_IfState_NoopWhenNoRow(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	// No row exists. Passing ifState=StateIdle (6) should be a no-op because
	// a missing row is treated as StateValue(0), not StateIdle.
	ifIdle := StateIdle
	err := d.StateSet("s:0:0", "claude", StateWorking, "running", SourceTool, false, &ifIdle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	st, _ := d.StateByTarget("s:0:0")
	if st != nil {
		t.Errorf("no row should exist: missing row != StateIdle, got: %+v", st)
	}
}
