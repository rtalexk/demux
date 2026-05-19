package cmd

import (
	"strings"
	"testing"

	"github.com/rtalexk/demux/internal/db"
	"github.com/rtalexk/demux/internal/sticky"
)

func paneTarget(paneID, windowID, sessionID string) db.Target {
	return db.Target{Type: db.TargetTypePane, ID: paneID, PaneID: paneID, WindowID: windowID, SessionID: sessionID}
}

func windowTarget(windowID, sessionID string) db.Target {
	return db.Target{Type: db.TargetTypeWindow, ID: windowID, WindowID: windowID, SessionID: sessionID}
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

func TestApplyPaneFocus_PaneOnlyIDClearsPaneState(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	// Old hook style: only pane-id is known (window-id/session-id empty).
	// Pane-level state must still be cleared even without parent IDs.
	pt := paneTarget("%5", "@1", "$0")
	d.StateSet(pt, "claude", db.StateDone, "finished", db.SourceTool, false, nil)

	if err := applyPaneFocus(d, "%5", "", ""); err != nil {
		t.Fatal(err)
	}

	st, _ := d.StateByID(pt)
	if st != nil {
		t.Errorf("pane state should be cleared even when window/session IDs absent, got: %+v", st)
	}
}

func TestApplyPaneClosed_DeletesPaneState(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	pt := db.Target{Type: db.TargetTypePane, ID: "%7", PaneID: "%7", WindowID: "@0", SessionID: "$0"}
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

// fakeStickyTmux is a minimal recorder for sticky.Tmux used in event tests.
type fakeStickyTmux struct {
	outputs map[string]string
	errs    map[string]error
	runs    [][]string
}

func (f *fakeStickyTmux) Run(args ...string) error {
	f.runs = append(f.runs, append([]string(nil), args...))
	return nil
}

func (f *fakeStickyTmux) Output(args ...string) (string, error) {
	key := strings.Join(args, " ")
	if err, ok := f.errs[key]; ok {
		return "", err
	}
	return f.outputs[key], nil
}

var _ sticky.Tmux = (*fakeStickyTmux)(nil)

func TestApplyPaneClosed_ClearsStickyEnvVar(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()
	pt := db.Target{Type: db.TargetTypePane, ID: "%42", PaneID: "%42"}
	d.StateSet(pt, "claude", db.StateWorking, "", db.SourceTool, false, nil)

	fake := &fakeStickyTmux{
		outputs: map[string]string{
			"show-environment -g": "DEMUX_STICKY_PANE__dev_ttys001=%42\nOTHER=keep\n",
		},
	}
	orig := stickyTmuxForEvents
	stickyTmuxForEvents = func() sticky.Tmux { return fake }
	defer func() { stickyTmuxForEvents = orig }()

	if err := applyPaneClosed(d, "%42"); err != nil {
		t.Fatal(err)
	}
	sawUnset := false
	for _, r := range fake.runs {
		if strings.Join(r, " ") == "set-environment -gu DEMUX_STICKY_PANE__dev_ttys001" {
			sawUnset = true
		}
	}
	if !sawUnset {
		t.Errorf("expected sticky env unset for %%42, got runs: %v", fake.runs)
	}
}
