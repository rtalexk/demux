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

// normalizedPattern is a Pattern with the glob lower-cased once and
// validated; invalid/empty entries are dropped during normalization.
type normalizedPattern struct {
	glob  string
	label Label
}

func normalize(patterns []Pattern) []normalizedPattern {
	out := make([]normalizedPattern, 0, len(patterns))
	for _, pat := range patterns {
		if pat.Match == "" || pat.Label == "" {
			continue
		}
		glob := strings.ToLower(pat.Match)
		if _, err := filepath.Match(glob, ""); err != nil {
			continue
		}
		out = append(out, normalizedPattern{
			glob:  glob,
			label: Label{Text: pat.Label, FG: pat.FG, BG: pat.BG},
		})
	}
	return out
}

func matchGlob(glob, candidate string) bool {
	hit, _ := filepath.Match(glob, candidate)
	return hit
}

// matchNormalized scans pre-normalized patterns against an already-lowered
// name and lowered cmdline tokens. Returns the first pattern's label.
func matchNormalized(name string, fields []string, patterns []normalizedPattern) (Label, bool) {
	for _, pat := range patterns {
		if matchGlob(pat.glob, name) {
			return pat.label, true
		}
		for _, tok := range fields {
			if matchGlob(pat.glob, tok) {
				return pat.label, true
			}
			if base := filepath.Base(tok); base != tok && matchGlob(pat.glob, base) {
				return pat.label, true
			}
		}
	}
	return Label{}, false
}

// MatchProcess returns the Label of the first pattern whose lowered glob
// matches either the lowered FriendlyName, any whitespace-separated token of
// the lowered Cmdline, or the basename of such a token. Returns (Label{},
// false) if no pattern matches. Patterns whose Match is not a valid glob are
// skipped silently.
func MatchProcess(p proc.Process, patterns []Pattern) (Label, bool) {
	return matchNormalized(
		strings.ToLower(p.FriendlyName()),
		strings.Fields(strings.ToLower(p.Cmdline)),
		normalize(patterns),
	)
}

// Match returns at most one Label per session. For each pane:
//   - The effective level 0 is the pane root proc, descending past any proc
//     whose FriendlyName is in ignored (case-insensitive). This mirrors the
//     shell-promotion done by the proc list display, so a pane whose raw root
//     is zsh -> make is treated as if make were the root.
//   - Effective level 0 is matched first; the first hit (in declaration
//     order) wins and children are not inspected.
//   - Effective level 1 is inspected only when level 0 had no hit. Children
//     of an ignored proc are flattened up to the same effective depth.
//
// Across all matched panes for a session, the label with the highest
// occurrence count is selected. Ties are broken by earliest declaration
// order in patterns.
//
// Sessions with zero matched panes do not appear in the returned map.
func Match(panes []tmux.Pane, procs []proc.Process, patterns []Pattern, ignored []string) map[string]Label {
	if len(panes) == 0 || len(procs) == 0 || len(patterns) == 0 {
		return map[string]Label{}
	}

	normalized := normalize(patterns)
	if len(normalized) == 0 {
		return map[string]Label{}
	}

	byPID := make(map[int32]proc.Process, len(procs))
	childrenOf := make(map[int32][]proc.Process, len(procs))
	for _, p := range procs {
		byPID[p.PID] = p
		childrenOf[p.PPID] = append(childrenOf[p.PPID], p)
	}

	ignoredSet := make(map[string]struct{}, len(ignored))
	for _, n := range ignored {
		ignoredSet[strings.ToLower(n)] = struct{}{}
	}
	isIgnored := func(p proc.Process) bool {
		_, ok := ignoredSet[strings.ToLower(p.FriendlyName())]
		return ok
	}

	// effectiveChildren flattens any direct child whose FriendlyName is in
	// ignored, replacing it with its own direct children (recursively). The
	// returned slice never contains an ignored proc.
	var effectiveChildren func(pid int32) []proc.Process
	effectiveChildren = func(pid int32) []proc.Process {
		var out []proc.Process
		for _, c := range childrenOf[pid] {
			if isIgnored(c) {
				out = append(out, effectiveChildren(c.PID)...)
				continue
			}
			out = append(out, c)
		}
		return out
	}

	// effectiveRoots returns the pane's effective level-0 procs: either the
	// raw root (if not ignored), or its flattened descendants past every
	// ignored ancestor.
	effectiveRoots := func(panePID int32) []proc.Process {
		root, ok := byPID[panePID]
		if !ok {
			return nil
		}
		if !isIgnored(root) {
			return []proc.Process{root}
		}
		return effectiveChildren(root.PID)
	}

	matchProc := func(p proc.Process) (Label, bool) {
		return matchNormalized(
			strings.ToLower(p.FriendlyName()),
			strings.Fields(strings.ToLower(p.Cmdline)),
			normalized,
		)
	}
	firstHit := func(procs []proc.Process) (Label, bool) {
		for _, p := range procs {
			if lbl, ok := matchProc(p); ok {
				return lbl, true
			}
		}
		return Label{}, false
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
		roots := effectiveRoots(pane.PanePID)
		if len(roots) == 0 {
			continue
		}
		if lbl, ok := firstHit(roots); ok {
			addHit(pane.Session, lbl)
			continue
		}
		for _, r := range roots {
			if lbl, ok := firstHit(effectiveChildren(r.PID)); ok {
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
