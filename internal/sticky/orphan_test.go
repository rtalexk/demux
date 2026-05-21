package sticky

import (
	"strings"
	"testing"
)

func ranCmd(f *fakeTmux, want string) bool {
	for _, r := range f.runs {
		if strings.Join(r, " ") == want {
			return true
		}
	}
	return false
}

func TestHandleWindowAfterKill_EmptyWindow_NoOp(t *testing.T) {
	f := newFakeTmux()
	s := &Sticky{T: f}
	if err := s.HandleWindowAfterKill("", 35); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.runs) != 0 {
		t.Errorf("expected no tmux runs, got: %v", f.runs)
	}
}

func TestHandleWindowAfterKill_WindowAlreadyGone_NoOp(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -t @5 -F #{pane_id} #{@demux_slot}"] = fakeReply{err: stderrExitError("can't find window\n")}
	s := &Sticky{T: f}
	if err := s.HandleWindowAfterKill("@5", 35); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.runs) != 0 {
		t.Errorf("expected no tmux runs, got: %v", f.runs)
	}
}

func TestHandleWindowAfterKill_MultiplePanesLeft_NoOp(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -t @5 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%10 1\n%11 \n"}
	s := &Sticky{T: f}
	if err := s.HandleWindowAfterKill("@5", 35); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.runs) != 0 {
		t.Errorf("expected no tmux runs, got: %v", f.runs)
	}
}

func TestHandleWindowAfterKill_SingleNonSlotPane_NoOp(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -t @5 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%10 \n"}
	s := &Sticky{T: f}
	if err := s.HandleWindowAfterKill("@5", 35); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.runs) != 0 {
		t.Errorf("expected no tmux runs, got: %v", f.runs)
	}
}

func TestHandleWindowAfterKill_OrphanPlaceholderInactiveWindow_KillsOnly(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -t @5 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%50 1\n"}
	// Sidebar is alive elsewhere (%42); env does not point at the orphan %50.
	f.outputs["show-environment -g"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	s := &Sticky{T: f}
	if err := s.HandleWindowAfterKill("@5", 35); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ranCmd(f, "kill-pane -t %50") {
		t.Errorf("expected kill-pane for orphan placeholder, got: %v", f.runs)
	}
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if strings.HasPrefix(j, "join-pane") || strings.HasPrefix(j, "select-window") {
			t.Errorf("did not expect %q for inactive placeholder, got: %v", j, f.runs)
		}
	}
}

func TestHandleWindowAfterKill_OrphanSidebar_MovesToLastActive(t *testing.T) {
	f := newFakeTmux()
	// Source window @5 has only the sidebar slot (which is the active sidebar).
	f.outputs["list-panes -t @5 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%50 1\n"}
	// Env: sidebar tracked at %50.
	f.outputs["show-environment -g"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%50\n"}
	// Session lookup.
	f.outputs["display-message -t @5 -p #{session_id}"] = fakeReply{out: "$1\n"}
	// Sibling windows; @3 is last-active.
	f.outputs["list-windows -t $1 -F #{window_id} #{window_last_flag}"] = fakeReply{out: "@5 0\n@2 0\n@3 1\n@4 0\n"}
	// Target @3 has a placeholder slot %30 we must remove.
	f.outputs["list-panes -t @3 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%30 1\n%31 \n"}
	// Sidebar still in its source window: the move has not happened yet.
	f.outputs["display-message -p -t %50 #{window_id}"] = fakeReply{out: "@5\n"}

	s := &Sticky{T: f}
	if err := s.HandleWindowAfterKill("@5", 42); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The move is a swap-pane (no pane killed, so after-kill-pane is not
	// re-entered), then a resize to the configured width, then the parked
	// placeholder is killed in the source window.
	if !ranCmd(f, "swap-pane -d -s %50 -t %30") {
		t.Errorf("expected swap-pane moving %%50 into target slot %%30, got: %v", f.runs)
	}
	if !ranCmd(f, "resize-pane -t %50 -x 42") {
		t.Errorf("expected resize-pane to configured width 42, got: %v", f.runs)
	}
	if !ranCmd(f, "kill-pane -t %30") {
		t.Errorf("expected kill-pane for the parked placeholder %%30, got: %v", f.runs)
	}
	if !ranCmd(f, "select-window -t @3") {
		t.Errorf("expected select-window -t @3, got: %v", f.runs)
	}
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if strings.HasPrefix(j, "join-pane") {
			t.Errorf("expected swap-pane, not join-pane, when target has a slot, got: %v", f.runs)
		}
		if j == "kill-pane -t %50" {
			t.Errorf("did not expect to kill the sidebar pane %%50, got: %v", f.runs)
		}
	}
}

func TestHandleWindowAfterKill_OrphanSidebar_NoLastFlag_UsesFirstSibling(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -t @5 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%50 1\n"}
	f.outputs["show-environment -g"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%50\n"}
	f.outputs["display-message -t @5 -p #{session_id}"] = fakeReply{out: "$1\n"}
	// No window has last_flag=1; fall back to first non-source.
	f.outputs["list-windows -t $1 -F #{window_id} #{window_last_flag}"] = fakeReply{out: "@5 0\n@2 0\n@3 0\n"}
	f.outputs["list-panes -t @2 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%20 1\n"}
	f.outputs["display-message -p -t %50 #{window_id}"] = fakeReply{out: "@5\n"}

	s := &Sticky{T: f}
	if err := s.HandleWindowAfterKill("@5", 35); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ranCmd(f, "swap-pane -d -s %50 -t %20") {
		t.Errorf("expected swap-pane into fallback target slot %%20, got: %v", f.runs)
	}
	if !ranCmd(f, "select-window -t @2") {
		t.Errorf("expected select-window -t @2, got: %v", f.runs)
	}
}

func TestHandleWindowAfterKill_OrphanSidebar_SingleWindowInSession_NoOp(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -t @5 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%50 1\n"}
	f.outputs["show-environment -g"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%50\n"}
	f.outputs["display-message -t @5 -p #{session_id}"] = fakeReply{out: "$1\n"}
	f.outputs["list-windows -t $1 -F #{window_id} #{window_last_flag}"] = fakeReply{out: "@5 0\n"}

	s := &Sticky{T: f}
	if err := s.HandleWindowAfterKill("@5", 35); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if strings.HasPrefix(j, "join-pane") || strings.HasPrefix(j, "kill-pane") || strings.HasPrefix(j, "select-window") {
			t.Errorf("did not expect %q for single-window session, got: %v", j, f.runs)
		}
	}
}

func TestHandleWindowAfterKill_SplitModeOrphanSidebar_MovesToLastActive(t *testing.T) {
	f := newFakeTmux()
	// Split mode: lone pane has no @demux_slot tag but IS the active sidebar.
	f.outputs["list-panes -t @5 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%50 \n"}
	f.outputs["show-environment -g"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%50\n"}
	f.outputs["display-message -t @5 -p #{session_id}"] = fakeReply{out: "$1\n"}
	f.outputs["list-windows -t $1 -F #{window_id} #{window_last_flag}"] = fakeReply{out: "@5 0\n@3 1\n"}
	// Target @3 in split mode has no slot pane.
	f.outputs["list-panes -t @3 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%30 \n%31 \n"}
	f.outputs["display-message -p -t %50 #{window_id}"] = fakeReply{out: "@5\n"}

	s := &Sticky{T: f}
	if err := s.HandleWindowAfterKill("@5", 35); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if strings.HasPrefix(j, "kill-pane") {
			t.Errorf("split mode: did not expect any kill-pane, got: %v", f.runs)
		}
	}
	if !ranCmd(f, "join-pane -f -h -b -d -l 35 -s %50 -t @3") {
		t.Errorf("split mode: expected join-pane moving sidebar %%50 into @3, got: %v", f.runs)
	}
	if !ranCmd(f, "select-window -t @3") {
		t.Errorf("split mode: expected select-window -t @3, got: %v", f.runs)
	}
}

func TestHandleWindowAfterKill_SingleUntrackedPane_NoOp(t *testing.T) {
	f := newFakeTmux()
	// Lone pane is NOT a slot AND NOT the tracked sidebar - leave alone.
	f.outputs["list-panes -t @5 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%50 \n"}
	f.outputs["show-environment -g"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%99\n"}
	s := &Sticky{T: f}
	if err := s.HandleWindowAfterKill("@5", 35); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if strings.HasPrefix(j, "kill-pane") || strings.HasPrefix(j, "join-pane") || strings.HasPrefix(j, "select-window") {
			t.Errorf("expected no mutations for untracked single pane, got: %v", f.runs)
		}
	}
}

func TestHandleWindowAfterKill_OrphanSidebar_NoTargetSlot_StillJoins(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -t @5 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%50 1\n"}
	f.outputs["show-environment -g"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%50\n"}
	f.outputs["display-message -t @5 -p #{session_id}"] = fakeReply{out: "$1\n"}
	f.outputs["list-windows -t $1 -F #{window_id} #{window_last_flag}"] = fakeReply{out: "@5 0\n@3 1\n"}
	// Target window has no slot (slots ecosystem partially uninstalled).
	f.outputs["list-panes -t @3 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%30 \n%31 \n"}
	f.outputs["display-message -p -t %50 #{window_id}"] = fakeReply{out: "@5\n"}

	s := &Sticky{T: f}
	if err := s.HandleWindowAfterKill("@5", 35); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if strings.HasPrefix(j, "kill-pane") {
			t.Errorf("did not expect kill-pane when no target slot exists, got: %v", f.runs)
		}
	}
	if !ranCmd(f, "join-pane -f -h -b -d -l 35 -s %50 -t @3") {
		t.Errorf("expected join-pane into @3 even without target slot, got: %v", f.runs)
	}
}

// Regression: by the time the after-kill-pane hook runs, the orphan sidebar
// pane has already reflowed to fill its now-single-pane source window, so its
// live #{pane_width} is the whole window (here 203). The move must be sized
// from the caller's configured width, never from that expanded measurement -
// otherwise the sidebar lands at the wrong width.
func TestHandleWindowAfterKill_OrphanSidebar_UsesConfiguredWidthNotExpandedPane(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -t @5 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%50 1\n"}
	f.outputs["show-environment -g"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%50\n"}
	f.outputs["display-message -t @5 -p #{session_id}"] = fakeReply{out: "$1\n"}
	f.outputs["list-windows -t $1 -F #{window_id} #{window_last_flag}"] = fakeReply{out: "@5 0\n@3 1\n"}
	f.outputs["list-panes -t @3 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%30 1\n%31 \n"}
	f.outputs["display-message -p -t %50 #{window_id}"] = fakeReply{out: "@5\n"}
	// The orphan pane reports the full window width it expanded into.
	f.outputs["display-message -p -t %50 #{pane_width}"] = fakeReply{out: "203\n"}

	s := &Sticky{T: f}
	if err := s.HandleWindowAfterKill("@5", 35); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ranCmd(f, "resize-pane -t %50 -x 35") {
		t.Errorf("expected resize to configured width 35, got: %v", f.runs)
	}
	for _, r := range f.runs {
		if strings.Contains(strings.Join(r, " "), "203") {
			t.Errorf("must not size the move from the expanded pane width 203, got: %v", f.runs)
		}
	}
}

// Regression: HandleWindowAfterKill kills the target window's placeholder slot,
// which re-fires the after-kill-pane hook and spawns a second, concurrent
// pane_closed handler. Running join-pane again on a sidebar pane that is
// already in the target window nearly doubles its width. When the sidebar has
// already left its source window, the handler must do nothing.
func TestHandleWindowAfterKill_OrphanSidebar_AlreadyMoved_NoDoubleJoin(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -t @5 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%50 1\n"}
	f.outputs["show-environment -g"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%50\n"}
	f.outputs["display-message -t @5 -p #{session_id}"] = fakeReply{out: "$1\n"}
	f.outputs["list-windows -t $1 -F #{window_id} #{window_last_flag}"] = fakeReply{out: "@5 0\n@3 1\n"}
	f.outputs["list-panes -t @3 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%30 1\n%31 \n"}
	// A concurrent handler already moved %50 out of @5 into @3.
	f.outputs["display-message -p -t %50 #{window_id}"] = fakeReply{out: "@3\n"}

	s := &Sticky{T: f}
	if err := s.HandleWindowAfterKill("@5", 35); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if strings.HasPrefix(j, "join-pane") || strings.HasPrefix(j, "kill-pane") || strings.HasPrefix(j, "select-window") {
			t.Errorf("sidebar already moved; expected no mutations, got: %v", f.runs)
		}
	}
}

// ── EjectSidebarIfAloneAfterExit tests ──────────────────────────────────────

func TestEjectSidebarIfAloneAfterExit_EmptyArgs_NoOp(t *testing.T) {
	f := newFakeTmux()
	s := &Sticky{T: f}
	if err := s.EjectSidebarIfAloneAfterExit("", "", 35); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.runs) != 0 {
		t.Errorf("expected no tmux runs, got: %v", f.runs)
	}
}

func TestEjectSidebarIfAloneAfterExit_WindowGone_NoOp(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -t @2 -F #{pane_id} #{@demux_slot}"] = fakeReply{err: stderrExitError("can't find window\n")}
	s := &Sticky{T: f}
	if err := s.EjectSidebarIfAloneAfterExit("%10", "@2", 35); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.runs) != 0 {
		t.Errorf("expected no tmux runs, got: %v", f.runs)
	}
}

func TestEjectSidebarIfAloneAfterExit_MoreThanTwoPanes_NoOp(t *testing.T) {
	f := newFakeTmux()
	// Window has 3 panes: exiting + sidebar + another.
	f.outputs["list-panes -t @2 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%10 \n%42 \n%99 \n"}
	f.outputs["show-environment -g"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	s := &Sticky{T: f}
	if err := s.EjectSidebarIfAloneAfterExit("%10", "@2", 35); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.runs) != 0 {
		t.Errorf("expected no tmux runs for 3-pane window, got: %v", f.runs)
	}
}

func TestEjectSidebarIfAloneAfterExit_OnlyExitingPane_NoOp(t *testing.T) {
	f := newFakeTmux()
	// Window has only the exiting pane (no sidebar). Remaining count = 0.
	f.outputs["list-panes -t @2 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%10 \n"}
	f.outputs["show-environment -g"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	s := &Sticky{T: f}
	if err := s.EjectSidebarIfAloneAfterExit("%10", "@2", 35); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.runs) != 0 {
		t.Errorf("expected no runs when remaining count = 0, got: %v", f.runs)
	}
}

func TestEjectSidebarIfAloneAfterExit_RemainingIsNotSidebar_NoOp(t *testing.T) {
	f := newFakeTmux()
	// Window has exiting pane + a regular (non-sidebar) pane.
	f.outputs["list-panes -t @2 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%10 \n%20 \n"}
	f.outputs["show-environment -g"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	s := &Sticky{T: f}
	if err := s.EjectSidebarIfAloneAfterExit("%10", "@2", 35); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.runs) != 0 {
		t.Errorf("expected no runs when remaining pane is not sidebar, got: %v", f.runs)
	}
}

// Regression: an ephemeral window (e.g. a lazygit `new-window`) holds a
// content pane plus a placeholder slot pane. When the content pane's process
// exits naturally, pane-exited fires but after-kill-pane does not, so this is
// the only cleanup that runs. The lone placeholder must be killed so the
// window closes instead of lingering as a useless slot-only window.
func TestEjectSidebarIfAloneAfterExit_RemainingIsPlaceholderSlot_KillsIt(t *testing.T) {
	f := newFakeTmux()
	// Window @2: content pane %10 is exiting; the only other pane %42 is a
	// placeholder slot (tagged) that is NOT the tracked sidebar.
	f.outputs["list-panes -t @2 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%10 \n%42 1\n"}
	// Active sidebar is alive elsewhere (%99); env does not point at %42.
	f.outputs["show-environment -g"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%99\n"}
	s := &Sticky{T: f}
	if err := s.EjectSidebarIfAloneAfterExit("%10", "@2", 35); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ranCmd(f, "kill-pane -t %42") {
		t.Errorf("expected kill-pane for lone placeholder slot, got: %v", f.runs)
	}
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if strings.HasPrefix(j, "join-pane") || strings.HasPrefix(j, "swap-pane") {
			t.Errorf("did not expect a sidebar move for a placeholder, got: %v", f.runs)
		}
	}
}

func TestEjectSidebarIfAloneAfterExit_SidebarAlone_MovesToLastActive(t *testing.T) {
	f := newFakeTmux()
	// Ephemeral window @2: lazygit pane %10 is exiting, sidebar %42 would be left alone.
	f.outputs["list-panes -t @2 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%10 \n%42 \n"}
	f.outputs["show-environment -g"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	f.outputs["display-message -t @2 -p #{session_id}"] = fakeReply{out: "$1\n"}
	f.outputs["list-windows -t $1 -F #{window_id} #{window_last_flag}"] = fakeReply{out: "@2 0\n@1 1\n"}
	f.outputs["display-message -p -t %42 #{window_id}"] = fakeReply{out: "@2\n"}
	// Target @1 has no slot pane (split mode).
	f.outputs["list-panes -t @1 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%30 \n%31 \n"}

	s := &Sticky{T: f}
	if err := s.EjectSidebarIfAloneAfterExit("%10", "@2", 35); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ranCmd(f, "join-pane -f -h -b -d -l 35 -s %42 -t @1") {
		t.Errorf("expected join-pane moving sidebar %%42 into @1, got: %v", f.runs)
	}
	if !ranCmd(f, "select-window -t @1") {
		t.Errorf("expected select-window -t @1, got: %v", f.runs)
	}
}

func TestEjectSidebarIfAloneAfterExit_SlotsMode_SwapsIntoTargetSlot(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -t @2 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%10 \n%42 1\n"}
	f.outputs["show-environment -g"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	f.outputs["display-message -t @2 -p #{session_id}"] = fakeReply{out: "$1\n"}
	f.outputs["list-windows -t $1 -F #{window_id} #{window_last_flag}"] = fakeReply{out: "@2 0\n@1 1\n"}
	f.outputs["display-message -p -t %42 #{window_id}"] = fakeReply{out: "@2\n"}
	// Target @1 has a placeholder slot %30.
	f.outputs["list-panes -t @1 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%30 1\n%31 \n"}

	s := &Sticky{T: f}
	if err := s.EjectSidebarIfAloneAfterExit("%10", "@2", 35); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ranCmd(f, "swap-pane -d -s %42 -t %30") {
		t.Errorf("expected swap-pane into target slot %%30, got: %v", f.runs)
	}
	if !ranCmd(f, "resize-pane -t %42 -x 35") {
		t.Errorf("expected resize-pane, got: %v", f.runs)
	}
	if !ranCmd(f, "kill-pane -t %30") {
		t.Errorf("expected kill-pane for parked placeholder, got: %v", f.runs)
	}
	if !ranCmd(f, "select-window -t @1") {
		t.Errorf("expected select-window -t @1, got: %v", f.runs)
	}
}

func TestEjectSidebarIfAloneAfterExit_NoSiblingWindow_NoOp(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -t @2 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%10 \n%42 \n"}
	f.outputs["show-environment -g"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	f.outputs["display-message -t @2 -p #{session_id}"] = fakeReply{out: "$1\n"}
	// Only window in session.
	f.outputs["list-windows -t $1 -F #{window_id} #{window_last_flag}"] = fakeReply{out: "@2 0\n"}

	s := &Sticky{T: f}
	if err := s.EjectSidebarIfAloneAfterExit("%10", "@2", 35); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if strings.HasPrefix(j, "join-pane") || strings.HasPrefix(j, "select-window") {
			t.Errorf("expected no move when no sibling window, got: %v", f.runs)
		}
	}
}

func TestEjectSidebarIfAloneAfterExit_AlreadyMoved_NoOp(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -t @2 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%10 \n%42 \n"}
	f.outputs["show-environment -g"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	f.outputs["display-message -t @2 -p #{session_id}"] = fakeReply{out: "$1\n"}
	f.outputs["list-windows -t $1 -F #{window_id} #{window_last_flag}"] = fakeReply{out: "@2 0\n@1 1\n"}
	// Sidebar already in target window (concurrent handler moved it first).
	f.outputs["display-message -p -t %42 #{window_id}"] = fakeReply{out: "@1\n"}

	s := &Sticky{T: f}
	if err := s.EjectSidebarIfAloneAfterExit("%10", "@2", 35); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if strings.HasPrefix(j, "join-pane") || strings.HasPrefix(j, "select-window") {
			t.Errorf("sidebar already moved: expected no mutation, got: %v", f.runs)
		}
	}
}
