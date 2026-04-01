package tmux

// PrimaryPaneCWD returns the CWD of pane index 0, falling back to the first pane.
// Returns "" if panes is empty.
func PrimaryPaneCWD(panes []Pane) string {
    for _, p := range panes {
        if p.PaneIndex == 0 {
            return p.CWD
        }
    }
    if len(panes) > 0 {
        return panes[0].CWD
    }
    return ""
}
