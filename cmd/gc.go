package cmd

import (
	"fmt"

	"github.com/rtalexk/demux/internal/db"
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

// sweepOrphanStates deletes state rows whose tmux target is no longer live.
// It returns the number of rows deleted and skipped=true when no live tmux
// targets are found (sweep skipped to avoid wiping the table during a tmux
// outage). Shared by `demux gc states` and the window/session close handlers.
func sweepOrphanStates(d *db.DB) (deleted int64, skipped bool, err error) {
	panes, err := tmux.ListPanes()
	if err != nil {
		return 0, false, fmt.Errorf("tmux not available: %w", err)
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
		return 0, true, nil
	}

	n, err := d.StateGCOrphaned(livePaneIDs, liveWindowIDs, liveSessionIDs)
	if err != nil {
		return 0, false, fmt.Errorf("gc states: %w", err)
	}
	return n, false, nil
}

// sweepOrphans is the orphan-state sweep, indirected so event-handler tests can
// stub it (the real sweep shells out to tmux, which is unavailable in tests).
var sweepOrphans = sweepOrphanStates

func runGCStates(_ *cobra.Command, _ []string) error {
	d, err := openDB()
	if err != nil {
		return err
	}
	defer d.Close()

	deleted, skipped, err := sweepOrphanStates(d)
	if err != nil {
		return err
	}
	if skipped {
		fmt.Println("No live tmux targets found; skipping GC to avoid data loss.")
		return nil
	}
	if deleted > 0 {
		fmt.Printf("Removed %d orphaned state record(s).\n", deleted)
	} else {
		fmt.Println("No orphaned state records found.")
	}
	return nil
}

func init() {
	gcCmd.AddCommand(gcStatesCmd)
	rootCmd.AddCommand(gcCmd)
}
