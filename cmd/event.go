package cmd

import (
	"github.com/rtalexk/demux/internal/db"
	demuxlog "github.com/rtalexk/demux/internal/log"
	"github.com/rtalexk/demux/internal/sticky"
	"github.com/spf13/cobra"
)

var (
	hookErrorTarget        string
	hookErrorHook          string
	hookErrorTool          string
	hookErrorMessage       string
	hookErrorSetStateError bool
)

var eventPaneFocusPaneID string
var eventPaneFocusWindowID string
var eventPaneFocusSessionID string
var eventPaneFocusTarget string // pre-v8: resolved to stable IDs via tmux

var eventCmd = &cobra.Command{
	Use:   "event",
	Short: "Send an external event to demux",
}

var eventPaneFocusCmd = &cobra.Command{
	Use:   "pane_focus",
	Short: "Clear done states for the focused pane, its window, and its session",
	RunE: func(cmd *cobra.Command, args []string) error {
		paneID := eventPaneFocusPaneID
		windowID := eventPaneFocusWindowID
		sessionID := eventPaneFocusSessionID

		var idErr error
		switch {
		case paneID != "":
			// New hook: stable IDs provided directly.
		case eventPaneFocusTarget != "":
			// Pre-v8 hook: resolve "session:window.pane" to stable IDs.
			paneID, windowID, sessionID, idErr = tmuxResolveTarget(eventPaneFocusTarget)
		default:
			paneID, windowID, sessionID, idErr = tmuxCurrentIDs()
		}
		if idErr != nil {
			return idErr
		}

		d, err := openDB()
		if err != nil {
			return err
		}
		defer d.Close()

		return applyPaneFocus(d, paneID, windowID, sessionID)
	},
}

func applyPaneFocus(d *db.DB, paneID, windowID, sessionID string) error {
	demuxlog.Debug("pane_focus", "pane_id", paneID, "window_id", windowID, "session_id", sessionID)

	// Clear resting states at all three levels.
	for _, item := range []struct {
		typ db.TargetType
		id  string
	}{
		{db.TargetTypePane, paneID},
		{db.TargetTypeWindow, windowID},
		{db.TargetTypeSession, sessionID},
	} {
		if item.id == "" {
			continue
		}
		t := db.Target{
			Type:      item.typ,
			ID:        item.id,
			PaneID:    paneID,
			WindowID:  windowID,
			SessionID: sessionID,
		}
		if err := d.StateDeleteIfResting(t); err != nil {
			demuxlog.Error("pane_focus: delete resting state failed", "target", t, "err", err)
			return err
		}
	}

	// Keep pane parent IDs fresh so TUI navigation stays accurate after moves
	// (e.g., break-pane, join-pane across sessions). Non-fatal: log and continue.
	if paneID != "" && windowID != "" && sessionID != "" {
		if err := d.StateRefreshParentIDs(paneID, windowID, sessionID); err != nil {
			demuxlog.Error("pane_focus: refresh parent IDs failed", "pane_id", paneID, "err", err)
		}
	}
	return nil
}

var eventHookErrorCmd = &cobra.Command{
	Use:   "hook_error",
	Short: "Record a hook delivery failure",
	Long: `Records a hook delivery failure to the log file. Use this as a fallback
in hook scripts instead of '|| true' so failures are observable:

  demux state set ... || demux event hook_error --hook Stop --target-id "$T" --message "state set failed" --set-state-error`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return applyHookError()
	},
}

func applyHookError() error {
	args := []any{"hook", hookErrorHook, "message", hookErrorMessage}
	if hookErrorTarget != "" {
		args = append(args, "target", hookErrorTarget)
	}
	demuxlog.Warn("hook_error", args...)

	if hookErrorTarget != "" && hookErrorSetStateError {
		d, err := openDB()
		if err != nil {
			demuxlog.Error("hook_error: open db failed", "err", err)
			return err
		}
		defer d.Close()
		return applyHookErrorDB(d, hookErrorTarget, hookErrorTool, hookErrorHook, hookErrorMessage)
	}
	return nil
}

// applyHookErrorDB writes a StateError record for targetID. It resolves
// parent IDs from tmux (best-effort) so the row is associated with its
// session in the TUI even when no prior state record exists.
func applyHookErrorDB(d *db.DB, targetID, tool, hook, message string) error {
	t, parseErr := db.ParseTargetID(targetID)
	if parseErr != nil {
		demuxlog.Warn("hook_error: invalid target id, skipping state set", "target", targetID, "hook", hook)
		return nil
	}
	// Resolve denormalized parent IDs (best-effort; no-op when tmux is unavailable).
	t = resolveParentIDs(t)
	if err := d.StateSet(t, tool, db.StateError, message, db.SourceTool, true, nil); err != nil {
		demuxlog.Error("hook_error: set state error failed", "target", t, "err", err)
		return err
	}
	return nil
}

var eventPaneExitingPaneID string
var eventPaneExitingWindowID string

var eventPaneExitingCmd = &cobra.Command{
	Use:   "pane_exiting",
	Short: "Eject sidebar if it would be stranded by an exiting pane (called by pane-exited hook)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := loadConfig()
		t := stickyTmuxForEvents()
		s := &sticky.Sticky{T: t}
		return s.EjectSidebarIfAloneAfterExit(eventPaneExitingPaneID, eventPaneExitingWindowID, cfg.Sidebar.Sticky.Width)
	},
}

var eventPaneClosedPaneID string
var eventPaneClosedWindowID string

var eventPaneClosedCmd = &cobra.Command{
	Use:   "pane_closed",
	Short: "Clear state for a closed tmux pane (called by after-kill-pane hook)",
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := openDB()
		if err != nil {
			return err
		}
		defer d.Close()
		return applyPaneClosed(d, eventPaneClosedPaneID, eventPaneClosedWindowID)
	},
}

func applyPaneClosed(d *db.DB, paneID, windowID string) error {
	demuxlog.Debug("pane_closed", "pane_id", paneID, "window_id", windowID)
	if paneID != "" {
		t := db.Target{Type: db.TargetTypePane, ID: paneID, PaneID: paneID}
		if err := d.StateClear(t); err != nil {
			return err
		}
	}
	tmuxClient := stickyTmuxForEvents()
	if paneID != "" {
		if err := sticky.ClearStickyEnvForPane(tmuxClient, paneID); err != nil {
			demuxlog.Warn("pane_closed: sticky env cleanup failed", "pane_id", paneID, "err", err)
		}
	}
	// tmux >= 3.5 leaves #{hook_pane} / #{hook_window} empty for
	// after-kill-pane, so we cannot rely on the targeted paths above. Always
	// run a server-wide sweep: stale env vars first, then orphan windows.
	//
	// This handler can re-enter itself: the sweep's kill-pane calls re-fire
	// after-kill-pane, and the hook's run-shell is synchronous, so the nested
	// handler runs to completion before kill-pane returns. The sweep is built
	// to be idempotent under that re-entry (see HandleWindowAfterKill), so no
	// lock is needed - and a lock would in fact deadlock against the nested,
	// synchronous handler.
	s := &sticky.Sticky{T: tmuxClient}
	if err := s.SweepStaleStickyEnv(); err != nil {
		demuxlog.Warn("pane_closed: stale env sweep failed", "err", err)
	}
	width := loadConfig().Sidebar.Sticky.Width
	if err := s.SweepOrphanWindows(width); err != nil {
		demuxlog.Warn("pane_closed: orphan window sweep failed", "err", err)
	}
	return nil
}

// stickyTmuxForEvents is a swappable real-tmux instance so tests can inject a
// fake without going through global state.
var stickyTmuxForEvents = func() sticky.Tmux {
	return sticky.New().T
}

func init() {
	eventPaneFocusCmd.Flags().StringVar(&eventPaneFocusPaneID, "pane-id", "", "Stable pane ID (%N format); auto-detected if omitted")
	eventPaneFocusCmd.Flags().StringVar(&eventPaneFocusWindowID, "window-id", "", "Stable window ID (@N format); auto-detected if omitted")
	eventPaneFocusCmd.Flags().StringVar(&eventPaneFocusSessionID, "session-id", "", "Stable session ID ($N format); auto-detected if omitted")
	// --target accepted for backward compatibility with pre-v8 hook snippets; value is ignored.
	eventPaneFocusCmd.Flags().StringVar(&eventPaneFocusTarget, "target", "", "")
	_ = eventPaneFocusCmd.Flags().MarkHidden("target")

	eventHookErrorCmd.Flags().StringVar(&hookErrorTarget, "target-id", "", "Target ID (%N, @N, or $N) (optional)")
	eventHookErrorCmd.Flags().StringVar(&hookErrorHook, "hook", "", "Hook name that failed (e.g. Stop, PreToolUse)")
	eventHookErrorCmd.Flags().StringVar(&hookErrorTool, "tool", "", "Tool name (e.g. claude)")
	eventHookErrorCmd.Flags().StringVar(&hookErrorMessage, "message", "", "Failure description")
	eventHookErrorCmd.Flags().BoolVar(&hookErrorSetStateError, "set-state-error", false, "Set the target state to error (requires --target-id)")
	eventHookErrorCmd.MarkFlagRequired("hook")

	eventPaneExitingCmd.Flags().StringVar(&eventPaneExitingPaneID, "pane", "", "Stable pane ID (%N format) of the exiting pane (required)")
	eventPaneExitingCmd.Flags().StringVar(&eventPaneExitingWindowID, "window", "", "Stable window ID (@N format) of the exiting pane's window (required)")
	eventPaneExitingCmd.MarkFlagRequired("pane")
	eventPaneExitingCmd.MarkFlagRequired("window")

	eventPaneClosedCmd.Flags().StringVar(&eventPaneClosedPaneID, "pane", "", "Stable pane ID (%N format) of the closed pane (required)")
	eventPaneClosedCmd.Flags().StringVar(&eventPaneClosedWindowID, "window", "", "Stable window ID (@N format) of the killed pane's window (optional)")
	eventPaneClosedCmd.MarkFlagRequired("pane")

	eventCmd.AddCommand(eventPaneFocusCmd, eventHookErrorCmd, eventPaneExitingCmd, eventPaneClosedCmd)
	rootCmd.AddCommand(eventCmd)
}
