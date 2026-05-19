package sticky

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rtalexk/demux/internal/config"
)

// Follow moves the sticky sidebar pane into the current client's session,
// preserving the pane's current width. It is a no-op when:
//   - no sidebar is tracked for this client,
//   - the tracked pane is gone (user runs `show` to recreate),
//   - the pane is already in the current session.
//
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
	paneSession, err := s.T.Output("display-message", "-p", "-t", val, "#{session_id}")
	if err != nil {
		return fmt.Errorf("tmux display-message pane session: %w", err)
	}
	paneSession = strings.TrimSpace(paneSession)
	currParts := strings.SplitN(curr, ":", 2)
	if len(currParts) < 2 {
		return fmt.Errorf("unexpected current target %q", curr)
	}
	if paneSession == currParts[0] {
		return nil
	}
	widthStr, _ := s.T.Output("display-message", "-p", "-t", val, "#{pane_width}")
	width, _ := strconv.Atoi(strings.TrimSpace(widthStr))
	if width <= 0 {
		width = config.DefaultStickySidebarWidth
	}
	_ = s.T.Run("join-pane", "-f", "-h", "-b", "-l", strconv.Itoa(width), "-s", val, "-t", curr)
	return nil
}
