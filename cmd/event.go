package cmd

import (
	"github.com/rtalexk/demux/internal/db"
	demuxlog "github.com/rtalexk/demux/internal/log"
	"github.com/spf13/cobra"
)

var (
	hookErrorTarget        string
	hookErrorHook          string
	hookErrorTool          string
	hookErrorMessage       string
	hookErrorSetStateError bool
)

var eventPaneFocusPaneID    string
var eventPaneFocusWindowID  string
var eventPaneFocusSessionID string

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

		// Auto-detect if not provided
		if paneID == "" {
			var err error
			paneID, windowID, sessionID, err = tmuxCurrentIDs()
			if err != nil {
				return err
			}
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

	// Clear resting states at all three levels
	for _, targetType := range []string{"pane", "window", "session"} {
		var targetID string
		switch targetType {
		case "pane":
			targetID = paneID
		case "window":
			targetID = windowID
		case "session":
			targetID = sessionID
		}
		if targetID == "" {
			continue
		}

		t := db.Target{
			Type:      targetType,
			ID:        targetID,
			PaneID:    paneID,
			WindowID:  windowID,
			SessionID: sessionID,
		}
		if err := d.StateDeleteIfResting(t); err != nil {
			demuxlog.Error("pane_focus: delete resting state failed", "target", t, "err", err)
			return err
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
		t, parseErr := db.ParseTargetID(hookErrorTarget)
		if parseErr != nil {
			demuxlog.Warn("hook_error: invalid target id, skipping state set", "target", hookErrorTarget)
			return nil
		}
		if err := d.StateSet(t, hookErrorTool, db.StateError, hookErrorMessage, db.SourceTool, true, nil); err != nil {
			demuxlog.Error("hook_error: set state error failed", "target", t, "err", err)
			return err
		}
	}
	return nil
}

var eventPaneClosedPaneID string

var eventPaneClosedCmd = &cobra.Command{
	Use:   "pane_closed",
	Short: "Clear state for a closed tmux pane (called by after-kill-pane hook)",
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := openDB()
		if err != nil {
			return err
		}
		defer d.Close()
		return applyPaneClosed(d, eventPaneClosedPaneID)
	},
}

func applyPaneClosed(d *db.DB, paneID string) error {
	demuxlog.Debug("pane_closed", "pane_id", paneID)
	t := db.Target{Type: "pane", ID: paneID, PaneID: paneID}
	return d.StateClear(t)
}

func init() {
	eventPaneFocusCmd.Flags().StringVar(&eventPaneFocusPaneID, "pane-id", "", "Stable pane ID (%N format); auto-detected if omitted")
	eventPaneFocusCmd.Flags().StringVar(&eventPaneFocusWindowID, "window-id", "", "Stable window ID (@N format); auto-detected if omitted")
	eventPaneFocusCmd.Flags().StringVar(&eventPaneFocusSessionID, "session-id", "", "Stable session ID ($N format); auto-detected if omitted")

	eventHookErrorCmd.Flags().StringVar(&hookErrorTarget, "target-id", "", "Target ID (%N, @N, or $N) (optional)")
	eventHookErrorCmd.Flags().StringVar(&hookErrorHook, "hook", "", "Hook name that failed (e.g. Stop, PreToolUse)")
	eventHookErrorCmd.Flags().StringVar(&hookErrorTool, "tool", "", "Tool name (e.g. claude)")
	eventHookErrorCmd.Flags().StringVar(&hookErrorMessage, "message", "", "Failure description")
	eventHookErrorCmd.Flags().BoolVar(&hookErrorSetStateError, "set-state-error", false, "Set the target state to error (requires --target-id)")
	eventHookErrorCmd.MarkFlagRequired("hook")

	eventPaneClosedCmd.Flags().StringVar(&eventPaneClosedPaneID, "pane", "", "Stable pane ID (%N format) of the closed pane (required)")
	eventPaneClosedCmd.MarkFlagRequired("pane")

	eventCmd.AddCommand(eventPaneFocusCmd, eventHookErrorCmd, eventPaneClosedCmd)
	rootCmd.AddCommand(eventCmd)
}
