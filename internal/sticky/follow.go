package sticky

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rtalexk/demux/internal/config"
)

// Follow moves the sticky sidebar pane into the current client's window
// (which implies the current session too), preserving the pane's current
// width. It is a no-op when:
//   - no sidebar is tracked for this client,
//   - the tracked pane is gone (user runs `show` to recreate),
//   - the pane is already in the current window.
//
// join-pane uses -d so user focus stays in the non-sidebar pane after the move.
// join-pane failures are swallowed so the tmux hook that drives Follow never
// crashes user navigation.
func (s *Sticky) Follow() error {
	tty, err := s.CurrentClientTTY()
	if err != nil {
		return err
	}
	key := EnvKey(tty)
	val, set, err := s.ReadEnv(key)
	if err != nil {
		return err
	}
	if !set || val == "" {
		return nil
	}
	alive, err := s.PaneAlive(val)
	if err != nil {
		return err
	}
	if !alive {
		return nil
	}
	curr, err := s.T.Output("display-message", "-p", "#{session_id}:#{window_id}")
	if err != nil {
		return fmt.Errorf("tmux display-message current target: %w", err)
	}
	curr = strings.TrimSpace(curr)
	paneWindow, err := s.T.Output("display-message", "-p", "-t", val, "#{window_id}")
	if err != nil {
		return fmt.Errorf("tmux display-message pane window: %w", err)
	}
	paneWindow = strings.TrimSpace(paneWindow)
	currParts := strings.SplitN(curr, ":", 2)
	if len(currParts) < 2 {
		return fmt.Errorf("unexpected current target %q", curr)
	}
	if paneWindow == currParts[1] {
		return nil
	}
	widthStr, _ := s.T.Output("display-message", "-p", "-t", val, "#{pane_width}")
	width, _ := strconv.Atoi(strings.TrimSpace(widthStr))
	if width <= 0 {
		width = config.DefaultStickySidebarWidth
	}
	if s.Slots {
		if err := s.followSlotsMode(val, curr, width); err != nil {
			return err
		}
		s.ejectFocusIfOnSidebar(val)
		s.pokeRefresh(val)
		return nil
	}
	_ = s.T.Run("join-pane", "-f", "-h", "-b", "-d", "-l", strconv.Itoa(width), "-s", val, "-t", curr)
	s.ejectFocusIfOnSidebar(val)
	s.pokeRefresh(val)
	return nil
}

// ejectFocusIfOnSidebar moves the client focus to the next pane in the
// current window when the sidebar pane is the active pane. Matches tmux's
// `prefix + o`. Without this, sticky-mode users who left focus inside the
// sidebar before switching tmux windows would still be inside the sidebar
// after the follow.
func (s *Sticky) ejectFocusIfOnSidebar(sidebarPane string) {
	if sidebarPane == "" {
		return
	}
	out, err := s.T.Output("display-message", "-p", "#{pane_id}")
	if err != nil {
		return
	}
	if strings.TrimSpace(out) == sidebarPane {
		_ = s.T.Run("select-pane", "-t", ":.+")
	}
}

// pokeRefresh sends F12 to the sticky pane. The TUI binds F12 (only
// reachable via this poke; not in the user-facing key list) to a
// refresh-plus-refocus action so the sidebar both picks up the new active
// session immediately AND snaps the cursor back to it. The auto-refresh
// tick still uses the plain refresh path so the user can browse the
// sidebar between window switches without their cursor being yanked.
func (s *Sticky) pokeRefresh(paneID string) {
	if paneID == "" {
		return
	}
	_ = s.T.Run("send-keys", "-t", paneID, "F12")
}

// followSlotsMode swaps the active sidebar pane with the destination window's
// slot pane (zero-flicker compared to join-pane), then resizes the swapped
// pane to preserve the user's chosen width.
func (s *Sticky) followSlotsMode(activePane, currTarget string, width int) error {
	slot, err := s.FindSlotInWindow(currTarget)
	if err != nil {
		return err
	}
	if slot == "" {
		if err := s.EnsureSlotInWindow(currTarget, width); err != nil {
			return nil
		}
		slot, err = s.FindSlotInWindow(currTarget)
		if err != nil || slot == "" {
			return nil
		}
	}
	if slot == activePane {
		return nil
	}
	_ = s.T.Run("swap-pane", "-d", "-s", activePane, "-t", slot)
	_ = s.T.Run("resize-pane", "-t", activePane, "-x", strconv.Itoa(width))
	return nil
}
