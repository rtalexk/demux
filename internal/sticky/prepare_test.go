package sticky

import (
	"strings"
	"testing"
)

func TestPrepareWindow_SlotsDisabled_NoOp(t *testing.T) {
	f := newFakeTmux()
	s := &Sticky{T: f, Slots: false}
	if err := s.PrepareWindow("@5", 35); err != nil {
		t.Fatalf("PrepareWindow: %v", err)
	}
	if len(f.runs) != 0 {
		t.Errorf("expected no tmux runs, got: %v", f.runs)
	}
}

func TestPrepareWindow_EmptyTarget_NoOp(t *testing.T) {
	f := newFakeTmux()
	s := &Sticky{T: f, Slots: true}
	if err := s.PrepareWindow("", 35); err != nil {
		t.Fatalf("PrepareWindow: %v", err)
	}
	if len(f.runs) != 0 {
		t.Errorf("expected no tmux runs, got: %v", f.runs)
	}
}

func TestPrepareWindow_NoActiveSidebar_NoOp(t *testing.T) {
	f := newFakeTmux()
	// AnySlotExists returns false.
	f.outputs["list-panes -aF #{pane_id} #{@demux_slot}"] = fakeReply{out: "%1 \n%2 \n"}
	s := &Sticky{T: f, Slots: true}
	if err := s.PrepareWindow("@5", 35); err != nil {
		t.Fatalf("PrepareWindow: %v", err)
	}
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if strings.HasPrefix(j, "set-environment") || strings.HasPrefix(j, "split-window") {
			t.Errorf("expected no env writes / no splits, got: %v", r)
		}
	}
}

func TestPrepareWindow_PushesMRUAndReconciles(t *testing.T) {
	f := newFakeTmux()
	// Sidebar active somewhere.
	f.outputs["list-panes -aF #{pane_id} #{@demux_slot}"] = fakeReply{out: "%99 1\n"}
	// MRU pre-seeded with @5 at head (simulating the just-pushed state -
	// fakeTmux does not propagate writes back to scripted reads).
	f.outputs["show-environment -g "+MRUEnvKey] = fakeReply{out: MRUEnvKey + "=@5\n"}
	// Current window/session.
	f.outputs["display-message -p #{session_id}:#{window_id}"] = fakeReply{out: "$1:@1\n"}
	// Reservation queries.
	f.outputs["list-windows -aF #{window_id}"] = fakeReply{out: "@1\n@5\n"}
	f.outputs["list-windows -t $1 -F #{window_id} #{window_last_flag}"] = fakeReply{out: "@1 1\n"}
	f.outputs["display-message -p #{client_last_session}"] = fakeReply{out: "\n"}
	// Reconcile pane scan: only @1 has a slot today (the sidebar).
	f.outputs["list-panes -aF #{window_id} #{pane_id} #{@demux_slot}"] = fakeReply{out: "@1 %99 1\n"}
	// EnsureSlotInWindow on @5 should split.
	f.outputs["list-panes -t @5 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%5 \n"}
	f.outputs["split-window -f -h -b -d -l 35 -t @5 -P -F #{pane_id} demux sidebar slot"] = fakeReply{out: "%50\n"}

	s := &Sticky{T: f, Slots: true}
	if err := s.PrepareWindow("@5", 35); err != nil {
		t.Fatalf("PrepareWindow: %v", err)
	}
	sawPush := false
	for _, r := range f.runs {
		if strings.HasPrefix(strings.Join(r, " "), "set-environment -g "+MRUEnvKey+" @5") {
			sawPush = true
		}
	}
	if !sawPush {
		t.Errorf("expected MRU push for @5, got: %v", f.runs)
	}
	sawTag := false
	for _, r := range f.runs {
		if strings.Join(r, " ") == "set-option -p -t %50 @demux_slot 1" {
			sawTag = true
		}
	}
	if !sawTag {
		t.Errorf("expected new slot %%50 tagged in @5, got: %v", f.runs)
	}
}
