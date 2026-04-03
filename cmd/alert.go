package cmd

import (
	"fmt"

	"github.com/rtalexk/demux/internal/format"
	"github.com/spf13/cobra"
)

var (
	alertSetTarget    string
	alertRemoveTarget string
	alertReason       string
	alertLevel        string
)

type alertRow struct {
	target  string
	level   string
	reason  string
	created string
}

func (r alertRow) Fields() []string {
	return []string{r.target, r.level, r.reason, r.created}
}

var alertCmd = &cobra.Command{
	Use:   "alert",
	Short: "Manage alerts",
	RunE:  runAlertList,
}

var alertListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List alerts",
	RunE:    runAlertList,
}

var alertSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set (create or replace) an alert",
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := openDB()
		if err != nil {
			return fmt.Errorf("open db: %w", err)
		}
		defer d.Close()

		if alertReason == "" {
			if alertLevel == "defer" {
				alertReason = loadConfig().Alerts.DeferDefaultReason
				if alertReason == "" {
					return fmt.Errorf("--reason is required (alerts.defer_default_reason is not set)")
				}
			} else {
				return fmt.Errorf("--reason is required")
			}
		}

		if err := d.AlertSet(alertSetTarget, alertReason, alertLevel); err != nil {
			return fmt.Errorf("alert set: %w", err)
		}
		fmt.Printf("Alert set for %s\n", alertSetTarget)
		return nil
	},
}

var alertRemoveCmd = &cobra.Command{
	Use:     "remove",
	Aliases: []string{"rm"},
	Short:   "Remove an alert",
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := openDB()
		if err != nil {
			return fmt.Errorf("open db: %w", err)
		}
		defer d.Close()

		if err := d.AlertRemove(alertRemoveTarget); err != nil {
			return fmt.Errorf("alert remove: %w", err)
		}
		fmt.Printf("Alert removed for %s\n", alertRemoveTarget)
		return nil
	},
}

func runAlertList(cmd *cobra.Command, args []string) error {
	d, err := openDB()
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer d.Close()

	alerts, err := d.AlertList()
	if err != nil {
		return fmt.Errorf("alert list: %w", err)
	}

	headers := []string{"TARGET", "LEVEL", "REASON", "CREATED"}
	rows := make([]format.Row, len(alerts))
	for i, a := range alerts {
		rows[i] = alertRow{
			target:  a.Target,
			level:   a.Level,
			reason:  a.Reason,
			created: format.Age(a.CreatedAt),
		}
	}

	output := format.Render(resolveFormat(cmd), headers, rows, isTTY())
	fmt.Println(output)
	return nil
}

func init() {
	// alert set flags
	alertSetCmd.Flags().StringVar(&alertSetTarget, "target", "", "Target: session:window or session:window.pane (required)")
	alertSetCmd.Flags().StringVar(&alertReason, "reason", "", "Alert reason text")
	alertSetCmd.Flags().StringVar(&alertLevel, "level", "info", "Alert level: info|warn|error|defer")
	alertSetCmd.MarkFlagRequired("target")

	// alert remove flags
	alertRemoveCmd.Flags().StringVar(&alertRemoveTarget, "target", "", "Target: session:window or session:window.pane (required)")
	alertRemoveCmd.MarkFlagRequired("target")

	// wire subcommands
	alertCmd.AddCommand(alertSetCmd, alertRemoveCmd, alertListCmd)

	// register with root
	rootCmd.AddCommand(alertCmd)
}
