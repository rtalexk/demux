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
	Short:  "Internal: reconcile slot panes against the MRU budget (called by tmux hook)",
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
		parts := strings.SplitN(strings.TrimSpace(target), ":", 2)
		if len(parts) < 2 {
			return nil
		}
		_ = s.PruneMRU()
		reserved, err := s.ComputeReservedWindows(parts[1], parts[0])
		if err != nil {
			return err
		}
		return s.ReconcileSlots(reserved, parts[1], cfg.Sidebar.Sticky.Width)
	},
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
	sidebarSlotsCmd.AddCommand(sidebarSlotsInstallCmd, sidebarSlotsUninstallCmd, sidebarSlotsEnsureCmd)
	sidebarCmd.AddCommand(sidebarShowCmd, sidebarHideCmd, sidebarToggleCmd, sidebarFollowCmd, sidebarSlotCmd, sidebarSlotsCmd)
	rootCmd.AddCommand(sidebarCmd)
}
