package cmd

import (
	"fmt"
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

var sidebarSlotsCmd = &cobra.Command{
	Use:   "slots",
	Short: "Manage per-window sidebar slots (sidebar.sticky.slots mode)",
}

var sidebarSlotsInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Create a sidebar slot pane in every existing window",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := loadConfig()
		return stickyClient().InstallSlotsInAllWindows(cfg.Sidebar.Sticky.Width)
	},
}

var sidebarSlotsUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Kill every sidebar slot pane",
	RunE: func(cmd *cobra.Command, args []string) error {
		return stickyClient().UninstallSlots()
	},
}

var sidebarSlotsEnsureCmd = &cobra.Command{
	Use:    "ensure",
	Short:  "Internal: ensure the current window has a sidebar slot (called by tmux hook)",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
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
		return s.EnsureSlotInWindow(strings.TrimSpace(target), cfg.Sidebar.Sticky.Width)
	},
}

var sidebarSlotCmd = &cobra.Command{
	Use:    "slot",
	Short:  "Internal: placeholder process for an inactive sticky sidebar slot",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		signal.Ignore(syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
		fmt.Print("\x1b[2J\x1b[H\n\n")
		fmt.Println("  demux sidebar slot")
		fmt.Println()
		fmt.Println("  Reserved by [sidebar.sticky] slots.")
		fmt.Println("  Activate this window's slot with")
		fmt.Println("  `demux sidebar show`.")
		select {}
	},
}

func init() {
	sidebarSlotsCmd.AddCommand(sidebarSlotsInstallCmd, sidebarSlotsUninstallCmd, sidebarSlotsEnsureCmd)
	sidebarCmd.AddCommand(sidebarShowCmd, sidebarHideCmd, sidebarToggleCmd, sidebarFollowCmd, sidebarSlotCmd, sidebarSlotsCmd)
	rootCmd.AddCommand(sidebarCmd)
}
