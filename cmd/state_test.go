package cmd

import (
	"strings"
	"testing"

	"github.com/rtalexk/demux/internal/db"
)

func TestStateSet_ToolWrite(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	stateTarget = "s:0.0"
	stateValue = "working"
	stateTool = "claude"
	stateMessage = "running"
	stateSource = "tool"
	stateIfState = ""

	if err := applyStateSet(d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	st, _ := d.StateByTarget("s:0.0")
	if st == nil || st.Value != db.StateWorking {
		t.Errorf("expected working state, got: %+v", st)
	}
}

func TestStateSet_FlaggedRequiresUserSource(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	stateTarget = "s:0.0"
	stateValue = "flagged"
	stateTool = "claude"
	stateSource = "tool"
	stateIfState = ""

	if err := applyStateSet(d); err == nil {
		t.Fatal("expected error: flagged requires --source user")
	}
}

func TestStateClear_FlaggedRequiresYes(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	d.StateSet("s:0.0", "", db.StateFlagged, "note", db.SourceUser, false, nil)

	clearTarget = "s:0.0"
	clearYes = false

	if err := applyStateClear(d); err == nil {
		t.Fatal("expected error: flagged requires --yes")
	}

	clearYes = true
	if err := applyStateClear(d); err != nil {
		t.Fatalf("with --yes should succeed: %v", err)
	}
	st, _ := d.StateByTarget("s:0.0")
	if st != nil {
		t.Error("state should be cleared")
	}
}

func TestStateClear_NonFlagged_NoYesRequired(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	d.StateSet("s:0.0", "claude", db.StateError, "boom", db.SourceTool, false, nil)

	clearTarget = "s:0.0"
	clearYes = false

	if err := applyStateClear(d); err != nil {
		t.Fatalf("non-flagged clear should not require --yes: %v", err)
	}
}

func TestStateSet_IdleIsValid(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	stateTarget = "s:0.0"
	stateValue = "idle"
	stateTool = "claude"
	stateMessage = "available"
	stateSource = "tool"
	stateForce = false
	stateIfState = ""

	if err := applyStateSet(d); err != nil {
		t.Fatalf("--state idle should be valid: %v", err)
	}
	st, _ := d.StateByTarget("s:0.0")
	if st == nil || st.Value != db.StateIdle {
		t.Errorf("expected idle state, got: %+v", st)
	}
}

func TestStateSet_IfState_NoopWhenMismatch(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	// Pre-set state to done.
	d.StateSet("s:0.0", "claude", db.StateDone, "finished", db.SourceTool, false, nil)

	stateTarget = "s:0.0"
	stateValue = "waiting"
	stateTool = "claude"
	stateMessage = "awaiting input"
	stateSource = "tool"
	stateForce = false
	stateIfState = "working"

	if err := applyStateSet(d); err != nil {
		t.Fatalf("--if-state mismatch should succeed silently: %v", err)
	}
	st, _ := d.StateByTarget("s:0.0")
	if st == nil || st.Value != db.StateDone {
		t.Errorf("state should remain done, got: %+v", st)
	}
}

func TestStateSet_IfState_WritesWhenMatch(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	d.StateSet("s:0.0", "claude", db.StateWorking, "running", db.SourceTool, false, nil)

	stateTarget = "s:0.0"
	stateValue = "done"
	stateTool = "claude"
	stateMessage = "task complete"
	stateSource = "tool"
	stateForce = false
	stateIfState = "working"

	if err := applyStateSet(d); err != nil {
		t.Fatalf("--if-state match should write: %v", err)
	}
	st, _ := d.StateByTarget("s:0.0")
	if st == nil || st.Value != db.StateDone {
		t.Errorf("expected done state, got: %+v", st)
	}
}

func TestStateSet_IfState_InvalidValue(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	stateTarget = "s:0.0"
	stateValue = "working"
	stateTool = "claude"
	stateSource = "tool"
	stateForce = false
	stateIfState = "bogus"

	err := applyStateSet(d)
	if err == nil {
		t.Fatal("expected error for invalid --if-state value")
	}
	if !strings.Contains(err.Error(), "--if-state") {
		t.Errorf("error should mention --if-state, got: %v", err)
	}
}
