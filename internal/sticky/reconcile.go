package sticky

import (
	"strings"
)

// ReconcileSlots brings the server's set of slot panes into agreement with
// reserved + currentWindow. For each window in reserved that lacks a slot, a
// placeholder is installed via EnsureSlotInWindow. For each window that has
// a slot but is neither reserved nor currentWindow, the slot is killed.
//
// currentWindow's slot (the active sidebar) is never touched here - that is
// the caller's responsibility via Show / Follow / swap-pane.
//
// width is the column count passed to EnsureSlotInWindow when creating new
// slots.
func (s *Sticky) ReconcileSlots(reserved []string, currentWindow string, width int) error {
	reservedSet := make(map[string]struct{}, len(reserved))
	for _, w := range reserved {
		reservedSet[w] = struct{}{}
	}

	out, err := s.T.Output("list-panes", "-aF", "#{window_id} #{pane_id} #{@demux_slot}")
	if err != nil {
		return err
	}
	existing := make(map[string]string)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 3 {
			continue
		}
		wid, pid, slot := parts[0], parts[1], strings.TrimSpace(parts[2])
		if slot != "1" {
			continue
		}
		if _, has := existing[wid]; !has {
			existing[wid] = pid
		}
	}

	for wid, pid := range existing {
		if wid == currentWindow {
			continue
		}
		if _, ok := reservedSet[wid]; ok {
			continue
		}
		_ = s.T.Run("kill-pane", "-t", pid)
	}

	for _, wid := range reserved {
		if _, has := existing[wid]; has {
			continue
		}
		_ = s.EnsureSlotInWindow(wid, width)
	}

	return nil
}
