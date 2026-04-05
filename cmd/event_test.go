package cmd

import (
    "testing"

    "github.com/rtalexk/demux/internal/db"
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

func TestWindowTargetFromPane_NamedWindowWithDots(t *testing.T) {
    // session name contains a dot; LastIndex correctly strips only the pane suffix
    got := windowTargetFromPane("a.b:c.0")
    want := "a.b:c"
    if got != want {
        t.Errorf("windowTargetFromPane(%q) = %q, want %q", "a.b:c.0", got, want)
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
    if err := d.AlertSet("work:2", "needs attention", "warn", false); err != nil {
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

func TestApplyPaneFocus_SkipsSticky(t *testing.T) {
    d, _ := db.Open(":memory:")
    defer d.Close()

    // Set a sticky defer on the pane target
    d.AlertSet("myses:0.1", "Come back", db.LevelDefer, true)
    // Set a non-sticky alert on the window
    d.AlertSet("myses:0", "note", db.LevelInfo, false)

    if err := applyPaneFocus(d, "myses:0.1"); err != nil {
        t.Fatalf("applyPaneFocus: %v", err)
    }

    // Sticky pane alert should survive
    a, _ := d.AlertByTarget("myses:0.1")
    if a == nil {
        t.Error("sticky pane alert should not have been cleared")
    }

    // Non-sticky window alert should be gone
    b, _ := d.AlertByTarget("myses:0")
    if b != nil {
        t.Error("non-sticky window alert should have been cleared")
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

func TestApplyPaneFocusClearsSessionLevel(t *testing.T) {
    d, err := db.Open(":memory:")
    if err != nil {
        t.Fatal(err)
    }
    defer d.Close()

    if err := d.AlertSet("main", "come back", "defer", false); err != nil {
        t.Fatalf("seed: %v", err)
    }
    if err := d.AlertSet("main:0", "window alert", "info", false); err != nil {
        t.Fatalf("seed: %v", err)
    }
    if err := d.AlertSet("main:0.0", "pane alert", "warn", false); err != nil {
        t.Fatalf("seed: %v", err)
    }

    seeded, err := d.AlertList()
    if err != nil {
        t.Fatal(err)
    }
    if len(seeded) != 3 {
        t.Fatalf("expected 3 seeded alerts, got %d", len(seeded))
    }

    if err := applyPaneFocus(d, "main:0.0"); err != nil {
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
