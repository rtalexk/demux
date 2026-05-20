package sticky

import (
	"strconv"
	"strings"

	"github.com/rtalexk/demux/internal/config"
	demuxlog "github.com/rtalexk/demux/internal/log"
)

// HandleWindowAfterKill is invoked from the after-kill-pane hook with the
// window id where the kill happened and the configured sidebar width. When
// that window is left with only one pane that is either an active sidebar pane
// (split mode) or a sidebar slot pane (slots mode), we close the window
// cleanly so a useless slot-only or sidebar-only window does not linger:
//
//   - If the lone pane is the active sidebar for some client (its pane id
//     appears as a DEMUX_STICKY_PANE_* env value), the sidebar pane is moved
//     into the last-active sibling window, sized to width. When that window
//     has a placeholder slot the move is a swap-pane (no pane killed, so the
//     after-kill-pane hook is not re-entered); otherwise it is a join-pane.
//     The source window auto-closes because it ends up with zero panes.
//   - Otherwise, if the lone pane is a slot (placeholder, slots mode), it is
//     killed outright and the source window closes naturally.
//   - Otherwise (a regular pane unrelated to sticky), it is left alone.
//
// width is the caller's configured sidebar width. The orphan pane's own
// #{pane_width} cannot be used: by the time this hook runs the pane has
// already reflowed to fill its now-single-pane source window.
//
// No-op when windowID is empty, when list-panes errors (window already gone),
// when the window has more than one pane, when the session has no sibling
// window to receive the sidebar (closing the last window would terminate the
// session), or when the sidebar has already been moved out of windowID by a
// concurrent handler (the function is idempotent under the hook re-firing it
// triggers itself).
func (s *Sticky) HandleWindowAfterKill(windowID string, width int) error {
	if windowID == "" {
		return nil
	}
	panesOut, err := s.T.Output("list-panes", "-t", windowID, "-F", "#{pane_id} #{@demux_slot}")
	if err != nil {
		return nil
	}
	var orphan, slot string
	count := 0
	for _, line := range strings.Split(panesOut, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		count++
		parts := strings.SplitN(line, " ", 2)
		orphan = parts[0]
		slot = ""
		if len(parts) >= 2 {
			slot = strings.TrimSpace(parts[1])
		}
	}
	if count != 1 {
		return nil
	}
	isSlot := slot == "1"
	demuxlog.Info("orphan: candidate", "window_id", windowID, "pane_id", orphan, "is_slot", isSlot)

	isSidebar := false
	if envOut, err := s.T.Output("show-environment", "-g"); err == nil {
		for _, line := range strings.Split(envOut, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, envKeyPrefix) {
				continue
			}
			eq := strings.IndexByte(line, '=')
			if eq < 0 {
				continue
			}
			if line[eq+1:] == orphan {
				isSidebar = true
				break
			}
		}
	}

	if !isSidebar {
		if isSlot {
			demuxlog.Info("orphan: kill placeholder", "window_id", windowID, "pane_id", orphan)
			_ = s.T.Run("kill-pane", "-t", orphan)
		} else {
			demuxlog.Info("orphan: skip (not slot, not sidebar)", "window_id", windowID, "pane_id", orphan)
		}
		return nil
	}

	sessOut, err := s.T.Output("display-message", "-t", windowID, "-p", "#{session_id}")
	if err != nil {
		return nil
	}
	sessID := strings.TrimSpace(sessOut)
	winsOut, err := s.T.Output("list-windows", "-t", sessID, "-F", "#{window_id} #{window_last_flag}")
	if err != nil {
		return nil
	}
	var target, fallback string
	for _, line := range strings.Split(winsOut, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 1 {
			continue
		}
		wid := parts[0]
		if wid == windowID {
			continue
		}
		flag := ""
		if len(parts) >= 2 {
			flag = strings.TrimSpace(parts[1])
		}
		if flag == "1" {
			target = wid
			break
		}
		if fallback == "" {
			fallback = wid
		}
	}
	if target == "" {
		target = fallback
	}
	if target == "" {
		demuxlog.Info("orphan: no sibling window in session, skip", "window_id", windowID, "session_id", sessID)
		return nil
	}
	// Defense in depth: if a concurrent pane_closed handler has already moved
	// the sidebar out of windowID, do not move it a second time.
	if curWin, curErr := s.T.Output("display-message", "-p", "-t", orphan, "#{window_id}"); curErr == nil {
		if w := strings.TrimSpace(curWin); w != "" && w != windowID {
			demuxlog.Info("orphan: sidebar already moved, skip", "pane_id", orphan, "now_window", w)
			return nil
		}
	}

	demuxlog.Info("orphan: moving sidebar", "window_id", windowID, "pane_id", orphan, "target", target)

	if width <= 0 {
		width = config.DefaultStickySidebarWidth
	}

	targetSlot, _ := s.FindSlotInWindow(target)

	if targetSlot != "" && targetSlot != orphan {
		// swap-pane moves the sidebar into the target window's slot position
		// without killing any pane, so it does NOT re-fire after-kill-pane:
		// no re-entrant handler, no double move, no width corruption. The
		// placeholder slot ends up parked alone in the (now content-less)
		// source window.
		_ = s.T.Run("swap-pane", "-d", "-s", orphan, "-t", targetSlot)
		_ = s.T.Run("resize-pane", "-t", orphan, "-x", strconv.Itoa(width))
		// Remove the parked placeholder so the empty source window closes.
		// This kill re-fires after-kill-pane, but the move is already done so
		// the nested sweep is a no-op - and the kill happens in the source
		// window, so it cannot disturb the sidebar geometry in the target.
		_ = s.T.Run("kill-pane", "-t", targetSlot)
	} else {
		// No placeholder slot in the target (split mode, or slots partially
		// uninstalled): join the sidebar in directly.
		_ = s.T.Run("join-pane", "-f", "-h", "-b", "-d",
			"-l", strconv.Itoa(width),
			"-s", orphan, "-t", target)
	}
	_ = s.T.Run("select-window", "-t", target)
	return nil
}
