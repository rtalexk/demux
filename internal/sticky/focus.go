package sticky

import "strings"

// activePane returns the pane id the invoking tmux client currently has
// focused.
func (s *Sticky) activePane() (string, error) {
	out, err := s.T.Output("display-message", "-p", "#{pane_id}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// focusPane moves the invoking client's focus to paneID. Best-effort: a
// select-pane failure is swallowed so callers never crash on a focus nicety.
// An empty paneID is a no-op.
func (s *Sticky) focusPane(paneID string) {
	if paneID == "" {
		return
	}
	_ = s.T.Run("select-pane", "-t", paneID)
}

// maybeFocusOnOpen focuses the just-created sidebar pane when the FocusOnOpen
// config flag is set. Shared tail of both Show modes.
func (s *Sticky) maybeFocusOnOpen(paneID string) {
	if s.FocusOnOpen {
		s.focusPane(paneID)
	}
}
