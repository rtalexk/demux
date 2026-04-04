package db

import (
    "database/sql"
    "testing"

    _ "modernc.org/sqlite"
)

func TestMigrate_StickyColumn(t *testing.T) {
    d, err := Open(":memory:")
    if err != nil {
        t.Fatalf("open: %v", err)
    }
    defer d.Close()

    // Sticky column must exist and accept 0/1
    _, err = d.sql.Exec(`INSERT INTO alerts (target, reason, level, sticky, created_at) VALUES ('test', 'r', 'defer', 1, CURRENT_TIMESTAMP)`)
    if err != nil {
        t.Fatalf("sticky column missing: %v", err)
    }
}

func TestMigrate_UserVersion(t *testing.T) {
    d, err := Open(":memory:")
    if err != nil {
        t.Fatalf("open: %v", err)
    }
    defer d.Close()

    var version int
    if err := d.sql.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
        t.Fatalf("read user_version: %v", err)
    }
    if version != 2 {
        t.Fatalf("expected user_version=2, got %d", version)
    }
}

func TestMigrate_UpgradeV1ToV2(t *testing.T) {
    // NOTE: since :memory: DBs are per-connection we can't truly "reopen" one.
    // Instead, verify the v2 guard handles a DB that reports version=1 by
    // running migrate() directly on a DB struct.
    inner, err := sql.Open("sqlite", ":memory:")
    if err != nil {
        t.Fatalf("open raw: %v", err)
    }
    inner.Exec(`CREATE TABLE alerts (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        target TEXT NOT NULL UNIQUE,
        reason TEXT NOT NULL,
        level TEXT NOT NULL,
        created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
    )`)
    inner.Exec(`PRAGMA user_version = 1`)
    d := &DB{sql: inner}
    if err := d.migrate(); err != nil {
        t.Fatalf("migrate v1->v2: %v", err)
    }
    // sticky column should now exist
    _, err = inner.Exec(`INSERT INTO alerts (target, reason, level, sticky, created_at) VALUES ('x', 'r', 'defer', 1, CURRENT_TIMESTAMP)`)
    if err != nil {
        t.Fatalf("sticky column not added during upgrade: %v", err)
    }
    var ver int
    inner.QueryRow(`PRAGMA user_version`).Scan(&ver)
    if ver != 2 {
        t.Fatalf("expected user_version=2 after upgrade, got %d", ver)
    }
    inner.Close()
}

func TestAlertUpgradeToSticky(t *testing.T) {
    d, _ := Open(":memory:")
    defer d.Close()

    d.AlertSet("main", "r", LevelDefer, false)
    if err := d.AlertUpgradeToSticky("main"); err != nil {
        t.Fatalf("UpgradeToSticky: %v", err)
    }
    a, _ := d.AlertByTarget("main")
    if a == nil || !a.Sticky {
        t.Error("expected sticky after upgrade")
    }
}

func TestAlertRemoveIfNotSticky(t *testing.T) {
    d, _ := Open(":memory:")
    defer d.Close()

    // non-sticky: should be removed
    d.AlertSet("main", "r", LevelDefer, false)
    d.AlertRemoveIfNotSticky("main")
    a, _ := d.AlertByTarget("main")
    if a != nil {
        t.Error("expected non-sticky alert to be removed")
    }

    // sticky: should survive
    d.AlertSet("main", "r", LevelDefer, true)
    d.AlertRemoveIfNotSticky("main")
    a, _ = d.AlertByTarget("main")
    if a == nil {
        t.Error("expected sticky alert to survive")
    }
}

func TestAlertSticky_RoundTrip(t *testing.T) {
    d, err := Open(":memory:")
    if err != nil {
        t.Fatalf("open: %v", err)
    }
    defer d.Close()

    if err := d.AlertSet("main", "Come back", LevelDefer, true); err != nil {
        t.Fatalf("AlertSet: %v", err)
    }

    a, err := d.AlertByTarget("main")
    if err != nil {
        t.Fatalf("AlertByTarget: %v", err)
    }
    if a == nil {
        t.Fatal("expected alert, got nil")
    }
    if !a.Sticky {
        t.Error("expected Sticky=true")
    }

    alerts, err := d.AlertList()
    if err != nil {
        t.Fatalf("AlertList: %v", err)
    }
    if len(alerts) != 1 || !alerts[0].Sticky {
        t.Error("AlertList: expected sticky alert")
    }
}
