package sticky

import (
	"fmt"
	"strconv"
	"strings"
)

// ShowOpts controls a Show invocation.
type ShowOpts struct {
	// Width is the initial pane width in columns. Honoured only when the pane
	// is being created (i.e. no live pane is already tracked).
	Width int
	// Cmd is the shell command to run inside the new pane.
	Cmd string
}

// Show ensures a sticky sidebar pane exists for the current client. If one is
// already tracked and the pane is alive, Show is a no-op. Otherwise it splits
// the current window with a full-height left column of opts.Width columns and
// records the new pane id.
func (s *Sticky) Show(opts ShowOpts) error {
	if err := s.VersionOK(); err != nil {
		return err
	}
	tty, err := s.CurrentClientTTY()
	if err != nil {
		return err
	}
	key := EnvKey(tty)
	val, set, err := s.ReadEnv(key)
	if err != nil {
		return err
	}
	if set && val != "" {
		alive, err := s.PaneAlive(val)
		if err != nil {
			return err
		}
		if alive {
			return nil
		}
	}
	target, err := s.T.Output("display-message", "-p", "#{session_id}:#{window_id}")
	if err != nil {
		return fmt.Errorf("tmux display-message current target: %w", err)
	}
	target = strings.TrimSpace(target)
	out, err := s.T.Output(
		"split-window", "-f", "-h", "-b",
		"-l", strconv.Itoa(opts.Width),
		"-t", target,
		"-P", "-F", "#{pane_id}",
		opts.Cmd,
	)
	if err != nil {
		return fmt.Errorf("tmux split-window: %w", err)
	}
	newPane := strings.TrimSpace(out)
	if newPane == "" {
		return fmt.Errorf("tmux split-window returned empty pane id")
	}
	return s.WriteEnv(key, newPane)
}
