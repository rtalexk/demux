package sticky

// Toggle hides the sticky sidebar if a live pane is tracked, otherwise shows
// it. Stale env entries (env set but pane gone) are treated as "not shown"
// and fall through to Show.
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
			return s.Hide()
		}
	}
	return s.Show(opts)
}
