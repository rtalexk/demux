package cmd

import (
	"testing"

	"github.com/rtalexk/demux/internal/db"
)

func paneTarget(paneID, windowID, sessionID string) db.Target {
	return db.Target{Type: "pane", ID: paneID, PaneID: paneID, WindowID: windowID, SessionID: sessionID}
}

func windowTarget(windowID, sessionID string) db.Target {
	return db.Target{Type: "window", ID: windowID, WindowID: windowID, SessionID: sessionID}
}

func TestPaneFocusClearsDoneStates(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	pt := paneTarget("%1", "@0", "$0")
	wt := windowTarget("@0", "$0")
	d.StateSet(pt, "claude", db.StateDone, "finished", db.SourceTool, false, nil)
	d.StateSet(wt, "claude", db.StateDone, "finished", db.SourceTool, false, nil)

	if err := applyPaneFocus(d, "%1", "@0", "$0"); err != nil {
		t.Fatal(err)
	}

	if st, _ := d.StateByID(pt); st != nil {
		t.Error("done pane state should be cleared on focus")
	}
	if st, _ := d.StateByID(wt); st != nil {
		t.Error("done window state should be cleared on focus")
	}
}

func TestPaneFocusDoesNotClearNonDone(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	pt := paneTarget("%1", "@0", "$0")
	d.StateSet(pt, "claude", db.StateError, "boom", db.SourceTool, false, nil)
	applyPaneFocus(d, "%1", "@0", "$0")

	st, _ := d.StateByID(pt)
	if st == nil {
		t.Error("error state should survive pane focus")
	}
}

func TestPaneFocusDoesNotClearFlagged(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	pt := paneTarget("%1", "@0", "$0")
	d.StateSet(pt, "", db.StateFlagged, "come back", db.SourceUser, false, nil)
	applyPaneFocus(d, "%1", "@0", "$0")

	st, _ := d.StateByID(pt)
	if st == nil {
		t.Error("flagged state should survive pane focus")
	}
}

func TestPaneFocusNoStatesIsNoop(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	if err := applyPaneFocus(d, "%1", "@0", "$0"); err != nil {
		t.Fatalf("unexpected error on empty db: %v", err)
	}
}

func TestPaneFocusClearsIdleStates(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	pt := paneTarget("%1", "@0", "$0")
	d.StateSet(pt, "claude", db.StateIdle, "available", db.SourceTool, false, nil)

	if err := applyPaneFocus(d, "%1", "@0", "$0"); err != nil {
		t.Fatal(err)
	}

	st, _ := d.StateByID(pt)
	if st != nil {
		t.Error("idle pane state should be cleared on focus")
	}
}

func TestApplyPaneFocus_ClearsByStableID(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	// State keyed by pane_id %5 — stable regardless of pane position.
	pt := paneTarget("%5", "@1", "$0")
	d.StateSet(pt, "claude", db.StateDone, "finished", db.SourceTool, false, nil)

	// pane_focus fires with the same pane_id; state should be cleared.
	if err := applyPaneFocus(d, "%5", "@1", "$0"); err != nil {
		t.Fatal(err)
	}

	st, _ := d.StateByID(pt)
	if st != nil {
		t.Errorf("state should be cleared by pane ID, got: %+v", st)
	}
}

func TestApplyPaneClosed_DeletesPaneState(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	pt := db.Target{Type: "pane", ID: "%7", PaneID: "%7", WindowID: "@0", SessionID: "$0"}
	d.StateSet(pt, "claude", db.StateWorking, "", db.SourceTool, false, nil)
	if err := applyPaneClosed(d, "%7"); err != nil {
		t.Fatal(err)
	}
	st, _ := d.StateByID(pt)
	if st != nil {
		t.Errorf("state should be deleted after pane_closed, got: %+v", st)
	}
}

func TestApplyPaneClosed_NoopUnknownPane(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	if err := applyPaneClosed(d, "%99"); err != nil {
		t.Errorf("unknown pane_id should be a no-op, got: %v", err)
	}
}

func TestApplyHookError_SetsErrorState(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	if err := applyHookErrorDB(d, "%5", "claude", "Stop", "state set failed"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	target, _ := db.ParseTargetID("%5")
	st, _ := d.StateByID(target)
	if st == nil {
		t.Fatal("expected error state to be written, got nil")
	}
	if st.Value != db.StateError {
		t.Errorf("expected StateError, got %v", st.Value)
	}
}

func TestApplyHookError_InvalidTargetIDIsNoop(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	if err := applyHookErrorDB(d, "invalid", "claude", "Stop", "failed"); err != nil {
		t.Fatalf("unexpected error for invalid target: %v", err)
	}
	states, _ := d.StateList(0, "")
	if len(states) != 0 {
		t.Errorf("expected no states for invalid target, got %d", len(states))
	}
}
