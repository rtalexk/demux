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
	if err := s.HandleWindowAfterKill(""); err != nil {
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
	if err := s.HandleWindowAfterKill("@5"); err != nil {
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
	if err := s.HandleWindowAfterKill("@5"); err != nil {
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
	if err := s.HandleWindowAfterKill("@5"); err != nil {
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
	if err := s.HandleWindowAfterKill("@5"); err != nil {
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
	// Width capture.
	f.outputs["display-message -p -t %50 #{pane_width}"] = fakeReply{out: "42\n"}

	s := &Sticky{T: f}
	if err := s.HandleWindowAfterKill("@5"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !ranCmd(f, "kill-pane -t %30") {
		t.Errorf("expected kill-pane for target's placeholder slot %%30, got: %v", f.runs)
	}
	if !ranCmd(f, "join-pane -f -h -b -d -l 42 -s %50 -t @3") {
		t.Errorf("expected join-pane moving %%50 into @3 with width 42, got: %v", f.runs)
	}
	if !ranCmd(f, "select-window -t @3") {
		t.Errorf("expected select-window -t @3, got: %v", f.runs)
	}
	for _, r := range f.runs {
		j := strings.Join(r, " ")
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
	f.outputs["display-message -p -t %50 #{pane_width}"] = fakeReply{out: "35\n"}

	s := &Sticky{T: f}
	if err := s.HandleWindowAfterKill("@5"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ranCmd(f, "join-pane -f -h -b -d -l 35 -s %50 -t @2") {
		t.Errorf("expected join-pane to fallback target @2, got: %v", f.runs)
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
	if err := s.HandleWindowAfterKill("@5"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if strings.HasPrefix(j, "join-pane") || strings.HasPrefix(j, "kill-pane") || strings.HasPrefix(j, "select-window") {
			t.Errorf("did not expect %q for single-window session, got: %v", j, f.runs)
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
	f.outputs["display-message -p -t %50 #{pane_width}"] = fakeReply{out: "35\n"}

	s := &Sticky{T: f}
	if err := s.HandleWindowAfterKill("@5"); err != nil {
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
