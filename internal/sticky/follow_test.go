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

func TestFollow_PaneAlreadyInCurrentSession_Noop(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	f.outputs["list-panes -aF #{pane_id}"] = fakeReply{out: "%42\n"}
	f.outputs["display-message -p #{session_id}:#{window_id}"] = fakeReply{out: "$1:@7\n"}
	f.outputs["display-message -p -t %42 #{session_id}"] = fakeReply{out: "$1\n"}
	s := &Sticky{T: f}
	if err := s.Follow(); err != nil {
		t.Fatalf("Follow: %v", err)
	}
	for _, r := range f.runs {
		if strings.HasPrefix(strings.Join(r, " "), "join-pane") {
			t.Errorf("expected no join-pane (same session), got: %v", r)
		}
	}
}

func TestFollow_DifferentSession_JoinsWithFFlagAndPreservedWidth(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	f.outputs["list-panes -aF #{pane_id}"] = fakeReply{out: "%42\n"}
	f.outputs["display-message -p #{session_id}:#{window_id}"] = fakeReply{out: "$2:@9\n"}
	f.outputs["display-message -p -t %42 #{session_id}"] = fakeReply{out: "$1\n"}
	f.outputs["display-message -p -t %42 #{pane_width}"] = fakeReply{out: "48\n"}
	s := &Sticky{T: f}
	if err := s.Follow(); err != nil {
		t.Fatalf("Follow: %v", err)
	}
	want := "join-pane -f -h -b -l 48 -s %42 -t $2:@9"
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
	f.outputs["display-message -p -t %42 #{session_id}"] = fakeReply{out: "$1\n"}
	f.outputs["display-message -p -t %42 #{pane_width}"] = fakeReply{out: "0\n"}
	s := &Sticky{T: f}
	if err := s.Follow(); err != nil {
		t.Fatalf("Follow: %v", err)
	}
	want := "join-pane -f -h -b -l 35 -s %42 -t $2:@9"
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

func TestFollow_JoinFailure_DoesNotPropagate(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	f.outputs["list-panes -aF #{pane_id}"] = fakeReply{out: "%42\n"}
	f.outputs["display-message -p #{session_id}:#{window_id}"] = fakeReply{out: "$2:@9\n"}
	f.outputs["display-message -p -t %42 #{session_id}"] = fakeReply{out: "$1\n"}
	f.outputs["display-message -p -t %42 #{pane_width}"] = fakeReply{out: "48\n"}
	f.runErrs["join-pane -f -h -b -l 48 -s %42 -t $2:@9"] = stderrExitError("target session vanished")
	s := &Sticky{T: f}
	if err := s.Follow(); err != nil {
		t.Errorf("Follow should swallow join-pane failure, got: %v", err)
	}
}
