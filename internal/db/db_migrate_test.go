package db

import (
    "testing"
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
    d.sql.QueryRow(`PRAGMA user_version`).Scan(&version)
    if version != 2 {
        t.Fatalf("expected user_version=2, got %d", version)
    }
}
