package sticky

import (
	"errors"
	"strings"
	"testing"
)

func TestToggle_EnvSetAlive_CallsHide(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	f.outputs["list-panes -aF #{pane_id}"] = fakeReply{out: "%42\n"}
	s := &Sticky{T: f}
	if err := s.Toggle(ShowOpts{Width: 35, Cmd: "demux --sticky"}); err != nil {
		t.Fatalf("Toggle: %v", err)
	}
	sawKill := false
	for _, r := range f.runs {
		if strings.Join(r, " ") == "kill-pane -t %42" {
			sawKill = true
		}
	}
	if !sawKill {
		t.Errorf("expected Toggle to call kill-pane (Hide), got: %v", f.runs)
	}
}

func TestToggle_EnvUnset_CallsShow(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{err: stderrExitError("unknown variable\n")}
	f.outputs["-V"] = fakeReply{out: "tmux 3.3\n"}
	f.outputs["display-message -p #{session_id}:#{window_id}"] = fakeReply{out: "$1:@7\n"}
	f.outputs["split-window -f -h -b -d -l 35 -t $1:@7 -P -F #{pane_id} demux --sticky"] = fakeReply{out: "%88\n"}
	s := &Sticky{T: f}
	if err := s.Toggle(ShowOpts{Width: 35, Cmd: "demux --sticky"}); err != nil {
		t.Fatalf("Toggle: %v", err)
	}
	sawSplit := false
	for _, r := range f.runs {
		if strings.HasPrefix(strings.Join(r, " "), "set-environment -g DEMUX_STICKY_PANE__dev_ttys001 %88") {
			sawSplit = true
		}
	}
	if !sawSplit {
		t.Errorf("expected Toggle to call Show (recording new pane id), got: %v", f.runs)
	}
}

func TestToggle_EnvSetButPaneDead_CallsShow(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	f.outputs["list-panes -aF #{pane_id}"] = fakeReply{out: "%1\n%3\n"}
	f.outputs["-V"] = fakeReply{out: "tmux 3.3\n"}
	f.outputs["display-message -p #{session_id}:#{window_id}"] = fakeReply{out: "$1:@7\n"}
	f.outputs["split-window -f -h -b -d -l 35 -t $1:@7 -P -F #{pane_id} demux --sticky"] = fakeReply{out: "%88\n"}
	s := &Sticky{T: f}
	if err := s.Toggle(ShowOpts{Width: 35, Cmd: "demux --sticky"}); err != nil {
		t.Fatalf("Toggle: %v", err)
	}
}

func TestToggle_FocusBeforeClose_Unfocused_Focuses(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	f.outputs["list-panes -aF #{pane_id}"] = fakeReply{out: "%42\n"}
	f.outputs["display-message -p #{pane_id}"] = fakeReply{out: "%7\n"}
	s := &Sticky{T: f, FocusBeforeToggleClose: true}
	if err := s.Toggle(ShowOpts{Width: 35, Cmd: "demux --sticky"}); err != nil {
		t.Fatalf("Toggle: %v", err)
	}
	var sawFocus, sawKill bool
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if j == "select-pane -t %42" {
			sawFocus = true
		}
		if strings.HasPrefix(j, "kill-pane") {
			sawKill = true
		}
	}
	if !sawFocus {
		t.Errorf("expected select-pane -t %%42 (focus sidebar), got: %v", f.runs)
	}
	if sawKill {
		t.Errorf("expected no kill-pane when focusing instead of closing, got: %v", f.runs)
	}
}

func TestToggle_FocusBeforeClose_Focused_Closes(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	f.outputs["list-panes -aF #{pane_id}"] = fakeReply{out: "%42\n"}
	f.outputs["display-message -p #{pane_id}"] = fakeReply{out: "%42\n"}
	s := &Sticky{T: f, FocusBeforeToggleClose: true}
	if err := s.Toggle(ShowOpts{Width: 35, Cmd: "demux --sticky"}); err != nil {
		t.Fatalf("Toggle: %v", err)
	}
	sawKill := false
	for _, r := range f.runs {
		if strings.Join(r, " ") == "kill-pane -t %42" {
			sawKill = true
		}
	}
	if !sawKill {
		t.Errorf("expected kill-pane (close) when sidebar is focused, got: %v", f.runs)
	}
}

func TestToggle_FocusBeforeClose_ActivePaneError_Closes(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	f.outputs["list-panes -aF #{pane_id}"] = fakeReply{out: "%42\n"}
	f.outputs["display-message -p #{pane_id}"] = fakeReply{err: errors.New("no server")}
	s := &Sticky{T: f, FocusBeforeToggleClose: true}
	if err := s.Toggle(ShowOpts{Width: 35, Cmd: "demux --sticky"}); err != nil {
		t.Fatalf("Toggle: %v", err)
	}
	sawKill := false
	for _, r := range f.runs {
		if strings.Join(r, " ") == "kill-pane -t %42" {
			sawKill = true
		}
	}
	if !sawKill {
		t.Errorf("expected fall-through to kill-pane on activePane error, got: %v", f.runs)
	}
}
