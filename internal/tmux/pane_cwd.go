package tmux

// PrimaryPane returns the primary pane of a session or window: pane index 0,
// falling back to the first pane. Sticky sidebar slot panes are skipped so an
// open sidebar does not hijack the primary path. Returns nil if panes is empty
// or contains only slot panes.
func PrimaryPane(panes []Pane) *Pane {
	for i := range panes {
		if panes[i].IsSlot {
			continue
		}
		if panes[i].PaneIndex == 0 {
			return &panes[i]
		}
	}
	for i := range panes {
		if !panes[i].IsSlot {
			return &panes[i]
		}
	}
	return nil
}

// PrimaryPaneCWD returns the CWD of the primary pane (see PrimaryPane).
// Returns "" if panes is empty or contains only slot panes.
func PrimaryPaneCWD(panes []Pane) string {
	if p := PrimaryPane(panes); p != nil {
		return p.CWD
	}
	return ""
}
