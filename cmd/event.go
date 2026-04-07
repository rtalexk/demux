package cmd

import (
	"strings"

	"github.com/rtalexk/demux/internal/db"
	demuxlog "github.com/rtalexk/demux/internal/log"
	"github.com/spf13/cobra"
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

func init() {
	eventPaneFocusCmd.Flags().StringVar(&eventPaneFocusTarget, "target", "", "Pane target: session:window.pane (auto-detected if omitted)")
	eventCmd.AddCommand(eventPaneFocusCmd)
	rootCmd.AddCommand(eventCmd)
}
