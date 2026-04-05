package query

import (
	"github.com/rtalexk/demux/internal/proc"
	"github.com/rtalexk/demux/internal/tmux"
	"github.com/sahilm/fuzzy"
)

type QueryScope int

const (
	ScopeSession QueryScope = iota
	ScopeWindow
	ScopeProcess
)

type ParsedQuery struct {
	Raw           string
	Scope         QueryScope
	Term          string
	ExtraSessions []string // non-live sessions to include in ScopeSession matching
}

type Result struct {
	Sessions []SessionMatch
}

type SessionMatch struct {
	Name     string
	Score    int
	MatchPos []int
	Windows  []WindowMatch
	Procs    []ProcMatch
}

type WindowMatch struct {
	Index    int
	Name     string
	Score    int
	MatchPos []int
}

type ProcMatch struct {
	PID      int32
	Name     string
	Score    int
	MatchPos []int
}

// Parse extracts scope and term from a raw query string.
// A scope tag (w:, p:) must appear at the start; mid-string tags are ignored.
// Bare text defaults to ScopeSession.
func Parse(raw string) ParsedQuery {
	pq := ParsedQuery{Raw: raw}
	switch {
	case len(raw) >= 2 && raw[:2] == "w:":
		pq.Scope = ScopeWindow
		pq.Term = raw[2:]
	case len(raw) >= 2 && raw[:2] == "p:":
		pq.Scope = ScopeProcess
		pq.Term = raw[2:]
	default:
		pq.Scope = ScopeSession
		pq.Term = raw
	}
	return pq
}

func maxScore(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func ensureSession(acc map[string]*SessionMatch, name string) *SessionMatch {
	if acc[name] == nil {
		acc[name] = &SessionMatch{Name: name}
	}
	return acc[name]
}

// matchSessionNames fuzzy-matches the query term against live and extra session names.
func matchSessionNames(pq ParsedQuery, sessions map[string]map[int][]tmux.Pane, acc map[string]*SessionMatch) {
	names := make([]string, 0, len(sessions)+len(pq.ExtraSessions))
	for name := range sessions {
		names = append(names, name)
	}
	for _, name := range pq.ExtraSessions {
		if _, exists := sessions[name]; !exists {
			names = append(names, name)
		}
	}
	for _, m := range fuzzy.Find(pq.Term, names) {
		sm := ensureSession(acc, m.Str)
		sm.Score = maxScore(sm.Score, m.Score)
		sm.MatchPos = m.MatchedIndexes
	}
}

// matchWindowNames fuzzy-matches the query term against window names across all sessions.
func matchWindowNames(pq ParsedQuery, sessions map[string]map[int][]tmux.Pane, acc map[string]*SessionMatch) {
	for sessionName, windows := range sessions {
		winNames := make([]string, 0, len(windows))
		// winIdxOrder maps dense slice positions -> real tmux window indices,
		// since fuzzy.Find returns m.Index as position in the input slice.
		winIdxOrder := make([]int, 0, len(windows))
		for idx, wPanes := range windows {
			if len(wPanes) > 0 {
				winNames = append(winNames, wPanes[0].WindowName)
				winIdxOrder = append(winIdxOrder, idx)
			}
		}
		for _, m := range fuzzy.Find(pq.Term, winNames) {
			sm := ensureSession(acc, sessionName)
			wm := WindowMatch{
				Index:    winIdxOrder[m.Index],
				Name:     m.Str,
				Score:    m.Score,
				MatchPos: m.MatchedIndexes,
			}
			sm.Windows = append(sm.Windows, wm)
			sm.Score = maxScore(sm.Score, m.Score)
		}
	}
}

// matchProcessNames fuzzy-matches the query term against descendant process names in each session.
func matchProcessNames(pq ParsedQuery, sessions map[string]map[int][]tmux.Pane, tree map[int32][]proc.Process, acc map[string]*SessionMatch) {
	for sessionName, windows := range sessions {
		var descendants []proc.Process
		for _, wPanes := range windows {
			for _, pane := range wPanes {
				descendants = append(descendants, collectDescendants(pane.PanePID, tree)...)
			}
		}
		procNames := make([]string, len(descendants))
		for i, p := range descendants {
			procNames[i] = p.FriendlyName()
		}
		for _, m := range fuzzy.Find(pq.Term, procNames) {
			sm := ensureSession(acc, sessionName)
			p := descendants[m.Index]
			pm := ProcMatch{
				PID:      p.PID,
				Name:     m.Str,
				Score:    m.Score,
				MatchPos: m.MatchedIndexes,
			}
			sm.Procs = append(sm.Procs, pm)
			sm.Score = maxScore(sm.Score, m.Score)
		}
	}
}

// RunWith is the pure core: accepts pre-fetched data, safe to use in tests.
func RunWith(pq ParsedQuery, panes []tmux.Pane, procs []proc.Process) Result {
	if pq.Term == "" {
		return Result{}
	}

	sessions := tmux.GroupBySessions(panes)
	acc := make(map[string]*SessionMatch, len(sessions))

	switch pq.Scope {
	case ScopeSession:
		matchSessionNames(pq, sessions, acc)
	case ScopeWindow:
		matchWindowNames(pq, sessions, acc)
	case ScopeProcess:
		tree := proc.BuildTree(procs)
		matchProcessNames(pq, sessions, tree, acc)
	}

	result := Result{Sessions: make([]SessionMatch, 0, len(acc))}
	for _, sm := range acc {
		result.Sessions = append(result.Sessions, *sm)
	}
	return result
}

// Run fetches live tmux panes and (if needed) processes, then calls RunWith.
func Run(pq ParsedQuery) (Result, error) {
	panes, err := tmux.ListPanes()
	if err != nil {
		return Result{}, err
	}
	var procs []proc.Process
	if pq.Scope == ScopeProcess {
		procs, err = proc.Snapshot()
		if err != nil {
			return Result{}, err
		}
	}
	return RunWith(pq, panes, procs), nil
}

// collectDescendants does a depth-first walk of the process tree from pid.
func collectDescendants(pid int32, tree map[int32][]proc.Process) []proc.Process {
	var result []proc.Process
	for _, child := range tree[pid] {
		result = append(result, child)
		result = append(result, collectDescendants(child.PID, tree)...)
	}
	return result
}
