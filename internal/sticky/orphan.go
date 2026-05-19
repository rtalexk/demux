package sticky

import (
	"strconv"
	"strings"

	"github.com/rtalexk/demux/internal/config"
)

// HandleWindowAfterKill is invoked from the after-kill-pane hook with the
// window id where the kill happened. When that window is left with only a
// sidebar slot pane (i.e. the user closed the last non-slot pane), we close
// the window cleanly so a useless slot-only window does not linger:
//
//   - If the orphan slot is the active sidebar for some client (its pane id
//     appears as a DEMUX_STICKY_PANE_* env value), the sidebar pane is moved
//     into the last-active sibling window via join-pane. The source window
//     auto-closes because it ends up with zero panes.
//   - Otherwise the orphan is a placeholder; it is killed outright and the
//     source window closes naturally.
//
// No-op when windowID is empty, when list-panes errors (window already gone),
// when the window has more than one pane or its lone pane is not a slot, or
// when the session has no sibling window to receive the sidebar (closing the
// last window would terminate the session).
func (s *Sticky) HandleWindowAfterKill(windowID string) error {
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
	if count != 1 || slot != "1" {
		return nil
	}

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
		_ = s.T.Run("kill-pane", "-t", orphan)
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
		return nil
	}

	if targetSlot, _ := s.FindSlotInWindow(target); targetSlot != "" && targetSlot != orphan {
		_ = s.T.Run("kill-pane", "-t", targetSlot)
	}

	width := config.DefaultStickySidebarWidth
	if wOut, _ := s.T.Output("display-message", "-p", "-t", orphan, "#{pane_width}"); strings.TrimSpace(wOut) != "" {
		if w, perr := strconv.Atoi(strings.TrimSpace(wOut)); perr == nil && w > 0 {
			width = w
		}
	}

	_ = s.T.Run("join-pane", "-f", "-h", "-b", "-d",
		"-l", strconv.Itoa(width),
		"-s", orphan, "-t", target)
	_ = s.T.Run("select-window", "-t", target)
	return nil
}
