package tmux

// PrimaryPaneCWD returns the CWD of the primary pane: pane index 0, falling
// back to the first pane. Sticky sidebar slot panes are skipped so an open
// sidebar does not hijack a session's primary path. Returns "" if panes is
// empty or contains only slot panes.
func PrimaryPaneCWD(panes []Pane) string {
	for _, p := range panes {
		if p.IsSlot {
			continue
		}
		if p.PaneIndex == 0 {
			return p.CWD
		}
	}
	for _, p := range panes {
		if !p.IsSlot {
			return p.CWD
		}
	}
	return ""
}
