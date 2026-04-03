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
	}
	for _, c := range cases {
		got := tmux.PrimaryPaneCWD(c.panes)
		if got != c.want {
			t.Errorf("%s: PrimaryPaneCWD = %q, want %q", c.name, got, c.want)
		}
	}
}
