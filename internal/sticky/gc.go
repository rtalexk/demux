package sticky

import (
	"fmt"
)

// ClearStickyEnvForPane scans every tmux server-global env var and unsets any
// DEMUX_STICKY_PANE_* whose value matches paneID. Intended to be called from
// the `after-kill-pane` hook path so a manual prefix-x on the sticky pane
// cleanly converts to "sticky off for that client".
//
// If at least one env var matched the killed pane (i.e. the user killed an
// active sticky sidebar), any remaining slot panes are also uninstalled so
// slots mode does not leave orphan placeholders across the other windows.
// In split mode there are no slot-tagged panes, so the uninstall pass is a
// no-op.
func ClearStickyEnvForPane(t Tmux, paneID string) error {
	envs, err := listStickyPaneEnv(t)
	if err != nil {
		return fmt.Errorf("tmux show-environment -g: %w", err)
	}
	matched := false
	for _, e := range envs {
		if e.pane == paneID {
			_ = t.Run("set-environment", "-gu", e.key)
			matched = true
		}
	}
	if matched {
		_ = (&Sticky{T: t}).UninstallSlots()
	}
	return nil
}
