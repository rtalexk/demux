package cmd

import (
	"strings"
	"testing"

	"github.com/rtalexk/demux/internal/db"
)

// stateTarget format is now "%N", "@N", or "$N" (stable tmux IDs).
// Tests use "%1" as a representative pane target.

// saveStateVars snapshots all package-level state-command globals and restores
// them via t.Cleanup. Call at the start of any test that sets these globals to
// eliminate ordering dependencies between tests.
func saveStateVars(t *testing.T) {
	t.Helper()
	prev := struct {
		target, value, tool, message, source, ifState string
		force                                         bool
		clearTarget                                   string
		clearYes                                      bool
	}{
		target:      stateTarget,
		value:       stateValue,
		tool:        stateTool,
		message:     stateMessage,
		source:      stateSource,
		ifState:     stateIfState,
		force:       stateForce,
		clearTarget: clearTarget,
		clearYes:    clearYes,
	}
	t.Cleanup(func() {
		stateTarget = prev.target
		stateValue = prev.value
		stateTool = prev.tool
		stateMessage = prev.message
		stateSource = prev.source
		stateIfState = prev.ifState
		stateForce = prev.force
		clearTarget = prev.clearTarget
		clearYes = prev.clearYes
	})
}

func TestStateSet_ToolWrite(t *testing.T) {
	saveStateVars(t)
	d, _ := db.Open(":memory:")
	defer d.Close()

	stateTarget = "%1"
	stateValue = "working"
	stateTool = "claude"
	stateMessage = "running"
	stateSource = "tool"
	stateIfState = ""

	if err := applyStateSet(d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	target, _ := db.ParseTargetID("%1")
	st, _ := d.StateByID(target)
	if st == nil || st.Value != db.StateWorking {
		t.Errorf("expected working state, got: %+v", st)
	}
}

func TestStateSet_FlaggedRequiresUserSource(t *testing.T) {
	saveStateVars(t)
	d, _ := db.Open(":memory:")
	defer d.Close()

	stateTarget = "%1"
	stateValue = "flagged"
	stateTool = "claude"
	stateSource = "tool"
	stateIfState = ""

	if err := applyStateSet(d); err == nil {
		t.Fatal("expected error: flagged requires --source user")
	}
}

func TestStateClear_FlaggedRequiresYes(t *testing.T) {
	saveStateVars(t)
	d, _ := db.Open(":memory:")
	defer d.Close()

	target := db.Target{Type: db.TargetTypePane, ID: "%1", PaneID: "%1"}
	d.StateSet(target, "", db.StateFlagged, "note", db.SourceUser, false, nil)

	clearTarget = "%1"
	clearYes = false

	if err := applyStateClear(d); err == nil {
		t.Fatal("expected error: flagged requires --yes")
	}

	clearYes = true
	if err := applyStateClear(d); err != nil {
		t.Fatalf("with --yes should succeed: %v", err)
	}
	st, _ := d.StateByID(target)
	if st != nil {
		t.Error("state should be cleared")
	}
}

func TestStateClear_NonFlagged_NoYesRequired(t *testing.T) {
	saveStateVars(t)
	d, _ := db.Open(":memory:")
	defer d.Close()

	target := db.Target{Type: db.TargetTypePane, ID: "%1", PaneID: "%1"}
	d.StateSet(target, "claude", db.StateError, "boom", db.SourceTool, false, nil)

	clearTarget = "%1"
	clearYes = false

	if err := applyStateClear(d); err != nil {
		t.Fatalf("non-flagged clear should not require --yes: %v", err)
	}
}

func TestStateSet_IdleIsValid(t *testing.T) {
	saveStateVars(t)
	d, _ := db.Open(":memory:")
	defer d.Close()

	stateTarget = "%1"
	stateValue = "idle"
	stateTool = "claude"
	stateMessage = "available"
	stateSource = "tool"
	stateForce = false
	stateIfState = ""

	if err := applyStateSet(d); err != nil {
		t.Fatalf("--state idle should be valid: %v", err)
	}
	target, _ := db.ParseTargetID("%1")
	st, _ := d.StateByID(target)
	if st == nil || st.Value != db.StateIdle {
		t.Errorf("expected idle state, got: %+v", st)
	}
}

func TestStateSet_IfState_NoopWhenMismatch(t *testing.T) {
	saveStateVars(t)
	d, _ := db.Open(":memory:")
	defer d.Close()

	// Pre-set state to done.
	target := db.Target{Type: db.TargetTypePane, ID: "%1", PaneID: "%1"}
	d.StateSet(target, "claude", db.StateDone, "finished", db.SourceTool, false, nil)

	stateTarget = "%1"
	stateValue = "waiting"
	stateTool = "claude"
	stateMessage = "awaiting input"
	stateSource = "tool"
	stateForce = false
	stateIfState = "working"

	if err := applyStateSet(d); err != nil {
		t.Fatalf("--if-state mismatch should succeed silently: %v", err)
	}
	st, _ := d.StateByID(target)
	if st == nil || st.Value != db.StateDone {
		t.Errorf("state should remain done, got: %+v", st)
	}
}

func TestStateSet_IfState_WritesWhenMatch(t *testing.T) {
	saveStateVars(t)
	d, _ := db.Open(":memory:")
	defer d.Close()

	target := db.Target{Type: db.TargetTypePane, ID: "%1", PaneID: "%1"}
	d.StateSet(target, "claude", db.StateWorking, "running", db.SourceTool, false, nil)

	stateTarget = "%1"
	stateValue = "done"
	stateTool = "claude"
	stateMessage = "task complete"
	stateSource = "tool"
	stateForce = false
	stateIfState = "working"

	if err := applyStateSet(d); err != nil {
		t.Fatalf("--if-state match should write: %v", err)
	}
	st, _ := d.StateByID(target)
	if st == nil || st.Value != db.StateDone {
		t.Errorf("expected done state, got: %+v", st)
	}
}

func TestStateSet_IfState_InvalidValue(t *testing.T) {
	saveStateVars(t)
	d, _ := db.Open(":memory:")
	defer d.Close()

	stateTarget = "%1"
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

func TestStateSet_InvalidTargetID(t *testing.T) {
	saveStateVars(t)
	d, _ := db.Open(":memory:")
	defer d.Close()

	stateTarget = "s:0.0" // old format — invalid
	stateValue = "working"
	stateTool = "claude"
	stateSource = "tool"
	stateForce = false
	stateIfState = ""

	err := applyStateSet(d)
	if err == nil {
		t.Fatal("expected error for invalid --target-id format")
	}
	if !strings.Contains(err.Error(), "--target-id") {
		t.Errorf("error should mention --target-id, got: %v", err)
	}
}

func TestStateClear_InvalidTargetID(t *testing.T) {
	saveStateVars(t)
	d, _ := db.Open(":memory:")
	defer d.Close()

	clearTarget = "s:0.0" // old format — invalid
	clearYes = false

	err := applyStateClear(d)
	if err == nil {
		t.Fatal("expected error for invalid --target-id format")
	}
}
