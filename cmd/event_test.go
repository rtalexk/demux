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

	d.StateSet("myses:0.1", "claude", db.StateDone, "finished", db.SourceTool, false)
	d.StateSet("myses:0", "claude", db.StateDone, "finished", db.SourceTool, false)

	if err := applyPaneFocus(d, "myses:0.1"); err != nil {
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

	d.StateSet("myses:0.1", "claude", db.StateError, "boom", db.SourceTool, false)
	applyPaneFocus(d, "myses:0.1")

	st, _ := d.StateByTarget("myses:0.1")
	if st == nil {
		t.Error("error state should survive pane focus")
	}
}

func TestPaneFocusDoesNotClearFlagged(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	d.StateSet("myses:0.1", "", db.StateFlagged, "come back", db.SourceUser, false)
	applyPaneFocus(d, "myses:0.1")

	st, _ := d.StateByTarget("myses:0.1")
	if st == nil {
		t.Error("flagged state should survive pane focus")
	}
}

func TestPaneFocusNoStatesIsNoop(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	if err := applyPaneFocus(d, "work:2.3"); err != nil {
		t.Fatalf("unexpected error on empty db: %v", err)
	}
}
