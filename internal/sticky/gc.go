package sticky

import (
	"fmt"
	"strings"
)

// ClearStickyEnvForPane scans every tmux server-global env var and unsets any
// DEMUX_STICKY_PANE_* whose value matches paneID. Intended to be called from
// the `after-kill-pane` hook path so a manual prefix-x on the sticky pane
// cleanly converts to "sticky off for that client".
func ClearStickyEnvForPane(t Tmux, paneID string) error {
	out, err := t.Output("show-environment", "-g")
	if err != nil {
		return fmt.Errorf("tmux show-environment -g: %w", err)
	}
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
		}
	}
	return nil
}
