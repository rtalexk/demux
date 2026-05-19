package sticky

import (
	"strings"
	"testing"
)

func TestHide_EnvUnset_Noop(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{err: stderrExitError("unknown variable\n")}
	s := &Sticky{T: f}
	if err := s.Hide(); err != nil {
		t.Fatalf("Hide: %v", err)
	}
	for _, r := range f.runs {
		joined := strings.Join(r, " ")
		if strings.HasPrefix(joined, "kill-pane") {
			t.Errorf("expected no kill-pane (noop), got: %v", r)
		}
		if strings.HasPrefix(joined, "set-environment") {
			t.Errorf("expected no set-environment (noop), got: %v", r)
		}
	}
}

func TestHide_EnvSet_KillsPaneAndUnsets(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	s := &Sticky{T: f}
	if err := s.Hide(); err != nil {
		t.Fatalf("Hide: %v", err)
	}
	var sawKill, sawUnset bool
	for _, r := range f.runs {
		joined := strings.Join(r, " ")
		if joined == "kill-pane -t %42" {
			sawKill = true
		}
		if joined == "set-environment -gu DEMUX_STICKY_PANE__dev_ttys001" {
			sawUnset = true
		}
	}
	if !sawKill {
		t.Errorf("expected kill-pane call, got runs: %v", f.runs)
	}
	if !sawUnset {
		t.Errorf("expected unset-env call, got runs: %v", f.runs)
	}
}

func TestHide_SlotsMode_RespawnsPlaceholder(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	s := &Sticky{T: f, Slots: true}
	if err := s.Hide(); err != nil {
		t.Fatalf("Hide: %v", err)
	}
	var sawRespawn, sawKill, sawUnset bool
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if j == "respawn-pane -k -t %42 demux sidebar slot" {
			sawRespawn = true
		}
		if strings.HasPrefix(j, "kill-pane") {
			sawKill = true
		}
		if j == "set-environment -gu DEMUX_STICKY_PANE__dev_ttys001" {
			sawUnset = true
		}
	}
	if !sawRespawn {
		t.Errorf("expected respawn-pane to placeholder, got: %v", f.runs)
	}
	if sawKill {
		t.Errorf("expected no kill-pane in slots mode, got: %v", f.runs)
	}
	if !sawUnset {
		t.Errorf("expected env unset, got: %v", f.runs)
	}
}

func TestHide_KillPaneFailure_StillUnsets(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	f.runErrs["kill-pane -t %42"] = stderrExitError("can't find pane: %42\n")
	s := &Sticky{T: f}
	if err := s.Hide(); err != nil {
		t.Fatalf("Hide should tolerate kill-pane failure, got: %v", err)
	}
	sawUnset := false
	for _, r := range f.runs {
		if strings.Join(r, " ") == "set-environment -gu DEMUX_STICKY_PANE__dev_ttys001" {
			sawUnset = true
		}
	}
	if !sawUnset {
		t.Errorf("expected env unset even when kill-pane failed, got runs: %v", f.runs)
	}
}
