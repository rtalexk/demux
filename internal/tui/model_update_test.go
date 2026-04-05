package tui

import (
    "testing"
    "time"

    "github.com/rtalexk/demux/internal/config"
    "github.com/rtalexk/demux/internal/db"
    "github.com/rtalexk/demux/internal/tmux"
)

func newPruneModel() Model {
    cfg := config.Default()
    database, _ := db.Open(":memory:")
    return New(cfg, database)
}

// TestPruneStaleAlerts_skipsSticky verifies that pruneStaleAlerts does not
// collect sticky alerts for removal, even when their target is absent from the
// live pane set.
func TestPruneStaleAlerts_skipsSticky(t *testing.T) {
    m := newPruneModel()

    // Populate panes: only "alpha:0" is live.
    m.panes = []tmux.Pane{
        {Session: "alpha", WindowIndex: 0, PaneIndex: 0},
    }

    // Set up two alerts for a window that is NOT in the live pane set:
    //   - one regular (non-sticky) defer alert
    //   - one sticky defer alert
    staleTarget := "beta:0"
    m.alerts = []db.Alert{
        {Target: staleTarget, Level: db.LevelDefer, Sticky: false, CreatedAt: time.Now()},
        {Target: "beta:1", Level: db.LevelDefer, Sticky: true, CreatedAt: time.Now()},
    }

    cmd := m.pruneStaleAlerts()
    if cmd == nil {
        t.Fatal("expected a prune cmd (stale non-sticky alert exists), got nil")
    }

    // Execute the cmd to capture which targets were removed.
    // We seed the in-memory DB with both alerts and then run the cmd.
    if err := m.db.AlertSet(staleTarget, "test", db.LevelDefer, false); err != nil {
        t.Fatalf("seed non-sticky alert: %v", err)
    }
    stickyTarget := "beta:1"
    if err := m.db.AlertSet(stickyTarget, "test", db.LevelDefer, true); err != nil {
        t.Fatalf("seed sticky alert: %v", err)
    }

    msg := cmd()
    result, ok := msg.(alertsMsg)
    if !ok {
        t.Fatalf("expected alertsMsg, got %T", msg)
    }

    // The sticky alert must still be present in the returned list.
    foundSticky := false
    for _, a := range result.alerts {
        if a.Target == stickyTarget {
            foundSticky = true
        }
        if a.Target == staleTarget {
            t.Errorf("non-sticky stale alert %q should have been pruned but was returned", staleTarget)
        }
    }
    if !foundSticky {
        t.Errorf("sticky alert %q should NOT have been pruned but was missing from result", stickyTarget)
    }
}

// TestPruneStaleAlerts_noOpWhenAllSticky verifies that pruneStaleAlerts returns
// nil (no-op) when the only stale alert is sticky.
func TestPruneStaleAlerts_noOpWhenAllSticky(t *testing.T) {
    m := newPruneModel()

    m.panes = []tmux.Pane{
        {Session: "alpha", WindowIndex: 0, PaneIndex: 0},
    }

    // Only a sticky alert for a target not in the live pane set.
    m.alerts = []db.Alert{
        {Target: "gone:0", Level: db.LevelDefer, Sticky: true, CreatedAt: time.Now()},
    }

    cmd := m.pruneStaleAlerts()
    if cmd != nil {
        t.Error("expected nil cmd when only stale alert is sticky, but got a cmd")
    }
}
