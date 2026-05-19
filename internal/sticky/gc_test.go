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

func TestClearStickyEnvForPane_TmuxError_ReturnsError(t *testing.T) {
	f := newFakeTmux()
	f.outputs["show-environment -g"] = fakeReply{err: stderrExitError("no server\n")}
	if err := ClearStickyEnvForPane(f, "%42"); err == nil {
		t.Error("expected error to surface")
	}
}
