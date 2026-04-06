package db

import (
	"testing"
)

func TestStateSet_BasicUpsert(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	if err := d.StateSet("s:0:0", "claude", StateWorking, "running tests", SourceTool); err != nil {
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

	d.StateSet("s:0:0", "claude", StateWorking, "", SourceTool)
	err := d.StateSet("s:0:0", "make", StateWorking, "", SourceTool)
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

	d.StateSet("s:0:0", "claude", StateWorking, "step 1", SourceTool)
	if err := d.StateSet("s:0:0", "claude", StateWorking, "step 2", SourceTool); err != nil {
		t.Errorf("same tool should not be locked: %v", err)
	}
}

func TestStateSet_LockFlagged_BlocksAllTools(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	d.StateSet("s:0:0", "", StateFlagged, "come back", SourceUser)
	// same tool name should still be blocked
	if err := d.StateSet("s:0:0", "claude", StateWorking, "", SourceTool); err == nil {
		t.Fatal("flagged should block all tool writes")
	}
}

func TestStateSet_UserWriteAlwaysSucceeds(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	d.StateSet("s:0:0", "claude", StateWorking, "", SourceTool)
	if err := d.StateSet("s:0:0", "", StateFlagged, "note", SourceUser); err != nil {
		t.Errorf("user write should always succeed: %v", err)
	}
}

func TestStateClear(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	d.StateSet("s:0:0", "claude", StateError, "boom", SourceTool)
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

	d.StateSet("s:0:0", "claude", StateDone, "finished", SourceTool)
	if err := d.StateDeleteIfDone("s:0:0"); err != nil {
		t.Fatal(err)
	}
	st, _ := d.StateByTarget("s:0:0")
	if st != nil {
		t.Errorf("expected done state deleted, got: %+v", st)
	}

	// non-done states must not be deleted
	d.StateSet("s:1:0", "claude", StateError, "boom", SourceTool)
	d.StateDeleteIfDone("s:1:0")
	st2, _ := d.StateByTarget("s:1:0")
	if st2 == nil {
		t.Error("non-done state should not be deleted by StateDeleteIfDone")
	}
}

func TestStateList(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	d.StateSet("s:0:0", "claude", StateWorking, "", SourceTool)
	d.StateSet("s:1:0", "make", StateError, "fail", SourceTool)

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
