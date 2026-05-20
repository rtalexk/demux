package sticky

// PrepareWindow is called by the TUI right before it issues a tmux switch to
// targetWindowID. In slots mode it pre-installs a slot pane in the target
// window so the after-select-window hook's follow can swap the sidebar in
// without a one-time split-window flicker.
//
// Steps: push target to the head of the MRU, prune stale entries, recompute
// the reserved set against the still-current window, and reconcile slot
// panes. Reconcile will install the slot in target (it sits at MRU head, so
// it's reserved) and evict any non-reserved slot that fell out of budget.
//
// No-op when slots mode is disabled, when no sidebar is active anywhere on
// the server, or when targetWindowID is empty. Best-effort overall: callers
// should not block their switch on this returning an error.
func (s *Sticky) PrepareWindow(targetWindowID string, width int) error {
	if !s.Slots || targetWindowID == "" {
		return nil
	}
	any, err := s.AnySlotExists()
	if err != nil || !any {
		return err
	}
	if err := s.PushMRU(targetWindowID); err != nil {
		return err
	}
	_, session, window, err := s.currentTarget()
	if err != nil {
		return nil
	}
	_ = s.PruneMRU()
	reserved, err := s.ComputeReservedWindows(window, session)
	if err != nil {
		return err
	}
	return s.ReconcileSlots(reserved, window, width)
}
