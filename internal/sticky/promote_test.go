package sticky

import (
	"strings"
	"testing"
)

func TestMaybePromoteSlot_NotSlotsMode_Noop(t *testing.T) {
	f := newFakeTmux()
	s := &Sticky{T: f, Slots: false}
	if err := s.MaybePromoteSlot(ShowOpts{Cmd: "demux --sticky", Width: 35}); err != nil {
		t.Fatalf("MaybePromoteSlot: %v", err)
	}
	if len(f.runs) != 0 {
		t.Errorf("expected no tmux runs when slots disabled, got: %v", f.runs)
	}
}

func TestMaybePromoteSlot_EnvSetAndAlive_Noop(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	f.outputs["list-panes -aF #{pane_id}"] = fakeReply{out: "%42\n"}
	s := &Sticky{T: f, Slots: true}
	if err := s.MaybePromoteSlot(ShowOpts{Cmd: "demux --sticky", Width: 35}); err != nil {
		t.Fatalf("MaybePromoteSlot: %v", err)
	}
	for _, r := range f.runs {
		if strings.HasPrefix(strings.Join(r, " "), "respawn-pane") {
			t.Errorf("expected no respawn (env set, alive), got: %v", r)
		}
	}
}

func TestMaybePromoteSlot_NoAnySlot_Noop(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{err: stderrExitError("unknown variable\n")}
	// No slots anywhere.
	f.outputs["list-panes -aF #{pane_id} #{@demux_slot}"] = fakeReply{out: ""}
	s := &Sticky{T: f, Slots: true}
	if err := s.MaybePromoteSlot(ShowOpts{Cmd: "demux --sticky", Width: 35}); err != nil {
		t.Fatalf("MaybePromoteSlot: %v", err)
	}
	for _, r := range f.runs {
		if strings.HasPrefix(strings.Join(r, " "), "respawn-pane") {
			t.Errorf("expected no respawn (user hasn't opted into slots), got: %v", r)
		}
	}
}

func TestMaybePromoteSlot_NoSlotInCurrentWindow_Noop(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{err: stderrExitError("unknown variable\n")}
	// Some other window has a slot, but not the current.
	f.outputs["list-panes -aF #{pane_id} #{@demux_slot}"] = fakeReply{out: "%99 1\n"}
	f.outputs["display-message -p #{session_id}:#{window_id}"] = fakeReply{out: "$1:@7\n"}
	f.outputs["list-panes -t $1:@7 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%50 \n"}
	s := &Sticky{T: f, Slots: true}
	if err := s.MaybePromoteSlot(ShowOpts{Cmd: "demux --sticky", Width: 35}); err != nil {
		t.Fatalf("MaybePromoteSlot: %v", err)
	}
	for _, r := range f.runs {
		if strings.HasPrefix(strings.Join(r, " "), "respawn-pane") {
			t.Errorf("expected no respawn (current window has no slot), got: %v", r)
		}
	}
}

func TestMaybePromoteSlot_HappyPath_RespawnsAndTracks(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{err: stderrExitError("unknown variable\n")}
	f.outputs["list-panes -aF #{pane_id} #{@demux_slot}"] = fakeReply{out: "%50 1\n"}
	f.outputs["display-message -p #{session_id}:#{window_id}"] = fakeReply{out: "$1:@7\n"}
	f.outputs["list-panes -t $1:@7 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%50 1\n"}

	s := &Sticky{T: f, Slots: true}
	if err := s.MaybePromoteSlot(ShowOpts{Cmd: "demux --sticky", Width: 35}); err != nil {
		t.Fatalf("MaybePromoteSlot: %v", err)
	}

	var respawned, wroteEnv bool
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if j == "respawn-pane -k -t %50 demux --sticky" {
			respawned = true
		}
		if j == "set-environment -g DEMUX_STICKY_PANE__dev_ttys001 %50" {
			wroteEnv = true
		}
	}
	if !respawned {
		t.Errorf("expected respawn-pane of %%50, got: %v", f.runs)
	}
	if !wroteEnv {
		t.Errorf("expected env to be written to %%50, got: %v", f.runs)
	}
}

// Regression: MaybePromoteSlot must NOT call ReconcileSlots. Reconcile fires
// kill-pane for stale/duplicate slots, which cascades into a backgrounded
// pane_closed sweep storm. Doing that on every nav into a fresh-slot window
// froze tmux input.
func TestMaybePromoteSlot_DoesNotReconcileOrKill(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{err: stderrExitError("unknown variable\n")}
	// Two duplicate slots in current window plus a stale slot in a non-reserved
	// window. A naive Show()-based promote would kill both extras.
	f.outputs["list-panes -aF #{pane_id} #{@demux_slot}"] = fakeReply{out: "%50 1\n%51 1\n%99 1\n"}
	f.outputs["display-message -p #{session_id}:#{window_id}"] = fakeReply{out: "$1:@7\n"}
	f.outputs["list-panes -t $1:@7 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%50 1\n"}

	s := &Sticky{T: f, Slots: true}
	if err := s.MaybePromoteSlot(ShowOpts{Cmd: "demux --sticky", Width: 35}); err != nil {
		t.Fatalf("MaybePromoteSlot: %v", err)
	}
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if strings.HasPrefix(j, "kill-pane") {
			t.Errorf("promote must not kill panes (cleanup belongs to slots prune / new_window), got: %v", r)
		}
		if strings.HasPrefix(j, "list-windows") {
			t.Errorf("promote must not enumerate windows for reconciliation, got: %v", r)
		}
	}
}
