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

// slotLines parses `tmux list-panes -F` output whose format string ends with
// "#{@demux_slot}". fieldCount is the total number of space-separated fields
// per line, including that trailing slot tag. For each line tagged "1" it
// returns the leading fields (everything before the tag); other lines and
// lines with too few fields are skipped.
func slotLines(out string, fieldCount int) [][]string {
	var rows [][]string
	for _, line := range nonEmptyLines(out) {
		parts := strings.SplitN(line, " ", fieldCount)
		if len(parts) < fieldCount {
			continue
		}
		if strings.TrimSpace(parts[fieldCount-1]) == "1" {
			rows = append(rows, parts[:fieldCount-1])
		}
	}
	return rows
}

// FindSlotInWindow returns the pane id of the slot in target window, or ""
// if the window has no slot pane.
func (s *Sticky) FindSlotInWindow(target string) (string, error) {
	out, err := s.T.Output("list-panes", "-t", target, "-F", "#{pane_id} #{@demux_slot}")
	if err != nil {
		return "", fmt.Errorf("tmux list-panes: %w", err)
	}
	for _, row := range slotLines(out, 2) {
		return row[0], nil
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
	return len(slotLines(out, 2)) > 0, nil
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

// AddMissingSlots ensures each window in reserved has at least one slot pane,
// creating placeholders only where missing. Unlike ReconcileSlots it never
// kills existing panes - safe to call from hot paths (Show, Follow) where a
// kill-pane storm would cascade into pane_closed sweeps and saturate tmux.
// Idempotent.
func (s *Sticky) AddMissingSlots(reserved []string, width int) error {
	for _, wid := range reserved {
		if err := s.EnsureSlotInWindow(wid, width); err != nil {
			return err
		}
	}
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
	for _, target := range nonEmptyLines(out) {
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
	for _, row := range slotLines(out, 2) {
		_ = s.T.Run("kill-pane", "-t", row[0])
	}
	return nil
}
