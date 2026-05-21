package tmuxhooks

import "testing"

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
