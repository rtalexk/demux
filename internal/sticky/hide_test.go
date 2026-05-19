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

func TestHide_SlotsMode_KillsActiveFirstThenSweeps(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	// After active is killed, sweep finds only the remaining placeholders.
	f.outputs["list-panes -aF #{pane_id} #{@demux_slot}"] = fakeReply{out: "%1 \n%50 1\n%60 1\n%99 \n"}
	s := &Sticky{T: f, Slots: true}
	if err := s.Hide(); err != nil {
		t.Fatalf("Hide: %v", err)
	}
	// Find first kill-pane call: must target the active pane (%42).
	var killOrder []string
	var sawUnset bool
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if strings.HasPrefix(j, "kill-pane -t ") {
			killOrder = append(killOrder, strings.TrimPrefix(j, "kill-pane -t "))
		}
		if j == "set-environment -gu DEMUX_STICKY_PANE__dev_ttys001" {
			sawUnset = true
		}
	}
	if len(killOrder) == 0 || killOrder[0] != "%42" {
		t.Errorf("first kill must be active pane %%42, got order: %v", killOrder)
	}
	want := map[string]bool{"%42": false, "%50": false, "%60": false}
	for _, p := range killOrder {
		if _, ok := want[p]; !ok {
			t.Errorf("unexpected kill target: %q", p)
		}
		want[p] = true
	}
	for k, ok := range want {
		if !ok {
			t.Errorf("missing kill-pane for %s", k)
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
