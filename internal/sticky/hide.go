package sticky

// Hide removes the sticky sidebar pane for the current client. It is a no-op
// when no pane is tracked. Errors from kill-pane (typically "pane not found"
// after a manual kill) are tolerated so the env var is still cleared.
func (s *Sticky) Hide() error {
	tty, err := s.CurrentClientTTY()
	if err != nil {
		return err
	}
	key := EnvKey(tty)
	val, set, err := s.ReadEnv(key)
	if err != nil {
		return err
	}
	if !set {
		return nil
	}
	if val != "" {
		_ = s.T.Run("kill-pane", "-t", val)
	}
	return s.UnsetEnv(key)
}
