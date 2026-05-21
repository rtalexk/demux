package sticky

// Toggle hides the sticky sidebar if a live pane is tracked, otherwise shows
// it. Stale env entries (env set but pane gone) are treated as "not shown"
// and fall through to Show.
//
// When FocusBeforeToggleClose is set and the tracked pane is live but is not
// the client's active pane, Toggle moves focus into the sidebar instead of
// hiding it; a subsequent Toggle from inside the sidebar then hides it. If the
// active pane cannot be determined, Toggle falls through to Hide.
func (s *Sticky) Toggle(opts ShowOpts) error {
	tty, err := s.CurrentClientTTY()
	if err != nil {
		return err
	}
	key := EnvKey(tty)
	val, set, err := s.ReadEnv(key)
	if err != nil {
		return err
	}
	if set && val != "" {
		alive, err := s.PaneAlive(val)
		if err != nil {
			return err
		}
		if alive {
			if s.FocusBeforeToggleClose {
				if active, err := s.activePane(); err == nil && active != val {
					s.focusPane(val)
					return nil
				}
			}
			return s.Hide()
		}
	}
	return s.Show(opts)
}
