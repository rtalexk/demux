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
	if err := s.ReconcileSlots([]string{"@5"}, "@1", 35); err != nil {
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
	if err := s.ReconcileSlots([]string{"@1"}, "@3", 35); err != nil {
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
	if err := s.ReconcileSlots([]string{"@1", "@2"}, "@3", 35); err != nil {
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
	if err := s.ReconcileSlots([]string{"@5"}, "@1", 35); err != nil {
		t.Fatalf("ReconcileSlots: %v", err)
	}
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if strings.HasPrefix(j, "kill-pane") || strings.HasPrefix(j, "split-window") {
			t.Errorf("expected no mutations when reserved already covered, got: %v", r)
		}
	}
}
