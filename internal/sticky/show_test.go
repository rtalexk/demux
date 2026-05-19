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
