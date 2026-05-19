package sticky

import (
	"strings"
)

// SweepStaleStickyEnv scans every DEMUX_STICKY_PANE_* env var on the server
// and unsets any whose pane id no longer exists (or is empty). If after this
// pass no live sidebar remains tracked, slot placeholder panes across all
// windows are uninstalled too. Intended as a fallback for tmux builds where
// `#{hook_pane}` is not populated for `after-kill-pane`, leaving targeted
// cleanup blind to which pane was actually killed.
func (s *Sticky) SweepStaleStickyEnv() error {
	out, err := s.T.Output("show-environment", "-g")
	if err != nil {
		return nil
	}
	var stale []string
	anyLive := false
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, envKeyPrefix) {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		k, v := line[:eq], line[eq+1:]
		if v == "" {
			stale = append(stale, k)
			continue
		}
		alive, _ := s.PaneAlive(v)
		if alive {
			anyLive = true
			continue
		}
		stale = append(stale, k)
	}
	if len(stale) == 0 {
		return nil
	}
	for _, k := range stale {
		_ = s.T.Run("set-environment", "-gu", k)
	}
	if !anyLive {
		_ = s.UninstallSlots()
	}
	return nil
}

// SweepOrphanWindows applies HandleWindowAfterKill to every window on the
// server, so that any window currently left with only a sidebar slot pane is
// either folded into a sibling window (if it holds the active sidebar) or
// closed outright (if its slot is just a placeholder). Safe to call when no
// orphan exists - HandleWindowAfterKill is a no-op for healthy windows.
func (s *Sticky) SweepOrphanWindows() error {
	out, err := s.T.Output("list-windows", "-aF", "#{window_id}")
	if err != nil {
		return nil
	}
	for _, line := range strings.Split(out, "\n") {
		wid := strings.TrimSpace(line)
		if wid == "" {
			continue
		}
		_ = s.HandleWindowAfterKill(wid)
	}
	return nil
}
