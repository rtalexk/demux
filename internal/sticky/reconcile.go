package sticky

// ReconcileSlots brings the server's set of slot panes into agreement with
// reserved + currentWindow. For each window in reserved that lacks a slot, a
// placeholder is installed via EnsureSlotInWindow. For each window that has
// a slot but is neither reserved nor currentWindow, the slot is killed.
//
// Within any single window, only one slot is kept; extras are killed. This
// recovers from corrupt state where a race in EnsureSlotInWindow or a
// swap-pane edge case left two slot-tagged panes in the same window.
//
// protectedPane is the env-tracked active sidebar pane id for the invoking
// client ("" if none). When duplicates exist in currentWindow and one of
// them is protectedPane, the protected pane is kept as survivor. This stops
// prune from yanking the user's active sidebar - which would cascade: next
// Follow sees the tracked pane dead and tears down every other slot via
// UninstallSlots. Without the protection the survivor is whichever slot
// tmux happens to list first.
//
// width is the column count passed to EnsureSlotInWindow when creating new
// slots.
func (s *Sticky) ReconcileSlots(reserved []string, currentWindow, protectedPane string, width int) error {
	reservedSet := make(map[string]struct{}, len(reserved))
	for _, w := range reserved {
		reservedSet[w] = struct{}{}
	}

	out, err := s.T.Output("list-panes", "-aF", "#{window_id} #{pane_id} #{@demux_slot}")
	if err != nil {
		return err
	}
	existing := make(map[string]string)
	for _, row := range slotLines(out, 3) {
		wid, pid := row[0], row[1]
		if prev, has := existing[wid]; has {
			// Duplicate slot in same window: kill the extra. In currentWindow,
			// if the incoming pane is the protected one and the kept pane is
			// not, swap so the protected pane survives.
			toKill := pid
			if wid == currentWindow && protectedPane != "" && pid == protectedPane && prev != protectedPane {
				toKill = prev
				existing[wid] = pid
			}
			_ = s.T.Run("kill-pane", "-t", toKill)
			continue
		}
		existing[wid] = pid
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
