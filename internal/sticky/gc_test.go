package sticky

import (
	"strings"
	"testing"
)

func TestClearStickyEnvForPane_MatchUnsets(t *testing.T) {
	f := newFakeTmux()
	f.outputs["show-environment -g"] = fakeReply{out: strings.Join([]string{
		"SOMETHING_ELSE=value",
		"DEMUX_STICKY_PANE__dev_ttys001=%42",
		"DEMUX_STICKY_PANE__dev_ttys003=%99",
		"",
	}, "\n")}
	if err := ClearStickyEnvForPane(f, "%42"); err != nil {
		t.Fatalf("ClearStickyEnvForPane: %v", err)
	}
	sawUnset := false
	for _, r := range f.runs {
		if strings.Join(r, " ") == "set-environment -gu DEMUX_STICKY_PANE__dev_ttys001" {
			sawUnset = true
		}
		if strings.Join(r, " ") == "set-environment -gu DEMUX_STICKY_PANE__dev_ttys003" {
			t.Errorf("unexpected unset of non-matching key: %v", r)
		}
	}
	if !sawUnset {
		t.Errorf("expected unset of DEMUX_STICKY_PANE__dev_ttys001, got: %v", f.runs)
	}
}

func TestClearStickyEnvForPane_NoMatch_NoUnset(t *testing.T) {
	f := newFakeTmux()
	f.outputs["show-environment -g"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%99\n"}
	if err := ClearStickyEnvForPane(f, "%42"); err != nil {
		t.Fatalf("ClearStickyEnvForPane: %v", err)
	}
	for _, r := range f.runs {
		if strings.HasPrefix(strings.Join(r, " "), "set-environment") {
			t.Errorf("expected no unset, got: %v", r)
		}
	}
}

func TestClearStickyEnvForPane_MatchAlsoUninstallsRemainingSlots(t *testing.T) {
	f := newFakeTmux()
	f.outputs["show-environment -g"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n"}
	// UninstallSlots will run list-panes -aF; placeholder slots remain.
	f.outputs["list-panes -aF #{pane_id} #{@demux_slot}"] = fakeReply{out: "%50 1\n%60 1\n%99 \n"}
	if err := ClearStickyEnvForPane(f, "%42"); err != nil {
		t.Fatalf("ClearStickyEnvForPane: %v", err)
	}
	var unset, kill50, kill60, killNonSlot bool
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
		if j == "kill-pane -t %99" {
			killNonSlot = true
		}
	}
	if !unset {
		t.Errorf("expected env unset, got: %v", f.runs)
	}
	if !kill50 || !kill60 {
		t.Errorf("expected slot panes %%50 and %%60 killed, got: %v", f.runs)
	}
	if killNonSlot {
		t.Errorf("did not expect non-slot pane %%99 to be killed, got: %v", f.runs)
	}
}

func TestClearStickyEnvForPane_NoMatch_NoUninstall(t *testing.T) {
	f := newFakeTmux()
	f.outputs["show-environment -g"] = fakeReply{out: "DEMUX_STICKY_PANE__dev_ttys001=%99\n"}
	if err := ClearStickyEnvForPane(f, "%42"); err != nil {
		t.Fatalf("ClearStickyEnvForPane: %v", err)
	}
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if strings.HasPrefix(j, "kill-pane") {
			t.Errorf("expected no kill-pane when no env match, got: %v", r)
		}
	}
}

func TestClearStickyEnvForPane_TmuxError_ReturnsError(t *testing.T) {
	f := newFakeTmux()
	f.outputs["show-environment -g"] = fakeReply{err: stderrExitError("no server\n")}
	if err := ClearStickyEnvForPane(f, "%42"); err == nil {
		t.Error("expected error to surface")
	}
}
