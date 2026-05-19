package sticky

import (
	"fmt"
	"strconv"
	"strings"
)

// SlotMarker is the tmux user-option set on every sidebar slot pane. Used to
// recognize slots when listing panes ("@demux_slot" -> "1").
const SlotMarker = "@demux_slot"

// SlotPaneTitle is the human-readable pane title applied to slot panes so
// they're recognizable when `pane-border-status` is on.
const SlotPaneTitle = "demux-slot"

// SlotPlaceholderCmd is the command run inside an inactive slot pane.
const SlotPlaceholderCmd = "demux sidebar slot"

// FindSlotInWindow returns the pane id of the slot in target window, or ""
// if the window has no slot pane.
func (s *Sticky) FindSlotInWindow(target string) (string, error) {
	out, err := s.T.Output("list-panes", "-t", target, "-F", "#{pane_id} #{@demux_slot}")
	if err != nil {
		return "", fmt.Errorf("tmux list-panes: %w", err)
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		if strings.TrimSpace(parts[1]) == "1" {
			return parts[0], nil
		}
	}
	return "", nil
}

// AnySlotExists reports whether any pane on the server is tagged as a sidebar
// slot. Used by the after-new-window hook to decide whether to ensure a slot
// in a freshly created window (only if the sidebar is already active for the
// session).
func (s *Sticky) AnySlotExists() (bool, error) {
	out, err := s.T.Output("list-panes", "-aF", "#{pane_id} #{@demux_slot}")
	if err != nil {
		return false, fmt.Errorf("tmux list-panes: %w", err)
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		if strings.TrimSpace(parts[1]) == "1" {
			return true, nil
		}
	}
	return false, nil
}

// EnsureSlotInWindow creates a sidebar slot pane in the target window if none
// exists. The new pane runs SlotPlaceholderCmd, gets the SlotMarker option,
// and is given a human-readable title.
func (s *Sticky) EnsureSlotInWindow(target string, width int) error {
	existing, err := s.FindSlotInWindow(target)
	if err != nil {
		return err
	}
	if existing != "" {
		return nil
	}
	out, err := s.T.Output(
		"split-window", "-f", "-h", "-b", "-d",
		"-l", strconv.Itoa(width),
		"-t", target,
		"-P", "-F", "#{pane_id}",
		SlotPlaceholderCmd,
	)
	if err != nil {
		return fmt.Errorf("tmux split-window slot: %w", err)
	}
	pane := strings.TrimSpace(out)
	if pane == "" {
		return fmt.Errorf("tmux split-window slot returned empty pane id")
	}
	if err := s.T.Run("set-option", "-p", "-t", pane, SlotMarker, "1"); err != nil {
		return fmt.Errorf("tmux set-option %s: %w", SlotMarker, err)
	}
	// Title is cosmetic; failure is non-fatal.
	_ = s.T.Run("select-pane", "-t", pane, "-T", SlotPaneTitle)
	return nil
}

// InstallSlotsInAllWindows ensures every window on the server has a sidebar
// slot pane at the given width. Idempotent.
func (s *Sticky) InstallSlotsInAllWindows(width int) error {
	out, err := s.T.Output("list-windows", "-aF", "#{session_id}:#{window_id}")
	if err != nil {
		return fmt.Errorf("tmux list-windows: %w", err)
	}
	var firstErr error
	for _, line := range strings.Split(out, "\n") {
		target := strings.TrimSpace(line)
		if target == "" {
			continue
		}
		if err := s.EnsureSlotInWindow(target, width); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// UninstallSlots kills every pane tagged as a sidebar slot.
func (s *Sticky) UninstallSlots() error {
	out, err := s.T.Output("list-panes", "-aF", "#{pane_id} #{@demux_slot}")
	if err != nil {
		return fmt.Errorf("tmux list-panes: %w", err)
	}
	var kills []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		if strings.TrimSpace(parts[1]) == "1" {
			kills = append(kills, parts[0])
		}
	}
	for _, p := range kills {
		_ = s.T.Run("kill-pane", "-t", p)
	}
	return nil
}
