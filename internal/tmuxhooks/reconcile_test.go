package tmuxhooks

import (
	"strings"
	"testing"
)

func TestParseShowHooks(t *testing.T) {
	out := `after-new-window[0] run-shell -b "demux sidebar follow"
after-new-window[1] run-shell "user thing"
after-select-pane[0] run-shell 'demux event pane_focus'
garbage line with no bracket
`
	got := parseShowHooks(out)
	if len(got["after-new-window"]) != 2 {
		t.Fatalf("after-new-window: want 2 entries, got %v", got["after-new-window"])
	}
	if got["after-new-window"][0] != `run-shell -b "demux sidebar follow"` {
		t.Errorf("entry 0 mismatch: %q", got["after-new-window"][0])
	}
	if got["after-new-window"][1] != `run-shell "user thing"` {
		t.Errorf("entry 1 mismatch: %q", got["after-new-window"][1])
	}
	if len(got["after-select-pane"]) != 1 {
		t.Errorf("after-select-pane: want 1, got %v", got["after-select-pane"])
	}
}

func TestIsDemuxEntry(t *testing.T) {
	cases := map[string]bool{
		`run-shell 'demux event pane_focus'`:  true,
		`run-shell -b 'demux sidebar follow'`: true,
		`run-shell 'tmux_claude_layout open'`: false,
		`run-shell 'echo demuxing'`:           false,
	}
	for in, want := range cases {
		if got := isDemuxEntry(in); got != want {
			t.Errorf("isDemuxEntry(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestReconcileDedupesAndPreservesUserHooks(t *testing.T) {
	f := newFakeTmux()
	// after-new-window has 3 stale demux copies + 1 user hook.
	f.outputs["show-hooks -g"] = strings.Join([]string{
		`after-new-window[0] run-shell -b "demux sidebar follow"`,
		`after-new-window[1] run-shell -b "demux sidebar follow"`,
		`after-new-window[2] run-shell "my own hook"`,
		`after-new-window[3] run-shell -b "demux sidebar slots ensure"`,
	}, "\n")

	if err := Reconcile(f); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	runs := f.runStrings()
	// after-new-window must be cleared exactly once.
	clears := 0
	for _, r := range runs {
		if r == "set-hook -gu after-new-window" {
			clears++
		}
	}
	if clears != 1 {
		t.Errorf("want 1 clear of after-new-window, got %d in %v", clears, runs)
	}
	// The user hook must be re-added before demux's canonical entry.
	userIdx, demuxIdx := -1, -1
	for i, r := range runs {
		if strings.Contains(r, `set-hook -ga after-new-window run-shell "my own hook"`) {
			userIdx = i
		}
		if strings.Contains(r, "set-hook -ga after-new-window run-shell -b 'demux sidebar follow 2>/dev/null; demux sidebar slots ensure") {
			demuxIdx = i
		}
	}
	if userIdx < 0 || demuxIdx < 0 {
		t.Fatalf("missing re-add calls: user=%d demux=%d runs=%v", userIdx, demuxIdx, runs)
	}
	if userIdx > demuxIdx {
		t.Errorf("user hook re-added after demux hook: user=%d demux=%d", userIdx, demuxIdx)
	}
}

func TestSyncFastPathSkipsWhenHashMatches(t *testing.T) {
	f := newFakeTmux()
	f.outputs["show -gv @demux_hooks_version"] = HooksHash()
	if err := Sync(f); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	for _, r := range f.runStrings() {
		if strings.HasPrefix(r, "set-hook") {
			t.Errorf("fast path must not touch hooks, got run %q", r)
		}
	}
}

func TestSyncReconcilesWhenHashStale(t *testing.T) {
	f := newFakeTmux()
	f.outputs["show -gv @demux_hooks_version"] = "stale000000"
	f.outputs["show-hooks -g"] = ""
	if err := Sync(f); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	wrote := false
	for _, r := range f.runStrings() {
		if r == "set -g @demux_hooks_version "+HooksHash() {
			wrote = true
		}
	}
	if !wrote {
		t.Errorf("Sync must record the new hash; runs=%v", f.runStrings())
	}
}
