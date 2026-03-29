package cmd

import (
    "testing"

    "github.com/rtalex/demux/internal/db"
)

func TestWindowTargetFromPane_WithPane(t *testing.T) {
    got := windowTargetFromPane("main:1.2")
    want := "main:1"
    if got != want {
        t.Errorf("windowTargetFromPane(%q) = %q, want %q", "main:1.2", got, want)
    }
}

func TestWindowTargetFromPane_NoDot(t *testing.T) {
    got := windowTargetFromPane("main:1")
    want := "main:1"
    if got != want {
        t.Errorf("windowTargetFromPane(%q) = %q, want %q", "main:1", got, want)
    }
}

func TestPaneFocusClearsAlertsIncludingSticky(t *testing.T) {
    d, err := db.Open(":memory:")
    if err != nil {
        t.Fatal(err)
    }
    defer d.Close()

    if err := d.AlertSet("work:2.3", "Claude finished", "info", false); err != nil {
        t.Fatal(err)
    }
    if err := d.AlertSet("work:2", "needs attention", "warn", true); err != nil {
        t.Fatal(err)
    }

    // confirm both alerts exist before clearing
    seeded, err := d.AlertList()
    if err != nil {
        t.Fatal(err)
    }
    if len(seeded) != 2 {
        t.Fatalf("expected 2 seeded alerts, got %d", len(seeded))
    }

    if err := applyPaneFocus(d, "work:2.3"); err != nil {
        t.Fatal(err)
    }

    alerts, err := d.AlertList()
    if err != nil {
        t.Fatal(err)
    }
    if len(alerts) != 0 {
        t.Errorf("expected 0 alerts after pane_focus, got %d: %+v", len(alerts), alerts)
    }
}

func TestPaneFocusNoAlertsIsNoop(t *testing.T) {
    d, err := db.Open(":memory:")
    if err != nil {
        t.Fatal(err)
    }
    defer d.Close()

    if err := applyPaneFocus(d, "work:2.3"); err != nil {
        t.Fatalf("unexpected error on empty db: %v", err)
    }
}
