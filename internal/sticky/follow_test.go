package sticky

import (
	"strings"
	"testing"
)

func TestFollow_EnvUnset_Noop(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{err: stderrExitError("unknown variable\n")}
	s := &Sticky{T: f}
	if err := s.Follow(); err != nil {
		t.Fatalf("Follow: %v", err)
	}
	for _, r := range f.runs {
		if strings.HasPrefix(strings.Join(r, " "), "join-pane") {
			t.Errorf("expected no join-pane (noop), got: %v", r)
		}
	}
}

func TestFollow_PaneAlreadyInCurrentWindow_Noop(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	f.outputs["list-panes -aF #{pane_id}"] = fakeReply{out: "%42\n"}
	f.outputs["display-message -p #{session_id}:#{window_id}"] = fakeReply{out: "$1:@7\n"}
	f.outputs["display-message -p -t %42 #{window_id}"] = fakeReply{out: "@7\n"}
	s := &Sticky{T: f}
	if err := s.Follow(); err != nil {
		t.Fatalf("Follow: %v", err)
	}
	for _, r := range f.runs {
		if strings.HasPrefix(strings.Join(r, " "), "join-pane") {
			t.Errorf("expected no join-pane (same window), got: %v", r)
		}
	}
}

func TestFollow_DifferentWindowSameSession_JoinsAndKeepsFocus(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	f.outputs["list-panes -aF #{pane_id}"] = fakeReply{out: "%42\n"}
	f.outputs["display-message -p #{session_id}:#{window_id}"] = fakeReply{out: "$1:@9\n"}
	f.outputs["display-message -p -t %42 #{window_id}"] = fakeReply{out: "@7\n"}
	f.outputs["display-message -p -t %42 #{pane_width}"] = fakeReply{out: "48\n"}
	s := &Sticky{T: f}
	if err := s.Follow(); err != nil {
		t.Fatalf("Follow: %v", err)
	}
	want := "join-pane -f -h -b -d -l 48 -s %42 -t $1:@9"
	found := false
	for _, r := range f.runs {
		if strings.Join(r, " ") == want {
			found = true
		}
	}
	if !found {
		t.Errorf("want join-pane call %q, got: %v", want, f.runs)
	}
}

func TestFollow_DifferentSession_JoinsWithFFlagAndPreservedWidth(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	f.outputs["list-panes -aF #{pane_id}"] = fakeReply{out: "%42\n"}
	f.outputs["display-message -p #{session_id}:#{window_id}"] = fakeReply{out: "$2:@9\n"}
	f.outputs["display-message -p -t %42 #{window_id}"] = fakeReply{out: "@3\n"}
	f.outputs["display-message -p -t %42 #{pane_width}"] = fakeReply{out: "48\n"}
	s := &Sticky{T: f}
	if err := s.Follow(); err != nil {
		t.Fatalf("Follow: %v", err)
	}
	want := "join-pane -f -h -b -d -l 48 -s %42 -t $2:@9"
	found := false
	for _, r := range f.runs {
		if strings.Join(r, " ") == want {
			found = true
		}
	}
	if !found {
		t.Errorf("want join-pane call %q, got: %v", want, f.runs)
	}
}

func TestFollow_InvalidWidth_FallsBackToDefault(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	f.outputs["list-panes -aF #{pane_id}"] = fakeReply{out: "%42\n"}
	f.outputs["display-message -p #{session_id}:#{window_id}"] = fakeReply{out: "$2:@9\n"}
	f.outputs["display-message -p -t %42 #{window_id}"] = fakeReply{out: "@3\n"}
	f.outputs["display-message -p -t %42 #{pane_width}"] = fakeReply{out: "0\n"}
	s := &Sticky{T: f}
	if err := s.Follow(); err != nil {
		t.Fatalf("Follow: %v", err)
	}
	want := "join-pane -f -h -b -d -l 35 -s %42 -t $2:@9"
	found := false
	for _, r := range f.runs {
		if strings.Join(r, " ") == want {
			found = true
		}
	}
	if !found {
		t.Errorf("want fallback-width join-pane call %q, got: %v", want, f.runs)
	}
}

func TestFollow_SlotsMode_SwapsAndResizes(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	f.outputs["list-panes -aF #{pane_id}"] = fakeReply{out: "%42\n"}
	f.outputs["display-message -p #{session_id}:#{window_id}"] = fakeReply{out: "$1:@9\n"}
	f.outputs["display-message -p -t %42 #{window_id}"] = fakeReply{out: "@7\n"}
	f.outputs["display-message -p -t %42 #{pane_width}"] = fakeReply{out: "50\n"}
	f.outputs["list-panes -t $1:@9 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%50 1\n%51 \n"}
	s := &Sticky{T: f, Slots: true}
	if err := s.Follow(); err != nil {
		t.Fatalf("Follow: %v", err)
	}
	var sawSwap, sawResize, sawJoin bool
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if j == "swap-pane -d -s %42 -t %50" {
			sawSwap = true
		}
		if j == "resize-pane -t %42 -x 50" {
			sawResize = true
		}
		if strings.HasPrefix(j, "join-pane") {
			sawJoin = true
		}
	}
	if !sawSwap {
		t.Errorf("expected swap-pane -d -s %%42 -t %%50, got: %v", f.runs)
	}
	if !sawResize {
		t.Errorf("expected resize-pane -t %%42 -x 50, got: %v", f.runs)
	}
	if sawJoin {
		t.Errorf("expected no join-pane in slots mode, got: %v", f.runs)
	}
}

func TestFollow_SlotsMode_NoSlotInTarget_CreatesOne(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	f.outputs["list-panes -aF #{pane_id}"] = fakeReply{out: "%42\n"}
	f.outputs["display-message -p #{session_id}:#{window_id}"] = fakeReply{out: "$1:@9\n"}
	f.outputs["display-message -p -t %42 #{window_id}"] = fakeReply{out: "@7\n"}
	f.outputs["display-message -p -t %42 #{pane_width}"] = fakeReply{out: "50\n"}
	// Window @9 has no slot initially.
	f.outputs["list-panes -t $1:@9 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%9 \n"}
	f.outputs["split-window -f -h -b -d -l 50 -t $1:@9 -P -F #{pane_id} demux sidebar slot"] = fakeReply{out: "%55\n"}
	s := &Sticky{T: f, Slots: true}
	if err := s.Follow(); err != nil {
		t.Fatalf("Follow: %v", err)
	}
	// The slot is created. The follow-up FindSlotInWindow call uses the same
	// list-panes mock (unchanged), so it returns no slot and Follow returns
	// without swap. The important assertion is: a new slot was tagged.
	sawTag := false
	for _, r := range f.runs {
		if strings.Join(r, " ") == "set-option -p -t %55 @demux_slot 1" {
			sawTag = true
		}
	}
	if !sawTag {
		t.Errorf("expected new slot to be tagged in target window, got: %v", f.runs)
	}
}

func TestFollow_PokesRefreshAfterMove(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	f.outputs["list-panes -aF #{pane_id}"] = fakeReply{out: "%42\n"}
	f.outputs["display-message -p #{session_id}:#{window_id}"] = fakeReply{out: "$2:@9\n"}
	f.outputs["display-message -p -t %42 #{window_id}"] = fakeReply{out: "@3\n"}
	f.outputs["display-message -p -t %42 #{pane_width}"] = fakeReply{out: "48\n"}
	s := &Sticky{T: f}
	if err := s.Follow(); err != nil {
		t.Fatalf("Follow: %v", err)
	}
	saw := false
	for _, r := range f.runs {
		if strings.Join(r, " ") == "send-keys -t %42 F12" {
			saw = true
		}
	}
	if !saw {
		t.Errorf("expected send-keys -t %%42 F12 after move, got: %v", f.runs)
	}
}

func TestFollow_DeadTrackedPane_CleansSlotsAndEnv(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	// Tracked pane %42 is no longer in list-panes.
	f.outputs["list-panes -aF #{pane_id}"] = fakeReply{out: "%1\n%2\n"}
	// UninstallSlots will scan tags; placeholder slots remain in other windows.
	f.outputs["list-panes -aF #{pane_id} #{@demux_slot}"] = fakeReply{out: "%50 1\n%60 1\n"}
	s := &Sticky{T: f, Slots: true}
	if err := s.Follow(); err != nil {
		t.Fatalf("Follow: %v", err)
	}
	var unset, kill50, kill60 bool
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if j == "set-environment -gu DEMUX_STICKY_PANE__dev_ttys001" {
			unset = true
		}
		if j == "kill-pane -t %50" {
			kill50 = true
		}
		if j == "kill-pane -t %60" {
			kill60 = true
		}
	}
	if !unset {
		t.Errorf("expected env var unset on dead pane, got: %v", f.runs)
	}
	if !kill50 || !kill60 {
		t.Errorf("expected leftover slots %%50 and %%60 killed, got: %v", f.runs)
	}
}

func TestFollow_EjectsFocusWhenSidebarIsActive(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	f.outputs["list-panes -aF #{pane_id}"] = fakeReply{out: "%42\n"}
	f.outputs["display-message -p #{session_id}:#{window_id}"] = fakeReply{out: "$2:@9\n"}
	f.outputs["display-message -p -t %42 #{window_id}"] = fakeReply{out: "@3\n"}
	f.outputs["display-message -p -t %42 #{pane_width}"] = fakeReply{out: "48\n"}
	// After follow's move, the sidebar pane is the active pane → eject.
	f.outputs["display-message -p #{pane_id}"] = fakeReply{out: "%42\n"}
	s := &Sticky{T: f}
	if err := s.Follow(); err != nil {
		t.Fatalf("Follow: %v", err)
	}
	saw := false
	for _, r := range f.runs {
		if strings.Join(r, " ") == "select-pane -t :.+" {
			saw = true
		}
	}
	if !saw {
		t.Errorf("expected select-pane -t :.+ to eject focus, got: %v", f.runs)
	}
}

func TestFollow_DoesNotEjectFocusWhenSidebarNotActive(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	f.outputs["list-panes -aF #{pane_id}"] = fakeReply{out: "%42\n"}
	f.outputs["display-message -p #{session_id}:#{window_id}"] = fakeReply{out: "$2:@9\n"}
	f.outputs["display-message -p -t %42 #{window_id}"] = fakeReply{out: "@3\n"}
	f.outputs["display-message -p -t %42 #{pane_width}"] = fakeReply{out: "48\n"}
	// Active pane is NOT the sidebar.
	f.outputs["display-message -p #{pane_id}"] = fakeReply{out: "%99\n"}
	s := &Sticky{T: f}
	if err := s.Follow(); err != nil {
		t.Fatalf("Follow: %v", err)
	}
	for _, r := range f.runs {
		if strings.Join(r, " ") == "select-pane -t :.+" {
			t.Errorf("did not expect select-pane when sidebar isn't active, got: %v", f.runs)
		}
	}
}

func TestFollow_JoinFailure_DoesNotPropagate(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	f.outputs["list-panes -aF #{pane_id}"] = fakeReply{out: "%42\n"}
	f.outputs["display-message -p #{session_id}:#{window_id}"] = fakeReply{out: "$2:@9\n"}
	f.outputs["display-message -p -t %42 #{window_id}"] = fakeReply{out: "@3\n"}
	f.outputs["display-message -p -t %42 #{pane_width}"] = fakeReply{out: "48\n"}
	f.runErrs["join-pane -f -h -b -d -l 48 -s %42 -t $2:@9"] = stderrExitError("target session vanished")
	s := &Sticky{T: f}
	if err := s.Follow(); err != nil {
		t.Errorf("Follow should swallow join-pane failure, got: %v", err)
	}
}
