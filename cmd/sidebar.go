package cmd

import (
	"fmt"

	"github.com/rtalexk/demux/internal/sticky"
	"github.com/spf13/cobra"
)

// stickyClient is overridable in tests so we can inject a fake Tmux.
var stickyClient = func() *sticky.Sticky { return sticky.New() }

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
		return stickyClient().Show(sidebarShowOpts())
	},
}

var sidebarHideCmd = &cobra.Command{
	Use:   "hide",
	Short: "Kill the sticky sidebar pane if it is currently shown",
	RunE: func(cmd *cobra.Command, args []string) error {
		return stickyClient().Hide()
	},
}

var sidebarToggleCmd = &cobra.Command{
	Use:   "toggle",
	Short: "Show the sticky sidebar pane if hidden, hide it if shown",
	RunE: func(cmd *cobra.Command, args []string) error {
		return stickyClient().Toggle(sidebarShowOpts())
	},
}

var sidebarFollowCmd = &cobra.Command{
	Use:    "follow",
	Short:  "Internal: move the sticky sidebar pane to the current session",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := stickyClient().Follow(); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "demux sidebar follow: %v\n", err)
		}
		return nil
	},
}

func init() {
	sidebarCmd.AddCommand(sidebarShowCmd, sidebarHideCmd, sidebarToggleCmd, sidebarFollowCmd)
	rootCmd.AddCommand(sidebarCmd)
}
