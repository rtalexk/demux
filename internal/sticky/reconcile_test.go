package sticky

import (
	"strings"
	"testing"
)

func TestReconcileSlots_InstallsMissing(t *testing.T) {
	f := newFakeTmux()
	// No existing slot panes.
	f.outputs["list-panes -aF #{window_id} #{pane_id} #{@demux_slot}"] = fakeReply{out: ""}
	// EnsureSlotInWindow will call FindSlotInWindow then split-window.
	f.outputs["list-panes -t @5 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%500 \n"}
	f.outputs["split-window -f -h -b -d -l 35 -t @5 -P -F #{pane_id} "+SlotPlaceholderCmd] = fakeReply{out: "%900\n"}

	s := &Sticky{T: f}
	if err := s.ReconcileSlots([]string{"@5"}, "@1", "", 35); err != nil {
		t.Fatalf("ReconcileSlots: %v", err)
	}
	saw := false
	for _, r := range f.runs {
		if strings.HasPrefix(strings.Join(r, " "), "set-option -p -t %900 "+SlotMarker) {
			saw = true
		}
	}
	if !saw {
		t.Errorf("expected slot marker set on new pane, got: %v", f.runs)
	}
}

func TestReconcileSlots_EvictsNonReserved(t *testing.T) {
	f := newFakeTmux()
	// @1 (reserved), @2 (NOT reserved), @3 (current, has slot, must NOT be killed).
	f.outputs["list-panes -aF #{window_id} #{pane_id} #{@demux_slot}"] = fakeReply{out: "@1 %10 1\n@2 %20 1\n@3 %30 1\n"}
	// @1 already has slot - no install path needed.
	s := &Sticky{T: f}
	if err := s.ReconcileSlots([]string{"@1"}, "@3", "", 35); err != nil {
		t.Fatalf("ReconcileSlots: %v", err)
	}
	var killed10, killed20, killed30 bool
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if j == "kill-pane -t %10" {
			killed10 = true
		}
		if j == "kill-pane -t %20" {
			killed20 = true
		}
		if j == "kill-pane -t %30" {
			killed30 = true
		}
	}
	if killed10 {
		t.Errorf("did not expect kill of reserved %%10")
	}
	if !killed20 {
		t.Errorf("expected kill of non-reserved %%20, got: %v", f.runs)
	}
	if killed30 {
		t.Errorf("did not expect kill of current-window slot %%30")
	}
}

func TestReconcileSlots_NoOpWhenAlreadyMatching(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -aF #{window_id} #{pane_id} #{@demux_slot}"] = fakeReply{out: "@1 %10 1\n@2 %20 1\n"}
	s := &Sticky{T: f}
	if err := s.ReconcileSlots([]string{"@1", "@2"}, "@3", "", 35); err != nil {
		t.Fatalf("ReconcileSlots: %v", err)
	}
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if strings.HasPrefix(j, "kill-pane") || strings.HasPrefix(j, "split-window") {
			t.Errorf("expected no mutations, got: %v", r)
		}
	}
}

func TestReconcileSlots_IgnoresNonSlotPanes(t *testing.T) {
	f := newFakeTmux()
	// Mix of slot and non-slot panes; only slots should be considered.
	f.outputs["list-panes -aF #{window_id} #{pane_id} #{@demux_slot}"] = fakeReply{out: "@5 %50 1\n@5 %51 \n@6 %60 \n"}
	s := &Sticky{T: f}
	if err := s.ReconcileSlots([]string{"@5"}, "@1", "", 35); err != nil {
		t.Fatalf("ReconcileSlots: %v", err)
	}
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if strings.HasPrefix(j, "kill-pane") || strings.HasPrefix(j, "split-window") {
			t.Errorf("expected no mutations when reserved already covered, got: %v", r)
		}
	}
}

// Reproduces the corrupt-state bug observed in the wild: a window that ended up
// with two slot-tagged panes (race in EnsureSlotInWindow + swap-pane edge case).
// Reconcile must keep one and kill the rest so the duplicate placeholder pane
// stops haunting the user.
func TestReconcileSlots_DedupesDuplicatesWithinReservedWindow(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -aF #{window_id} #{pane_id} #{@demux_slot}"] = fakeReply{out: "@1 %10 1\n@1 %11 1\n"}
	s := &Sticky{T: f}
	if err := s.ReconcileSlots([]string{"@1"}, "@9", "", 35); err != nil {
		t.Fatalf("ReconcileSlots: %v", err)
	}
	var killed10, killed11 bool
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if j == "kill-pane -t %10" {
			killed10 = true
		}
		if j == "kill-pane -t %11" {
			killed11 = true
		}
	}
	if killed10 == killed11 {
		t.Errorf("expected exactly one of %%10/%%11 to be killed (kept the other), got killed10=%v killed11=%v runs=%v",
			killed10, killed11, f.runs)
	}
}

// Same dedupe, but for the current window: the kept slot must not be killed
// (that would yank the sidebar), and the extra must still go.
func TestReconcileSlots_DedupesDuplicatesInCurrentWindow(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -aF #{window_id} #{pane_id} #{@demux_slot}"] = fakeReply{out: "@9 %90 1\n@9 %91 1\n"}
	s := &Sticky{T: f}
	if err := s.ReconcileSlots(nil, "@9", "", 35); err != nil {
		t.Fatalf("ReconcileSlots: %v", err)
	}
	var killed90, killed91 bool
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if j == "kill-pane -t %90" {
			killed90 = true
		}
		if j == "kill-pane -t %91" {
			killed91 = true
		}
	}
	if killed90 == killed91 {
		t.Errorf("expected exactly one of %%90/%%91 to be killed (kept the other), got killed90=%v killed91=%v runs=%v",
			killed90, killed91, f.runs)
	}
}

// When the env-tracked active sidebar pane is among the duplicates in the
// current window, ReconcileSlots must preserve it as survivor even when it
// is not the first-listed pane. Otherwise prune would kill the user's
// sidebar and the next Follow would cascade into UninstallSlots, tearing
// down every other slot on the server.
func TestReconcileSlots_DedupesDuplicatesInCurrentWindow_PreservesProtected(t *testing.T) {
	f := newFakeTmux()
	// %90 listed first, %91 is the env-tracked active sidebar.
	f.outputs["list-panes -aF #{window_id} #{pane_id} #{@demux_slot}"] = fakeReply{out: "@9 %90 1\n@9 %91 1\n"}
	s := &Sticky{T: f}
	if err := s.ReconcileSlots(nil, "@9", "%91", 35); err != nil {
		t.Fatalf("ReconcileSlots: %v", err)
	}
	var killed90, killed91 bool
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if j == "kill-pane -t %90" {
			killed90 = true
		}
		if j == "kill-pane -t %91" {
			killed91 = true
		}
	}
	if !killed90 {
		t.Errorf("expected %%90 (non-protected duplicate) to be killed, got: %v", f.runs)
	}
	if killed91 {
		t.Errorf("must not kill %%91 (env-tracked active sidebar), got: %v", f.runs)
	}
}

// Symmetric to the above: protected pane happens to be listed first. The
// non-protected duplicate must still be the one killed.
func TestReconcileSlots_DedupesDuplicatesInCurrentWindow_ProtectedListedFirst(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -aF #{window_id} #{pane_id} #{@demux_slot}"] = fakeReply{out: "@9 %91 1\n@9 %90 1\n"}
	s := &Sticky{T: f}
	if err := s.ReconcileSlots(nil, "@9", "%91", 35); err != nil {
		t.Fatalf("ReconcileSlots: %v", err)
	}
	var killed90, killed91 bool
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if j == "kill-pane -t %90" {
			killed90 = true
		}
		if j == "kill-pane -t %91" {
			killed91 = true
		}
	}
	if !killed90 {
		t.Errorf("expected %%90 (non-protected duplicate) to be killed, got: %v", f.runs)
	}
	if killed91 {
		t.Errorf("must not kill %%91 (env-tracked active sidebar), got: %v", f.runs)
	}
}

// Non-reserved, non-current window with duplicate slots: kill them all.
func TestReconcileSlots_KillsAllDuplicatesInNonReservedWindow(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -aF #{window_id} #{pane_id} #{@demux_slot}"] = fakeReply{out: "@7 %70 1\n@7 %71 1\n"}
	s := &Sticky{T: f}
	if err := s.ReconcileSlots(nil, "@9", "", 35); err != nil {
		t.Fatalf("ReconcileSlots: %v", err)
	}
	var killed70, killed71 bool
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if j == "kill-pane -t %70" {
			killed70 = true
		}
		if j == "kill-pane -t %71" {
			killed71 = true
		}
	}
	if !killed70 || !killed71 {
		t.Errorf("expected both %%70 and %%71 killed (non-reserved window), got killed70=%v killed71=%v runs=%v",
			killed70, killed71, f.runs)
	}
}
