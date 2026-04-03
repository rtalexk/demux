package git_test

import (
	"testing"

	"github.com/rtalexk/demux/internal/git"
)

func TestIndicators(t *testing.T) {
	cases := []struct {
		info git.Info
		want string
	}{
		{git.Info{}, ""},
		{git.Info{Ahead: 2}, "↑2"},
		{git.Info{Behind: 1}, "↓1"},
		{git.Info{Dirty: true}, "*"},
		{git.Info{Ahead: 1, Behind: 2, Dirty: true}, "↑1 ↓2 *"},
	}
	for _, c := range cases {
		got := git.Indicators(c.info)
		if got != c.want {
			t.Errorf("Indicators(%+v) = %q, want %q", c.info, got, c.want)
		}
	}
}
