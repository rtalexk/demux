package tui

import (
    "strings"
    "testing"
    "time"

    "github.com/rtalexk/demux/internal/config"
    "github.com/rtalexk/demux/internal/db"
    "github.com/rtalexk/demux/internal/tmux"
)

// ── overlayCenter ─────────────────────────────────────────────────────────────

func TestOverlayCenter_FgPlacedAtCenter(t *testing.T) {
    // bg: 10 cols × 5 rows, fg: 3 cols × 1 row
    // startX=(10-3)/2=3, startY=(5-1)/2=2 → overlay lands on line 2
    bg := "1234567890\nabcdefghij\nABCDEFGHIJ\n0987654321\nzyxwvutsrq"
    result := overlayCenter("XXX", bg, 10, 5)
    lines := strings.Split(result, "\n")
    if lines[2] != "ABCXXXGHIJ" {
        t.Errorf("line 2: expected ABCXXXGHIJ, got %q", lines[2])
    }
}

func TestOverlayCenter_BackgroundLinesOutsideRangeUnchanged(t *testing.T) {
    bg := "1234567890\nabcdefghij\nABCDEFGHIJ\n0987654321\nzyxwvutsrq"
    result := overlayCenter("XXX", bg, 10, 5)
    lines := strings.Split(result, "\n")
    if lines[0] != "1234567890" {
        t.Errorf("line 0 should be unchanged: %q", lines[0])
    }
    if lines[4] != "zyxwvutsrq" {
        t.Errorf("line 4 should be unchanged: %q", lines[4])
    }
}

func TestOverlayCenter_MultiLineFg(t *testing.T) {
    // bg: 10 cols × 3 rows, fg: 3 cols × 2 rows
    // startX=(10-3)/2=3, startY=(3-2)/2=0 → lines 0 and 1 get overlaid
    bg := "1234567890\nabcdefghij\nABCDEFGHIJ"
    result := overlayCenter("XXX\nYYY", bg, 10, 3)
    lines := strings.Split(result, "\n")
    if lines[0] != "123XXX7890" {
        t.Errorf("line 0: expected 123XXX7890, got %q", lines[0])
    }
    if lines[1] != "abcYYYghij" {
        t.Errorf("line 1: expected abcYYYghij, got %q", lines[1])
    }
    if lines[2] != "ABCDEFGHIJ" {
        t.Errorf("line 2 should be unchanged: %q", lines[2])
    }
}

func TestOverlayCenter_FgWiderThanBgClampsStartXToZero(t *testing.T) {
    // fg (12 cols) wider than bg (5 cols): startX clamped to 0, not negative
    bg := "abcde\nfghij"
    result := overlayCenter("XXXXXXXXXXXX", bg, 5, 2)
    lines := strings.Split(result, "\n")
    // line 0 (startY=0): fg replaces the entire line content
    if !strings.Contains(lines[0], "XXX") {
        t.Errorf("expected fg content at line 0 when fg wider than bg, got: %q", lines[0])
    }
    if lines[1] != "fghij" {
        t.Errorf("line 1 should be unchanged: %q", lines[1])
    }
}

func TestOverlayCenter_FgTallerThanBgClampsStartYToZero(t *testing.T) {
    // fg (3 rows) taller than bg (1 row): startY clamped to 0
    // Only fgLines[0] overlaid on the single bg line
    // startX=(10-1)/2=4; "background"[0:4]+"X"+"background"[5:]="backXround"
    result := overlayCenter("X\nY\nZ", "background", 10, 1)
    lines := strings.Split(result, "\n")
    if len(lines) != 1 {
        t.Fatalf("expected 1 output line, got %d", len(lines))
    }
    if lines[0] != "backXround" {
        t.Errorf("expected backXround, got %q", lines[0])
    }
}

// ── pruneStaleAlerts ──────────────────────────────────────────────────────────

func newPruneTestModel(t *testing.T) Model {
    t.Helper()
    d, err := db.Open(":memory:")
    if err != nil {
        t.Fatal(err)
    }
    t.Cleanup(func() { d.Close() })
    return New(config.Default(), d)
}

func TestPruneStaleAlerts_NilWhenNoPanes(t *testing.T) {
    m := newPruneTestModel(t)
    m.panes = nil
    m.alerts = []db.Alert{{Target: "work:0.0", CreatedAt: time.Now()}}
    if cmd := m.pruneStaleAlerts(); cmd != nil {
        t.Error("expected nil when panes list is empty")
    }
}

func TestPruneStaleAlerts_NilWhenAllAlertsAreLive(t *testing.T) {
    m := newPruneTestModel(t)
    m.panes = []tmux.Pane{{Session: "work", WindowIndex: 0, PaneIndex: 0}}
    m.alerts = []db.Alert{
        {Target: "work:0.0", CreatedAt: time.Now()}, // live pane
        {Target: "work:0", CreatedAt: time.Now()},   // live window
        {Target: "work", CreatedAt: time.Now()},     // live session
    }
    if cmd := m.pruneStaleAlerts(); cmd != nil {
        t.Error("expected nil when all alerts reference live targets")
    }
}

func TestPruneStaleAlerts_PrunesStalePaneTarget(t *testing.T) {
    m := newPruneTestModel(t)
    m.db.AlertSet("gone:9.9", "r", "warn")
    m.panes = []tmux.Pane{{Session: "work", WindowIndex: 0, PaneIndex: 0}}
    alerts, _ := m.db.AlertList()
    m.alerts = alerts

    cmd := m.pruneStaleAlerts()
    if cmd == nil {
        t.Fatal("expected non-nil cmd for stale pane-level alert")
    }
    msg := cmd().(alertsMsg)
    if len(msg.alerts) != 0 {
        t.Errorf("expected stale pane alert removed; %d remain", len(msg.alerts))
    }
}

func TestPruneStaleAlerts_PrunesStaleWindowTarget(t *testing.T) {
    m := newPruneTestModel(t)
    m.db.AlertSet("gone:9", "r", "warn")
    m.panes = []tmux.Pane{{Session: "work", WindowIndex: 0, PaneIndex: 0}}
    alerts, _ := m.db.AlertList()
    m.alerts = alerts

    cmd := m.pruneStaleAlerts()
    if cmd == nil {
        t.Fatal("expected non-nil cmd for stale window-level alert")
    }
    msg := cmd().(alertsMsg)
    if len(msg.alerts) != 0 {
        t.Errorf("expected stale window alert removed; %d remain", len(msg.alerts))
    }
}

func TestPruneStaleAlerts_PrunesStaleSessionTarget(t *testing.T) {
    m := newPruneTestModel(t)
    m.db.AlertSet("gone-session", "r", "warn")
    m.panes = []tmux.Pane{{Session: "work", WindowIndex: 0, PaneIndex: 0}}
    alerts, _ := m.db.AlertList()
    m.alerts = alerts

    cmd := m.pruneStaleAlerts()
    if cmd == nil {
        t.Fatal("expected non-nil cmd for stale session-level alert")
    }
    msg := cmd().(alertsMsg)
    if len(msg.alerts) != 0 {
        t.Errorf("expected stale session alert removed; %d remain", len(msg.alerts))
    }
}

func TestPruneStaleAlerts_PreservesLiveAlerts(t *testing.T) {
    m := newPruneTestModel(t)
    m.db.AlertSet("work:0.0", "live pane", "info")
    m.db.AlertSet("gone:9.9", "stale pane", "warn")
    m.panes = []tmux.Pane{{Session: "work", WindowIndex: 0, PaneIndex: 0}}
    alerts, _ := m.db.AlertList()
    m.alerts = alerts

    cmd := m.pruneStaleAlerts()
    if cmd == nil {
        t.Fatal("expected non-nil cmd (one stale alert present)")
    }
    msg := cmd().(alertsMsg)
    if len(msg.alerts) != 1 {
        t.Fatalf("expected 1 alert to survive, got %d", len(msg.alerts))
    }
    if msg.alerts[0].Target != "work:0.0" {
        t.Errorf("expected live alert work:0.0 to survive, got: %q", msg.alerts[0].Target)
    }
}
