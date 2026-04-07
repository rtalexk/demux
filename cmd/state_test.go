package cmd

import (
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

	if err := applyStateSet(d); err == nil {
		t.Fatal("expected error: flagged requires --source user")
	}
}

func TestStateClear_FlaggedRequiresYes(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	d.StateSet("s:0.0", "", db.StateFlagged, "note", db.SourceUser)

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

	d.StateSet("s:0.0", "claude", db.StateError, "boom", db.SourceTool)

	clearTarget = "s:0.0"
	clearYes = false

	if err := applyStateClear(d); err != nil {
		t.Fatalf("non-flagged clear should not require --yes: %v", err)
	}
}
