// Package procmatch matches processes inside tmux pane trees against
// user-configured glob patterns. It is pure: no TUI or tmux side effects,
// only data transformation.
package procmatch

import (
	"path/filepath"
	"strings"

	"github.com/rtalexk/demux/internal/proc"
)

type Pattern struct {
	Match string
	Label string
	FG    string
	BG    string
}

type Label struct {
	Text string
	FG   string
	BG   string
}

// MatchProcess returns the Label of the first pattern whose lowered glob
// matches either the lowered FriendlyName, any whitespace-separated token of
// the lowered Cmdline, or the basename of such a token. Returns (Label{},
// false) if no pattern matches. Patterns whose Match is not a valid glob are
// skipped silently.
func MatchProcess(p proc.Process, patterns []Pattern) (Label, bool) {
	name := strings.ToLower(p.FriendlyName())
	fields := strings.Fields(strings.ToLower(p.Cmdline))
	for _, pat := range patterns {
		if pat.Match == "" || pat.Label == "" {
			continue
		}
		glob := strings.ToLower(pat.Match)
		if _, err := filepath.Match(glob, ""); err != nil {
			continue
		}
		if hit, _ := filepath.Match(glob, name); hit {
			return Label{Text: pat.Label, FG: pat.FG, BG: pat.BG}, true
		}
		for _, tok := range fields {
			if hit, _ := filepath.Match(glob, tok); hit {
				return Label{Text: pat.Label, FG: pat.FG, BG: pat.BG}, true
			}
			if base := filepath.Base(tok); base != tok {
				if hit, _ := filepath.Match(glob, base); hit {
					return Label{Text: pat.Label, FG: pat.FG, BG: pat.BG}, true
				}
			}
		}
	}
	return Label{}, false
}
