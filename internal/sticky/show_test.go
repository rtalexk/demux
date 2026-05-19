package sticky

import (
	"errors"
	"strings"
	"testing"
)

func TestShow_EnvUnset_CreatesPane(t *testing.T) {
	f := newFakeTmux()
	f.outputs["-V"] = fakeReply{out: "tmux 3.3\n"}
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{err: stderrExitError("unknown variable: DEMUX_STICKY_PANE__dev_ttys001\n")}
	f.outputs["display-message -p #{session_id}:#{window_id}"] = fakeReply{out: "$1:@7\n"}
	f.outputs["split-window -f -h -b -l 35 -t $1:@7 -P -F #{pane_id} demux --sticky"] = fakeReply{out: "%88\n"}
	s := &Sticky{T: f}
	if err := s.Show(ShowOpts{Width: 35, Cmd: "demux --sticky"}); err != nil {
		t.Fatalf("Show: %v", err)
	}
	found := false
	for _, r := range f.runs {
		if strings.Join(r, " ") == "set-environment -g DEMUX_STICKY_PANE__dev_ttys001 %88" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected set-environment call recording pane id, got runs: %v", f.runs)
	}
}

func TestShow_EnvSetAndPaneAlive_Noop(t *testing.T) {
	f := newFakeTmux()
	f.outputs["-V"] = fakeReply{out: "tmux 3.3\n"}
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	f.outputs["list-panes -aF #{pane_id}"] = fakeReply{out: "%1\n%42\n"}
	s := &Sticky{T: f}
	if err := s.Show(ShowOpts{Width: 35, Cmd: "demux --sticky"}); err != nil {
		t.Fatalf("Show: %v", err)
	}
	for _, r := range f.runs {
		if strings.HasPrefix(strings.Join(r, " "), "set-environment") {
			t.Errorf("expected no set-environment call (noop), got: %v", r)
		}
		if strings.HasPrefix(strings.Join(r, " "), "split-window") {
			t.Errorf("expected no split-window call (noop), got: %v", r)
		}
	}
}

func TestShow_EnvSetButPaneDead_Recreates(t *testing.T) {
	f := newFakeTmux()
	f.outputs["-V"] = fakeReply{out: "tmux 3.3\n"}
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	f.outputs["list-panes -aF #{pane_id}"] = fakeReply{out: "%1\n%2\n"}
	f.outputs["display-message -p #{session_id}:#{window_id}"] = fakeReply{out: "$2:@9\n"}
	f.outputs["split-window -f -h -b -l 35 -t $2:@9 -P -F #{pane_id} demux --sticky"] = fakeReply{out: "%99\n"}
	s := &Sticky{T: f}
	if err := s.Show(ShowOpts{Width: 35, Cmd: "demux --sticky"}); err != nil {
		t.Fatalf("Show: %v", err)
	}
	found := false
	for _, r := range f.runs {
		if strings.Join(r, " ") == "set-environment -g DEMUX_STICKY_PANE__dev_ttys001 %99" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected env var rewrite to new pane id, got runs: %v", f.runs)
	}
}

func TestShow_OldTmux_Errors(t *testing.T) {
	f := newFakeTmux()
	f.outputs["-V"] = fakeReply{out: "tmux 2.4\n"}
	s := &Sticky{T: f}
	err := s.Show(ShowOpts{Width: 35, Cmd: "demux --sticky"})
	if err == nil {
		t.Fatal("expected error on tmux < 2.6")
	}
	if !strings.Contains(err.Error(), "too old") {
		t.Errorf("expected version error, got: %v", err)
	}
}

func TestShow_SlotsMode_ActivatesCurrentSlotBeforeOtherWindows(t *testing.T) {
	f := newFakeTmux()
	f.outputs["-V"] = fakeReply{out: "tmux 3.3\n"}
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{err: stderrExitError("unknown variable\n")}
	f.outputs["display-message -p #{session_id}:#{window_id}"] = fakeReply{out: "$1:@7\n"}
	// Current window has no slot. After split + tag, list-panes returns it tagged.
	f.outputs["list-panes -t $1:@7 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%1 \n%10 1\n"}
	f.outputs["split-window -f -h -b -d -l 35 -t $1:@7 -P -F #{pane_id} demux sidebar slot"] = fakeReply{out: "%10\n"}
	// Reservation queries.
	f.outputs["list-windows -aF #{window_id}"] = fakeReply{out: "@7\n@8\n"}
	f.outputs["list-windows -t $1 -F #{window_id} #{window_last_flag}"] = fakeReply{out: "@7 0\n@8 1\n"}
	f.outputs["display-message -p #{client_last_session}"] = fakeReply{out: "\n"}
	f.outputs["show-environment -g "+MRUEnvKey] = fakeReply{err: stderrExitError("unknown variable\n")}
	// Reconcile: list slot panes - none yet for @8.
	f.outputs["list-panes -aF #{window_id} #{pane_id} #{@demux_slot}"] = fakeReply{out: "@7 %10 1\n"}
	// EnsureSlotInWindow for @8: list-panes shows no slot, split installs %11.
	f.outputs["list-panes -t @8 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%2 \n"}
	f.outputs["split-window -f -h -b -d -l 35 -t @8 -P -F #{pane_id} demux sidebar slot"] = fakeReply{out: "%11\n"}
	s := &Sticky{T: f, Slots: true}
	if err := s.Show(ShowOpts{Width: 35, Cmd: "demux --sticky"}); err != nil {
		t.Fatalf("Show: %v", err)
	}
	// Verify respawn-pane (activate current) and env-var set happen BEFORE
	// the @8 slot tag (which only runs during the later InstallSlotsInAllWindows).
	idx := func(target string) int {
		for i, r := range f.runs {
			if strings.Join(r, " ") == target {
				return i
			}
		}
		return -1
	}
	iRespawn := idx("respawn-pane -k -t %10 demux --sticky")
	iRecord := idx("set-environment -g DEMUX_STICKY_PANE__dev_ttys001 %10")
	iOtherTag := idx("set-option -p -t %11 @demux_slot 1")
	if iRespawn < 0 {
		t.Fatalf("expected respawn-pane on current slot, got: %v", f.runs)
	}
	if iRecord < 0 {
		t.Fatalf("expected env-var record, got: %v", f.runs)
	}
	if iOtherTag < 0 {
		t.Fatalf("expected slot tag on other window's new pane, got: %v", f.runs)
	}
	if !(iRespawn < iRecord && iRecord < iOtherTag) {
		t.Errorf("expected order respawn < record < other-window-tag, got %d/%d/%d", iRespawn, iRecord, iOtherTag)
	}
}

func TestShow_SplitFailure_Surfaces(t *testing.T) {
	f := newFakeTmux()
	f.outputs["-V"] = fakeReply{out: "tmux 3.3\n"}
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{err: stderrExitError("unknown variable\n")}
	f.outputs["display-message -p #{session_id}:#{window_id}"] = fakeReply{out: "$1:@7\n"}
	f.outputs["split-window -f -h -b -l 35 -t $1:@7 -P -F #{pane_id} demux --sticky"] = fakeReply{err: errors.New("can't split")}
	s := &Sticky{T: f}
	if err := s.Show(ShowOpts{Width: 35, Cmd: "demux --sticky"}); err == nil {
		t.Error("expected split-window failure to surface")
	}
}
