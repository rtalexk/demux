package sticky

// MaybePromoteSlot activates the current window's sidebar slot if all of:
//   - slots mode is enabled (s.Slots),
//   - this client has no tracked sidebar (env unset or its pane is dead),
//   - at least one slot pane exists somewhere on the server (i.e. the user has
//     opted into slots via `demux sidebar slots install` or a prior `show`),
//   - the current window already has a slot pane.
//
// Hooked into the session_changed / window_focus event so that switching to a
// session with installed slots brings the sidebar along, without forcing every
// session change through `auto_show=true` (which also fires on client attach).
//
// Unlike Show, this path is deliberately narrow: respawn the slot, record the
// env, optionally focus. NO ReconcileSlots, NO EnsureSlotInWindow elsewhere.
// Reconciliation would kick off a server-wide kill-pane cascade on every
// keystroke that promotes a slot, and the resulting after-kill-pane sweeps
// piling up would freeze tmux input. Slot hygiene is left to the
// after-new-window event and the interactive `slots prune` escape hatch.
//
// Best-effort: any tmux failure short-circuits to nil so the navigation hook
// driving this never crashes the user.
func (s *Sticky) MaybePromoteSlot(opts ShowOpts) error {
	if !s.Slots {
		return nil
	}
	tty, err := s.CurrentClientTTY()
	if err != nil {
		return nil
	}
	key := EnvKey(tty)
	val, set, err := s.ReadEnv(key)
	if err != nil {
		return nil
	}
	if set && val != "" {
		alive, err := s.PaneAlive(val)
		if err == nil && alive {
			return nil
		}
	}
	any, err := s.AnySlotExists()
	if err != nil || !any {
		return nil
	}
	target, _, _, err := s.currentTarget()
	if err != nil {
		return nil
	}
	slot, err := s.FindSlotInWindow(target)
	if err != nil || slot == "" {
		return nil
	}
	if err := s.T.Run("respawn-pane", "-k", "-t", slot, opts.Cmd); err != nil {
		return nil
	}
	if err := s.WriteEnv(key, slot); err != nil {
		return nil
	}
	s.maybeFocusOnOpen(slot)
	return nil
}
