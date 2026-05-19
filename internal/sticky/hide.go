package sticky

// Hide deactivates the sticky sidebar pane for the current client.
//
// In split mode the pane is killed. In slots mode the pane is respawned with
// the slot placeholder so the slot stays in place (matching the rest of the
// windows). The env var is always cleared so the next Show treats the
// sidebar as inactive. Errors from kill-pane/respawn-pane (typically "pane
// not found" after a manual kill) are tolerated.
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
		if s.Slots {
			_ = s.T.Run("respawn-pane", "-k", "-t", val, SlotPlaceholderCmd)
		} else {
			_ = s.T.Run("kill-pane", "-t", val)
		}
	}
	return s.UnsetEnv(key)
}
