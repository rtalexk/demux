package db_test

import (
    "os"
    "path/filepath"
    "strings"
    "testing"
    "time"

    "github.com/rtalexk/demux/internal/db"
)

func openTestDB(t *testing.T) *db.DB {
    t.Helper()
    d, err := db.Open(":memory:")
    if err != nil {
        t.Fatal(err)
    }
    t.Cleanup(func() { d.Close() })
    return d
}

func TestAlertSetAndList(t *testing.T) {
    d := openTestDB(t)

    err := d.AlertSet("feature-auth:2", "waiting for input", "info")
    if err != nil {
        t.Fatal(err)
    }

    alerts, err := d.AlertList()
    if err != nil {
        t.Fatal(err)
    }
    if len(alerts) != 1 {
        t.Fatalf("expected 1 alert, got %d", len(alerts))
    }
    a := alerts[0]
    if a.Target != "feature-auth:2" {
        t.Errorf("unexpected target: %s", a.Target)
    }
    if a.Level != "info" {
        t.Errorf("unexpected level: %s", a.Level)
    }
}

func TestAlertSetReplacesExisting(t *testing.T) {
    d := openTestDB(t)

    d.AlertSet("s:1", "reason 1", "info")
    d.AlertSet("s:1", "reason 2", "error")

    alerts, _ := d.AlertList()
    if len(alerts) != 1 {
        t.Fatalf("expected 1, got %d", len(alerts))
    }
    if alerts[0].Reason != "reason 2" {
        t.Errorf("expected updated reason, got %s", alerts[0].Reason)
    }
}

func TestAlertSetDoesNotDowngrade(t *testing.T) {
    d := openTestDB(t)

    d.AlertSet("s:1", "reason 1", "error")
    d.AlertSet("s:1", "reason 2", "defer")

    alerts, _ := d.AlertList()
    if len(alerts) != 1 {
        t.Fatalf("expected 1, got %d", len(alerts))
    }
    if alerts[0].Level != "error" {
        t.Errorf("expected level=error to be preserved, got %s", alerts[0].Level)
    }
    if alerts[0].Reason != "reason 1" {
        t.Errorf("expected reason to be preserved, got %s", alerts[0].Reason)
    }
}

func TestAlertRemove(t *testing.T) {
    d := openTestDB(t)

    d.AlertSet("s:1", "r", "info")
    err := d.AlertRemove("s:1")
    if err != nil {
        t.Fatal(err)
    }

    alerts, _ := d.AlertList()
    if len(alerts) != 0 {
        t.Errorf("expected 0 alerts, got %d", len(alerts))
    }
}

func TestAlertCreatedAt(t *testing.T) {
    d := openTestDB(t)
    before := time.Now().UTC().Truncate(time.Second)
    d.AlertSet("s:1", "r", "warn")
    alerts, _ := d.AlertList()
    if alerts[0].CreatedAt.Before(before) {
        t.Error("created_at is before insert time")
    }
}

func TestOpen_DirPermissions(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "subdir", "state.db")
    d, err := db.Open(path)
    if err != nil {
        t.Fatal(err)
    }
    if err := d.Close(); err != nil {
        t.Fatalf("close: %v", err)
    }
    info, err := os.Stat(filepath.Dir(path))
    if err != nil {
        t.Fatal(err)
    }
    // 0700 has no bits in common with any standard umask, so the
    // kernel will not mask any bits away.
    if got := info.Mode().Perm(); got != 0700 {
        t.Errorf("dir perm = %04o, want 0700", got)
    }
}

func TestDefaultPath_ContainsExpectedSuffix(t *testing.T) {
    path, err := db.DefaultPath()
    if err != nil {
        t.Skipf("UserHomeDir not available: %v", err)
    }
    const suffix = ".local/share/demux/state.db"
    if !strings.HasSuffix(path, suffix) {
        t.Errorf("DefaultPath() = %q, want suffix %q", path, suffix)
    }
}

func TestAlertByTarget(t *testing.T) {
    d := openTestDB(t)

    // not found
    a, err := d.AlertByTarget("s:1")
    if err != nil {
        t.Fatal(err)
    }
    if a != nil {
        t.Error("expected nil for missing target")
    }

    // found
    d.AlertSet("s:1", "reason", "warn")
    a, err = d.AlertByTarget("s:1")
    if err != nil {
        t.Fatal(err)
    }
    if a == nil {
        t.Fatal("expected alert, got nil")
    }
    if a.Target != "s:1" || a.Level != "warn" {
        t.Errorf("unexpected alert: %+v", a)
    }
    if a.CreatedAt.IsZero() {
        t.Error("CreatedAt should not be zero")
    }
}
