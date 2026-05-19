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

func TestHide_SlotsMode_UninstallsAllSlots(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	// UninstallSlots scans all panes; active (%42) and three other slots.
	f.outputs["list-panes -aF #{pane_id} #{@demux_slot}"] = fakeReply{out: "%1 \n%42 1\n%50 1\n%60 1\n%99 \n"}
	s := &Sticky{T: f, Slots: true}
	if err := s.Hide(); err != nil {
		t.Fatalf("Hide: %v", err)
	}
	wantKills := map[string]bool{
		"kill-pane -t %42": false,
		"kill-pane -t %50": false,
		"kill-pane -t %60": false,
	}
	var sawUnset bool
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if _, ok := wantKills[j]; ok {
			wantKills[j] = true
		}
		if strings.HasPrefix(j, "kill-pane -t %1") || strings.HasPrefix(j, "kill-pane -t %99") {
			t.Errorf("kill-pane on non-slot pane: %q", j)
		}
		if j == "set-environment -gu DEMUX_STICKY_PANE__dev_ttys001" {
			sawUnset = true
		}
	}
	for k, ok := range wantKills {
		if !ok {
			t.Errorf("missing expected kill-pane: %s", k)
		}
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
