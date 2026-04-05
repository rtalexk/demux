package cmd

import (
	"strings"

	"github.com/rtalexk/demux/internal/db"
	"github.com/spf13/cobra"
)

var eventPaneFocusTarget string

var eventCmd = &cobra.Command{
	Use:   "event",
	Short: "Send an external event to demux",
}

var eventPaneFocusCmd = &cobra.Command{
	Use:   "pane_focus",
	Short: "Clear alerts for the focused pane, its window, and its session",
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
	if err := d.AlertRemoveIfNotSticky(paneTarget); err != nil {
		return err
	}
	if err := d.AlertRemoveIfNotSticky(windowTargetFromPane(paneTarget)); err != nil {
		return err
	}
	return d.AlertRemoveIfNotSticky(sessionTargetFromPane(paneTarget))
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
