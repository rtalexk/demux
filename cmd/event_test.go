package cmd

import (
	"testing"

	"github.com/rtalexk/demux/internal/db"
)

func TestWindowTargetFromPane_WithDot(t *testing.T) {
	got := windowTargetFromPane("main:1.2")
	want := "main:1"
	if got != want {
		t.Errorf("windowTargetFromPane(%q) = %q, want %q", "main:1.2", got, want)
	}
}

func TestWindowTargetFromPane_NoColon(t *testing.T) {
	got := windowTargetFromPane("main")
	want := "main"
	if got != want {
		t.Errorf("windowTargetFromPane(%q) = %q, want %q", "main", got, want)
	}
}

func TestSessionTargetFromPane(t *testing.T) {
	got := sessionTargetFromPane("myses:0.1")
	want := "myses"
	if got != want {
		t.Errorf("sessionTargetFromPane(%q) = %q, want %q", "myses:0.1", got, want)
	}
}

func TestPaneFocusClearsDoneStates(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	d.StateSet("myses:0.1", "claude", db.StateDone, "finished", db.SourceTool, false, nil, "")
	d.StateSet("myses:0", "claude", db.StateDone, "finished", db.SourceTool, false, nil, "")

	if err := applyPaneFocus(d, "myses:0.1", ""); err != nil {
		t.Fatal(err)
	}

	st1, _ := d.StateByTarget("myses:0.1")
	if st1 != nil {
		t.Error("done pane state should be cleared on focus")
	}
	st2, _ := d.StateByTarget("myses:0")
	if st2 != nil {
		t.Error("done window state should be cleared on focus")
	}
}

func TestPaneFocusDoesNotClearNonDone(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	d.StateSet("myses:0.1", "claude", db.StateError, "boom", db.SourceTool, false, nil, "")
	applyPaneFocus(d, "myses:0.1", "")

	st, _ := d.StateByTarget("myses:0.1")
	if st == nil {
		t.Error("error state should survive pane focus")
	}
}

func TestPaneFocusDoesNotClearFlagged(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	d.StateSet("myses:0.1", "", db.StateFlagged, "come back", db.SourceUser, false, nil, "")
	applyPaneFocus(d, "myses:0.1", "")

	st, _ := d.StateByTarget("myses:0.1")
	if st == nil {
		t.Error("flagged state should survive pane focus")
	}
}

func TestPaneFocusNoStatesIsNoop(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	if err := applyPaneFocus(d, "work:2.3", ""); err != nil {
		t.Fatalf("unexpected error on empty db: %v", err)
	}
}

func TestPaneFocusClearsIdleStates(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	d.StateSet("myses:0.1", "claude", db.StateIdle, "available", db.SourceTool, false, nil, "")

	if err := applyPaneFocus(d, "myses:0.1", ""); err != nil {
		t.Fatal(err)
	}

	st, _ := d.StateByTarget("myses:0.1")
	if st != nil {
		t.Error("idle pane state should be cleared on focus")
	}
}

func TestApplyPaneFocus_ClearsByPaneID(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	// Pane %5 was at s:0.0 (stored target), but is now at s:1.0 (new target).
	// The done state was written when pane was at s:0.0.
	d.StateSet("s:0.0", "claude", db.StateDone, "finished", db.SourceTool, false, nil, "%5")

	// pane_focus fires with new target s:1.0 and pane_id %5.
	if err := applyPaneFocus(d, "s:1.0", "%5"); err != nil {
		t.Fatal(err)
	}

	// The state should be cleared via pane_id even though the target string didn't match.
	st, _ := d.StateByTarget("s:0.0")
	if st != nil {
		t.Errorf("state should be cleared by pane_id after pane moved, got: %+v", st)
	}
}

func TestApplyPaneFocus_EmptyPaneIDFallsBackToTarget(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	d.StateSet("s:0.0", "claude", db.StateDone, "finished", db.SourceTool, false, nil, "")

	if err := applyPaneFocus(d, "s:0.0", ""); err != nil {
		t.Fatal(err)
	}
	st, _ := d.StateByTarget("s:0.0")
	if st != nil {
		t.Errorf("legacy target should still be cleared when pane_id is empty, got: %+v", st)
	}
}

func TestApplyPaneClosed_DeletesByPaneID(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	d.StateSet("s:0.0", "claude", db.StateWorking, "", db.SourceTool, false, nil, "%7")
	if err := applyPaneClosed(d, "%7"); err != nil {
		t.Fatal(err)
	}
	st, _ := d.StateByTarget("s:0.0")
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
