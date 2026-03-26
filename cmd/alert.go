package cmd

import (
	"fmt"

	"github.com/rtalex/demux/internal/format"
	"github.com/spf13/cobra"
)

var (
	alertSetTarget    string
	alertRemoveTarget string
	alertReason       string
	alertLevel        string
	alertSticky       bool
)

type alertRow struct {
	target  string
	level   string
	reason  string
	sticky  string
	created string
}

func (r alertRow) Fields() []string {
	return []string{r.target, r.level, r.reason, r.sticky, r.created}
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

		if err := d.AlertSet(alertSetTarget, alertReason, alertLevel, alertSticky); err != nil {
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

	headers := []string{"TARGET", "LEVEL", "REASON", "STICKY", "CREATED"}
	rows := make([]format.Row, len(alerts))
	for i, a := range alerts {
		sticky := "false"
		if a.Sticky {
			sticky = "true"
		}
		rows[i] = alertRow{
			target:  a.Target,
			level:   a.Level,
			reason:  a.Reason,
			sticky:  sticky,
			created: formatAge(a.CreatedAt),
		}
	}

	output := format.Render(resolveFormat(cmd), headers, rows, isTTY())
	fmt.Println(output)
	return nil
}

func init() {
	// alert set flags
	alertSetCmd.Flags().StringVar(&alertSetTarget, "target", "", "Session:window target (required)")
	alertSetCmd.Flags().StringVar(&alertReason, "reason", "", "Alert reason text (required)")
	alertSetCmd.Flags().StringVar(&alertLevel, "level", "info", "Alert level: info|warn|error")
	alertSetCmd.Flags().BoolVar(&alertSticky, "sticky", false, "Make alert sticky")
	alertSetCmd.MarkFlagRequired("target")
	alertSetCmd.MarkFlagRequired("reason")

	// alert remove flags
	alertRemoveCmd.Flags().StringVar(&alertRemoveTarget, "target", "", "Session:window target (required)")
	alertRemoveCmd.MarkFlagRequired("target")

	// wire subcommands
	alertCmd.AddCommand(alertSetCmd, alertRemoveCmd, alertListCmd)

	// register with root
	rootCmd.AddCommand(alertCmd)
}
