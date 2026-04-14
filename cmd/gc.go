package cmd

import (
	"fmt"

	"github.com/rtalexk/demux/internal/tmux"
	"github.com/spf13/cobra"
)

var gcCmd = &cobra.Command{
	Use:   "gc",
	Short: "Garbage collect stale records",
}

var gcStatesCmd = &cobra.Command{
	Use:   "states",
	Short: "Remove orphaned state records for closed tmux panes",
	RunE:  runGCStates,
}

func runGCStates(_ *cobra.Command, _ []string) error {
	panes, err := tmux.ListPanes()
	if err != nil {
		return fmt.Errorf("tmux not available: %w", err)
	}

	livePaneIDs := make(map[string]bool, len(panes))
	liveWindowIDs := make(map[string]bool)
	liveSessionIDs := make(map[string]bool)
	for _, p := range panes {
		if p.PaneID != "" {
			livePaneIDs[p.PaneID] = true
		}
		if p.WindowID != "" {
			liveWindowIDs[p.WindowID] = true
		}
		if p.SessionID != "" {
			liveSessionIDs[p.SessionID] = true
		}
	}

	if len(livePaneIDs) == 0 && len(liveWindowIDs) == 0 && len(liveSessionIDs) == 0 {
		fmt.Println("No live tmux targets found; skipping GC to avoid data loss.")
		return nil
	}

	d, err := openDB()
	if err != nil {
		return err
	}
	defer d.Close()

	n, err := d.StateGCOrphaned(livePaneIDs, liveWindowIDs, liveSessionIDs)
	if err != nil {
		return fmt.Errorf("gc states: %w", err)
	}
	if n > 0 {
		fmt.Printf("Removed %d orphaned state record(s).\n", n)
	} else {
		fmt.Println("No orphaned state records found.")
	}
	return nil
}

func init() {
	gcCmd.AddCommand(gcStatesCmd)
	rootCmd.AddCommand(gcCmd)
}
