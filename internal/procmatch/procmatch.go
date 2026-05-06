// Package procmatch matches processes inside tmux pane trees against
// user-configured glob patterns. It is pure: no TUI or tmux side effects,
// only data transformation.
package procmatch

import (
	"path/filepath"
	"strings"

	"github.com/rtalexk/demux/internal/proc"
	"github.com/rtalexk/demux/internal/tmux"
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

// Match returns at most one Label per session. For each pane:
//   - Level 0 (pane root) is matched first; the first hit (in declaration
//     order) wins and children are not inspected.
//   - Level 1 is inspected only when level 0 had no hit. The first child
//     proc whose name or cmdline matches a pattern wins.
//
// Across all matched panes for a session, the label with the highest
// occurrence count is selected. Ties are broken by earliest declaration
// order in patterns.
//
// Sessions with zero matched panes do not appear in the returned map.
func Match(panes []tmux.Pane, procs []proc.Process, patterns []Pattern) map[string]Label {
	if len(panes) == 0 || len(procs) == 0 || len(patterns) == 0 {
		return map[string]Label{}
	}

	byPID := make(map[int32]proc.Process, len(procs))
	childrenOf := make(map[int32][]proc.Process, len(procs))
	for _, p := range procs {
		byPID[p.PID] = p
		childrenOf[p.PPID] = append(childrenOf[p.PPID], p)
	}

	patternRank := make(map[string]int, len(patterns))
	for i, pat := range patterns {
		if _, seen := patternRank[pat.Label]; !seen {
			patternRank[pat.Label] = i
		}
	}

	type tally struct {
		label Label
		count int
	}
	tallies := make(map[string]map[string]*tally)

	addHit := func(session string, lbl Label) {
		bySession, ok := tallies[session]
		if !ok {
			bySession = make(map[string]*tally)
			tallies[session] = bySession
		}
		t, ok := bySession[lbl.Text]
		if !ok {
			t = &tally{label: lbl}
			bySession[lbl.Text] = t
		}
		t.count++
	}

	for _, pane := range panes {
		root, ok := byPID[pane.PanePID]
		if !ok {
			continue
		}
		if lbl, hit := MatchProcess(root, patterns); hit {
			addHit(pane.Session, lbl)
			continue
		}
		for _, child := range childrenOf[pane.PanePID] {
			if lbl, hit := MatchProcess(child, patterns); hit {
				addHit(pane.Session, lbl)
				break
			}
		}
	}

	out := make(map[string]Label, len(tallies))
	for sess, bySession := range tallies {
		var best *tally
		for _, t := range bySession {
			switch {
			case best == nil:
				best = t
			case t.count > best.count:
				best = t
			case t.count == best.count && patternRank[t.label.Text] < patternRank[best.label.Text]:
				best = t
			}
		}
		if best != nil {
			out[sess] = best.label
		}
	}
	return out
}
