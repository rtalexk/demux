package sticky

import (
	"fmt"
	"strings"
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
	out, err := t.Output("show-environment", "-g")
	if err != nil {
		return fmt.Errorf("tmux show-environment -g: %w", err)
	}
	matched := false
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, envKeyPrefix) {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		k, v := line[:eq], line[eq+1:]
		if v == paneID {
			_ = t.Run("set-environment", "-gu", k)
			matched = true
		}
	}
	if matched {
		_ = (&Sticky{T: t}).UninstallSlots()
	}
	return nil
}
