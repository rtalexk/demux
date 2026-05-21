package tmux_test

import (
	"testing"

	"github.com/rtalexk/demux/internal/tmux"
)

func TestPrimaryPaneCWD(t *testing.T) {
	cases := []struct {
		name  string
		panes []tmux.Pane
		want  string
	}{
		{"empty", nil, ""},
		{"pane 0 first", []tmux.Pane{{PaneIndex: 0, CWD: "/a"}, {PaneIndex: 1, CWD: "/b"}}, "/a"},
		{"no pane 0 fallback", []tmux.Pane{{PaneIndex: 1, CWD: "/b"}}, "/b"},
		{"slot at index 0 skipped", []tmux.Pane{{PaneIndex: 0, CWD: "/sidebar", IsSlot: true}, {PaneIndex: 1, CWD: "/real"}}, "/real"},
		{"slot not at index 0 ignored", []tmux.Pane{{PaneIndex: 0, CWD: "/real"}, {PaneIndex: 1, CWD: "/sidebar", IsSlot: true}}, "/real"},
		{"only slot pane", []tmux.Pane{{PaneIndex: 0, CWD: "/sidebar", IsSlot: true}}, ""},
		{"slot at index 0, fallback skips it", []tmux.Pane{{PaneIndex: 0, CWD: "/sidebar", IsSlot: true}, {PaneIndex: 2, CWD: "/real"}}, "/real"},
	}
	for _, c := range cases {
		got := tmux.PrimaryPaneCWD(c.panes)
		if got != c.want {
			t.Errorf("%s: PrimaryPaneCWD = %q, want %q", c.name, got, c.want)
		}
	}
}
