package sticky

import (
	"strings"
)

// ComputeReservedWindows returns the window ids that should hold a sidebar
// slot pane right now, capped at MRUMaxLen. The current window is excluded
// (it holds the active sidebar pane, which is accounted for separately).
//
// Priority fill order (highest first):
//  1. window_last_flag=1 in current session (the tmux "last-window").
//  2. Active window of the client's last session (cross-session).
//  3. Other windows in the current session, ordered by MRU then tmux index.
//  4. Cross-session windows from MRU, in MRU order.
//
// Stops once MRUMaxLen entries are collected. Stale MRU entries (dead window
// ids) are silently skipped, but PruneMRU should be invoked separately to
// trim them from the persisted env var.
func (s *Sticky) ComputeReservedWindows(currentWindow, currentSession string) ([]string, error) {
	reserved := make([]string, 0, MRUMaxLen)
	seen := map[string]struct{}{}
	if currentWindow != "" {
		seen[currentWindow] = struct{}{}
	}

	add := func(w string) bool {
		if w == "" {
			return false
		}
		if _, ok := seen[w]; ok {
			return false
		}
		seen[w] = struct{}{}
		reserved = append(reserved, w)
		return len(reserved) >= MRUMaxLen
	}

	live, err := s.liveWindowSet()
	if err != nil {
		return nil, err
	}

	currentSessionWindows, currentLast, err := s.listCurrentSessionWindows(currentSession)
	if err != nil {
		return nil, err
	}

	if currentLast != "" {
		if _, ok := live[currentLast]; ok {
			if add(currentLast) {
				return reserved, nil
			}
		}
	}

	lastSessionActive, err := s.lastSessionActiveWindow(currentSession)
	if err == nil && lastSessionActive != "" {
		if _, ok := live[lastSessionActive]; ok {
			if add(lastSessionActive) {
				return reserved, nil
			}
		}
	}

	mru, err := s.ReadMRU()
	if err != nil {
		return nil, err
	}
	currentSet := map[string]struct{}{}
	for _, w := range currentSessionWindows {
		currentSet[w] = struct{}{}
	}

	for _, w := range mru {
		if _, ok := currentSet[w]; !ok {
			continue
		}
		if _, ok := live[w]; !ok {
			continue
		}
		if add(w) {
			return reserved, nil
		}
	}
	for _, w := range currentSessionWindows {
		if _, ok := live[w]; !ok {
			continue
		}
		if add(w) {
			return reserved, nil
		}
	}

	for _, w := range mru {
		if _, ok := live[w]; !ok {
			continue
		}
		if add(w) {
			return reserved, nil
		}
	}

	return reserved, nil
}

// listCurrentSessionWindows returns the window ids in currentSession in tmux
// index order, plus the id of the window with window_last_flag=1 (or "" if
// none).
func (s *Sticky) listCurrentSessionWindows(currentSession string) ([]string, string, error) {
	if currentSession == "" {
		return nil, "", nil
	}
	out, err := s.T.Output("list-windows", "-t", currentSession, "-F", "#{window_id} #{window_last_flag}")
	if err != nil {
		return nil, "", err
	}
	var windows []string
	var lastFlag string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 1 {
			continue
		}
		wid := parts[0]
		windows = append(windows, wid)
		if len(parts) >= 2 && strings.TrimSpace(parts[1]) == "1" {
			lastFlag = wid
		}
	}
	return windows, lastFlag, nil
}

// lastSessionActiveWindow returns the window id of the active window in the
// client's last session, or "" when there is no last session, the last
// session equals currentSession, or tmux cannot resolve it. Best-effort.
func (s *Sticky) lastSessionActiveWindow(currentSession string) (string, error) {
	name, err := s.T.Output("display-message", "-p", "#{client_last_session}")
	if err != nil {
		return "", err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", nil
	}
	out, err := s.T.Output("display-message", "-t", name, "-p", "#{session_id} #{window_id}")
	if err != nil {
		return "", err
	}
	parts := strings.Fields(strings.TrimSpace(out))
	if len(parts) < 2 {
		return "", nil
	}
	if parts[0] == currentSession {
		return "", nil
	}
	return parts[1], nil
}
