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
//
// In slots mode (s.Slots==true), Show first installs a slot pane in every
// window, then respawns the current window's slot with opts.Cmd to make it
// the active sidebar pane.
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
	if s.Slots {
		return s.showSlotsMode(key, opts)
	}
	return s.showSplitMode(key, opts)
}

func (s *Sticky) showSplitMode(key string, opts ShowOpts) error {
	target, _, _, err := s.currentTarget()
	if err != nil {
		return err
	}
	out, err := s.T.Output(
		"split-window", "-f", "-h", "-b", "-d",
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
	if err := s.WriteEnv(key, newPane); err != nil {
		return err
	}
	if s.FocusOnOpen {
		s.focusPane(newPane)
	}
	return nil
}

func (s *Sticky) showSlotsMode(key string, opts ShowOpts) error {
	// Activate the current window's slot first so the user sees demux
	// appear immediately, then create placeholder slots in the remaining
	// windows in the background.
	target, currSession, currWindow, err := s.currentTarget()
	if err != nil {
		return err
	}
	if err := s.EnsureSlotInWindow(target, opts.Width); err != nil {
		return err
	}
	slot, err := s.FindSlotInWindow(target)
	if err != nil {
		return err
	}
	if slot == "" {
		return fmt.Errorf("no slot in current window after ensure")
	}
	if err := s.T.Run("respawn-pane", "-k", "-t", slot, opts.Cmd); err != nil {
		return fmt.Errorf("tmux respawn-pane: %w", err)
	}
	if err := s.WriteEnv(key, slot); err != nil {
		return err
	}
	if s.FocusOnOpen {
		s.focusPane(slot)
	}
	// Install slots only in the MRU-reserved set (capped at MRUMaxLen).
	// Current window already has its own slot (the sidebar), so we pass it
	// in explicitly so reconcile won't touch it.
	reserved, err := s.ComputeReservedWindows(currWindow, currSession)
	if err == nil {
		_ = s.ReconcileSlots(reserved, currWindow, opts.Width)
	}
	return nil
}
