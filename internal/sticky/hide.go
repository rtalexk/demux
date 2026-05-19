package sticky

// Hide tears down the sticky sidebar for the current client.
//
// In split mode the tracked pane is killed. In slots mode every slot pane on
// the server is killed (including the active sidebar pane, which itself is
// tagged @demux_slot). The env var is always cleared so the next Show
// treats the sidebar as inactive. Errors from kill-pane are tolerated.
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
	if s.Slots {
		_ = s.UninstallSlots()
	} else if val != "" {
		_ = s.T.Run("kill-pane", "-t", val)
	}
	return s.UnsetEnv(key)
}
