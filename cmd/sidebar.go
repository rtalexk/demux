package cmd

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/rtalexk/demux/internal/sticky"
	"github.com/spf13/cobra"
)

// stickyClient is overridable in tests so we can inject a fake Tmux.
var stickyClient = func() *sticky.Sticky {
	cfg := loadConfig()
	s := sticky.New()
	s.Slots = cfg.Sidebar.Sticky.Slots
	s.FocusOnOpen = cfg.Sidebar.Sticky.FocusOnOpen
	s.FocusBeforeToggleClose = cfg.Sidebar.Sticky.FocusBeforeToggleClose
	return s
}

var sidebarCmd = &cobra.Command{
	Use:   "sidebar",
	Short: "Manage the sticky sidebar tmux pane",
	Long: `Manage the per-client sticky sidebar tmux pane that runs
'demux --sticky' and follows the attached client across session switches.

These commands require the invoking shell to be inside tmux.`,
}

func sidebarShowOpts() sticky.ShowOpts {
	cfg := loadConfig()
	return sticky.ShowOpts{
		Width: cfg.Sidebar.Sticky.Width,
		Cmd:   "demux --sticky",
	}
}

var sidebarShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Create the sticky sidebar pane if it does not already exist",
	RunE: func(cmd *cobra.Command, args []string) error {
		return withEventLock(func() error { return stickyClient().Show(sidebarShowOpts()) })
	},
}

var sidebarHideCmd = &cobra.Command{
	Use:   "hide",
	Short: "Kill the sticky sidebar pane if it is currently shown",
	RunE: func(cmd *cobra.Command, args []string) error {
		return withEventLock(func() error { return stickyClient().Hide() })
	},
}

var sidebarToggleCmd = &cobra.Command{
	Use:   "toggle",
	Short: "Show the sticky sidebar pane if hidden, hide it if shown",
	RunE: func(cmd *cobra.Command, args []string) error {
		return withEventLock(func() error { return stickyClient().Toggle(sidebarShowOpts()) })
	},
}

var sidebarSlotsCmd = &cobra.Command{
	Use:   "slots",
	Short: "Manage per-window sidebar slots (sidebar.sticky.slots mode)",
}

var sidebarSlotsInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Create a sidebar slot pane in every existing window",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := loadConfig()
		return withEventLock(func() error { return stickyClient().InstallSlotsInAllWindows(cfg.Sidebar.Sticky.Width) })
	},
}

var sidebarSlotsUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Kill every sidebar slot pane",
	RunE: func(cmd *cobra.Command, args []string) error {
		return withEventLock(func() error { return stickyClient().UninstallSlots() })
	},
}

// sidebarSlotsPruneCmd reconciles the slot set against the MRU budget right
// now, without waiting for an after-new-window event. Use it to clean up
// corrupt state - e.g. duplicate slots in the same window from an old race,
// or slot panes left behind in non-reserved windows.
//
// Each kill-pane issued by reconcile normally fires the after-kill-pane and
// pane-exited tmux hooks, which fork backgrounded `demux event pane_closed` /
// `demux event pane_exiting` handlers. Each handler walks every window on the
// server and may itself issue more kill-pane calls, snowballing the cascade
// until tmux's command queue is saturated. To make prune cheap and bounded
// we temporarily clear those two hooks for the duration of the reconcile and
// restore them afterwards.
var sidebarSlotsPruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Reconcile slot panes against the MRU budget right now (fixes duplicates and stale slots)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return withEventLock(func() error {
			cfg := loadConfig()
			s := stickyClient()
			return withHooksSuppressed(s.T, []string{"after-kill-pane", "pane-exited"}, func() error {
				target, err := s.T.Output("display-message", "-p", "#{session_id}:#{window_id}")
				if err != nil {
					return err
				}
				parts := strings.SplitN(strings.TrimSpace(target), ":", 2)
				if len(parts) < 2 {
					return fmt.Errorf("unexpected current target %q", target)
				}
				_ = s.PruneMRU()
				reserved, err := s.ComputeReservedWindows(parts[1], parts[0])
				if err != nil {
					return err
				}
				return s.ReconcileSlots(reserved, parts[1], cfg.Sidebar.Sticky.Width)
			})
		})
	},
}

// withHooksSuppressed clears the given tmux global hooks for the duration of
// fn, restoring their original commands when fn returns. Used by prune to
// stop pane_closed/pane_exiting handlers from cascading on every kill-pane
// it issues.
//
// Both clear and restore are best-effort: if show-hooks fails for an event,
// we skip it (no clear, nothing to restore). The restore runs via defer so
// it fires even when fn panics.
//
// Side effects beyond fn: any third party that kills panes elsewhere on this
// tmux server during the suppression window will also miss those hooks. The
// window is short (a few tmux RPCs) so the trade is worth it.
func withHooksSuppressed(t sticky.Tmux, events []string, fn func() error) error {
	type saved struct {
		event string
		cmds  []string
	}
	var snapshots []saved
	for _, ev := range events {
		out, err := t.Output("show-hooks", "-g", ev)
		if err != nil {
			continue
		}
		cmds := parseSuppressedHookCmds(out, ev)
		snapshots = append(snapshots, saved{event: ev, cmds: cmds})
		_ = t.Run("set-hook", "-gu", ev)
	}
	defer func() {
		for _, snap := range snapshots {
			for _, c := range snap.cmds {
				_ = t.Run("set-hook", "-ga", snap.event, c)
			}
		}
	}()
	return fn()
}

// parseSuppressedHookCmds extracts the command strings registered on event
// from `tmux show-hooks -g <event>` output. Each input line has the form
//
//	after-kill-pane[0] run-shell -b "demux ..."
//
// and we return the trailing command portion (`run-shell -b "demux ..."`)
// for each line whose prefix matches event.
func parseSuppressedHookCmds(out, event string) []string {
	var cmds []string
	prefix := event + "["
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		// Skip past "event[N] " - the closing bracket may be followed by
		// any digits, so find the first space after the prefix.
		sp := strings.IndexByte(line, ' ')
		if sp < 0 || sp+1 >= len(line) {
			continue
		}
		cmds = append(cmds, line[sp+1:])
	}
	return cmds
}

// ensureSlots adds missing sidebar slot panes for the MRU-reserved windows.
// It is a no-op when slots mode is disabled or no slot panes have been
// installed. Called by the after-new-window hook via `demux event new_window`.
//
// Additive only - never kills panes. ReconcileSlots used to live here, but its
// kill-pane fan-out cascades through the backgrounded after-kill-pane /
// pane-exited handlers and saturates tmux's command queue. Dedupe and eviction
// belong to `demux sidebar slots prune`.
func ensureSlots() error {
	cfg := loadConfig()
	if !cfg.Sidebar.Sticky.Slots {
		return nil
	}
	s := stickyClient()
	any, err := s.AnySlotExists()
	if err != nil || !any {
		return err
	}
	target, err := s.T.Output("display-message", "-p", "#{session_id}:#{window_id}")
	if err != nil {
		return err
	}
	parts := strings.SplitN(strings.TrimSpace(target), ":", 2)
	if len(parts) < 2 {
		return nil
	}
	_ = s.PruneMRU()
	reserved, err := s.ComputeReservedWindows(parts[1], parts[0])
	if err != nil {
		return err
	}
	return s.AddMissingSlots(reserved, cfg.Sidebar.Sticky.Width)
}

var sidebarSlotCmd = &cobra.Command{
	Use:    "slot",
	Short:  "Internal: placeholder process for an inactive sticky sidebar slot",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		runSidebarSlotPlaceholder(cmd.OutOrStdout())
		return nil
	},
}

// runSidebarSlotPlaceholder is the body of `demux sidebar slot`: it paints a
// static placeholder screen in the slot's tmux pane and blocks until the pane
// is torn down.
//
// SIGINT is ignored so an accidental Ctrl-C while the slot pane is focused
// cannot destroy the reserved slot. SIGHUP and SIGTERM are deliberately NOT
// ignored: tmux delivers SIGHUP (by closing the pane's pty) when it kills the
// pane, and the placeholder MUST exit then so its pty device is released back
// to the system. Previously all three signals were ignored, which orphaned
// these processes onto launchd and leaked one pty each until the host hit
// kern.tty.ptmx_max and could no longer fork new terminals.
func runSidebarSlotPlaceholder(w io.Writer) {
	signal.Ignore(syscall.SIGINT)
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGHUP, syscall.SIGTERM)

	fmt.Fprint(w, "\x1b[2J\x1b[H\n\n")
	fmt.Fprintln(w, "  demux sidebar slot")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  Reserved by [sidebar.sticky] slots.")
	fmt.Fprintln(w, "  Activate this window's slot with")
	fmt.Fprintln(w, "  `demux sidebar show`.")

	<-done
}

func init() {
	rejectUnknownSubcommand(sidebarCmd)
	rejectUnknownSubcommand(sidebarSlotsCmd)
	sidebarSlotsCmd.AddCommand(sidebarSlotsInstallCmd, sidebarSlotsUninstallCmd, sidebarSlotsPruneCmd)
	sidebarCmd.AddCommand(sidebarShowCmd, sidebarHideCmd, sidebarToggleCmd, sidebarSlotCmd, sidebarSlotsCmd)
	rootCmd.AddCommand(sidebarCmd)
}
