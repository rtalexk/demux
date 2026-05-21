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

func TestPrimaryPane(t *testing.T) {
	cases := []struct {
		name  string
		panes []tmux.Pane
		want  string // CWD of the expected pane, or "<nil>" when none
	}{
		{"empty", nil, "<nil>"},
		{"pane 0", []tmux.Pane{{PaneIndex: 0, CWD: "/a"}, {PaneIndex: 1, CWD: "/b"}}, "/a"},
		{"slot at index 0 skipped", []tmux.Pane{{PaneIndex: 0, CWD: "/sidebar", IsSlot: true}, {PaneIndex: 1, CWD: "/real"}}, "/real"},
		{"only slot pane", []tmux.Pane{{PaneIndex: 0, CWD: "/sidebar", IsSlot: true}}, "<nil>"},
	}
	for _, c := range cases {
		got := tmux.PrimaryPane(c.panes)
		if c.want == "<nil>" {
			if got != nil {
				t.Errorf("%s: expected nil, got %+v", c.name, *got)
			}
			continue
		}
		if got == nil {
			t.Errorf("%s: expected pane with CWD %q, got nil", c.name, c.want)
			continue
		}
		if got.CWD != c.want {
			t.Errorf("%s: PrimaryPane CWD = %q, want %q", c.name, got.CWD, c.want)
		}
	}
}
