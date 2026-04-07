package cmd

import (
	"strings"

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

var eventPaneFocusTarget string

var eventCmd = &cobra.Command{
	Use:   "event",
	Short: "Send an external event to demux",
}

var eventPaneFocusCmd = &cobra.Command{
	Use:   "pane_focus",
	Short: "Clear done states for the focused pane, its window, and its session",
	RunE: func(cmd *cobra.Command, args []string) error {
		target := eventPaneFocusTarget
		if target == "" {
			var err error
			target, err = tmuxPaneTarget()
			if err != nil {
				return err
			}
		}

		d, err := openDB()
		if err != nil {
			return err
		}
		defer d.Close()

		return applyPaneFocus(d, target)
	},
}

func applyPaneFocus(d *db.DB, paneTarget string) error {
	demuxlog.Debug("pane_focus", "target", paneTarget)
	if err := d.StateDeleteIfResting(paneTarget); err != nil {
		demuxlog.Error("pane_focus: delete resting state failed", "target", paneTarget, "err", err)
		return err
	}
	windowTarget := windowTargetFromPane(paneTarget)
	if err := d.StateDeleteIfResting(windowTarget); err != nil {
		demuxlog.Error("pane_focus: delete resting state failed", "target", windowTarget, "err", err)
		return err
	}
	sessionTarget := sessionTargetFromPane(paneTarget)
	if err := d.StateDeleteIfResting(sessionTarget); err != nil {
		demuxlog.Error("pane_focus: delete resting state failed", "target", sessionTarget, "err", err)
		return err
	}
	return nil
}

func windowTargetFromPane(paneTarget string) string {
	if i := strings.LastIndex(paneTarget, "."); i != -1 {
		return paneTarget[:i]
	}
	return paneTarget
}

func sessionTargetFromPane(paneTarget string) string {
	if i := strings.Index(paneTarget, ":"); i != -1 {
		return paneTarget[:i]
	}
	return paneTarget
}

var eventHookErrorCmd = &cobra.Command{
	Use:   "hook_error",
	Short: "Record a hook delivery failure",
	Long: `Records a hook delivery failure to the log file. Use this as a fallback
in hook scripts instead of '|| true' so failures are observable:

  demux state set ... || demux event hook_error --hook Stop --target "$T" --message "state set failed" --set-state-error`,
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
		if err := d.StateSet(hookErrorTarget, hookErrorTool, db.StateError, hookErrorMessage, db.SourceTool, true, nil); err != nil {
			demuxlog.Error("hook_error: set state error failed", "target", hookErrorTarget, "err", err)
			return err
		}
	}
	return nil
}

func init() {
	eventPaneFocusCmd.Flags().StringVar(&eventPaneFocusTarget, "target", "", "Pane target: session:window.pane (auto-detected if omitted)")

	eventHookErrorCmd.Flags().StringVar(&hookErrorTarget, "target", "", "Pane target: session:window.pane (optional)")
	eventHookErrorCmd.Flags().StringVar(&hookErrorHook, "hook", "", "Hook name that failed (e.g. Stop, PreToolUse)")
	eventHookErrorCmd.Flags().StringVar(&hookErrorTool, "tool", "", "Tool name (e.g. claude)")
	eventHookErrorCmd.Flags().StringVar(&hookErrorMessage, "message", "", "Failure description")
	eventHookErrorCmd.Flags().BoolVar(&hookErrorSetStateError, "set-state-error", false, "Set the target state to error (requires --target)")
	eventHookErrorCmd.MarkFlagRequired("hook")

	eventCmd.AddCommand(eventPaneFocusCmd, eventHookErrorCmd)
	rootCmd.AddCommand(eventCmd)
}
