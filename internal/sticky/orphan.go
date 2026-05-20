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
	for _, line := range nonEmptyLines(panesOut) {
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
	if envs, err := listStickyPaneEnv(s.T); err == nil {
		for _, e := range envs {
			if e.pane == orphan {
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

	s.moveOrphanSidebar(orphan, windowID, width)
	return nil
}

// EjectSidebarIfAloneAfterExit is the proactive companion to
// HandleWindowAfterKill. It is called from the pane-exited hook BEFORE
// exitingPaneID is actually destroyed. By excluding exitingPaneID from the
// pane count it detects that the sidebar will be left alone and relocates it
// immediately, so the user is not briefly stranded in a sidebar-only window
// while waiting for after-kill-pane to fire (which may be unreliable for
// natural process exits in some tmux versions).
//
// No-op when the window has more than two panes, when the remaining pane after
// excluding exitingPaneID is not the tracked sidebar, or when there is no
// sibling window to receive the sidebar.
func (s *Sticky) EjectSidebarIfAloneAfterExit(exitingPaneID, windowID string, width int) error {
	if exitingPaneID == "" || windowID == "" {
		return nil
	}
	panesOut, err := s.T.Output("list-panes", "-t", windowID, "-F", "#{pane_id} #{@demux_slot}")
	if err != nil {
		return nil
	}
	var orphan string
	remaining := 0
	for _, line := range nonEmptyLines(panesOut) {
		parts := strings.SplitN(line, " ", 2)
		paneID := parts[0]
		if paneID == exitingPaneID {
			continue
		}
		remaining++
		orphan = paneID
	}
	if remaining != 1 {
		return nil
	}
	isSidebar := false
	if envs, err := listStickyPaneEnv(s.T); err == nil {
		for _, e := range envs {
			if e.pane == orphan {
				isSidebar = true
				break
			}
		}
	}
	if !isSidebar {
		return nil
	}
	demuxlog.Info("pre-exit: sidebar will be alone, relocating", "window_id", windowID, "pane_id", orphan)
	s.moveOrphanSidebar(orphan, windowID, width)
	return nil
}

// moveOrphanSidebar relocates the sidebar pane (orphan) from windowID to the
// last-active sibling window. It is shared by HandleWindowAfterKill (called
// after the last non-sidebar pane is gone) and EjectSidebarIfAloneAfterExit
// (called while the exiting pane is still present).
func (s *Sticky) moveOrphanSidebar(orphan, windowID string, width int) {
	sessOut, err := s.T.Output("display-message", "-t", windowID, "-p", "#{session_id}")
	if err != nil {
		return
	}
	sessID := strings.TrimSpace(sessOut)
	target, err := s.siblingWindow(sessID, windowID)
	if err != nil {
		return
	}
	if target == "" {
		demuxlog.Info("orphan: no sibling window in session, skip", "window_id", windowID, "session_id", sessID)
		return
	}
	// Defense in depth: if a concurrent pane_closed handler has already moved
	// the sidebar out of windowID, do not move it a second time.
	if curWin, curErr := s.T.Output("display-message", "-p", "-t", orphan, "#{window_id}"); curErr == nil {
		if w := strings.TrimSpace(curWin); w != "" && w != windowID {
			demuxlog.Info("orphan: sidebar already moved, skip", "pane_id", orphan, "now_window", w)
			return
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
}

// siblingWindow returns a window in sessID other than excludeWindow that can
// receive a folded-in sidebar. It prefers the session's last-active window
// (window_last_flag=1); otherwise it falls back to the first other window in
// tmux index order. Returns "" when sessID has no other window.
func (s *Sticky) siblingWindow(sessID, excludeWindow string) (string, error) {
	out, err := s.T.Output("list-windows", "-t", sessID, "-F", "#{window_id} #{window_last_flag}")
	if err != nil {
		return "", err
	}
	var fallback string
	for _, line := range nonEmptyLines(out) {
		parts := strings.SplitN(line, " ", 2)
		wid := parts[0]
		if wid == excludeWindow {
			continue
		}
		if len(parts) >= 2 && strings.TrimSpace(parts[1]) == "1" {
			return wid, nil
		}
		if fallback == "" {
			fallback = wid
		}
	}
	return fallback, nil
}
