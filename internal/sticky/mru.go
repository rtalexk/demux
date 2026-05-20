package sticky

import (
	"strings"
)

// MRUEnvKey is the tmux global env var that stores the bounded MRU list of
// window ids that should receive sidebar slot panes. Head = most recently
// visited. Comma-separated. Bounded at MRUMaxLen.
const MRUEnvKey = "DEMUX_STICKY_MRU"

// MRUMaxLen caps both the in-memory MRU history and the placeholder slot
// budget. Total slot panes on the server is at most MRUMaxLen placeholders
// plus 1 for the active sidebar in the current window.
const MRUMaxLen = 8

// ReadMRU returns the MRU window-id list from the tmux global env var. Empty
// when unset. Entries are returned in MRU order (head = most recent).
func (s *Sticky) ReadMRU() ([]string, error) {
	val, set, err := s.ReadEnv(MRUEnvKey)
	if err != nil {
		return nil, err
	}
	if !set || val == "" {
		return nil, nil
	}
	parts := strings.Split(val, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out, nil
}

// WriteMRU persists the MRU list to the tmux env var. An empty list unsets it.
func (s *Sticky) WriteMRU(list []string) error {
	if len(list) == 0 {
		return s.UnsetEnv(MRUEnvKey)
	}
	return s.WriteEnv(MRUEnvKey, strings.Join(list, ","))
}

// PushMRU moves windowID to the head of the MRU list and persists it. Dedupes
// any existing occurrence. Trims the tail to MRUMaxLen entries. Empty
// windowID is a no-op.
func (s *Sticky) PushMRU(windowID string) error {
	if windowID == "" {
		return nil
	}
	current, err := s.ReadMRU()
	if err != nil {
		return err
	}
	out := make([]string, 0, len(current)+1)
	out = append(out, windowID)
	for _, w := range current {
		if w == windowID {
			continue
		}
		out = append(out, w)
		if len(out) >= MRUMaxLen {
			break
		}
	}
	return s.WriteMRU(out)
}

// PruneMRU removes window ids that no longer exist on the tmux server from
// the persisted MRU list. Safe to call before computing the reserved set so
// stale entries do not consume budget that should go to live windows.
func (s *Sticky) PruneMRU() error {
	current, err := s.ReadMRU()
	if err != nil {
		return err
	}
	if len(current) == 0 {
		return nil
	}
	live, err := s.liveWindowSet()
	if err != nil {
		return nil
	}
	kept := make([]string, 0, len(current))
	for _, w := range current {
		if _, ok := live[w]; ok {
			kept = append(kept, w)
		}
	}
	if len(kept) == len(current) {
		return nil
	}
	return s.WriteMRU(kept)
}

// liveWindowSet returns the set of @N window ids currently known to tmux.
func (s *Sticky) liveWindowSet() (map[string]struct{}, error) {
	out, err := s.T.Output("list-windows", "-aF", "#{window_id}")
	if err != nil {
		return nil, err
	}
	live := make(map[string]struct{})
	for _, line := range nonEmptyLines(out) {
		live[line] = struct{}{}
	}
	return live, nil
}
