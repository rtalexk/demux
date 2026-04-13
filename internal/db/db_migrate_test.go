package db

import (
	"testing"
)

func TestMigrateV3_CreatesToolStates(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	_, err = d.sql.Exec(`INSERT INTO tool_states (target, tool, value, message, source) VALUES ('s:0:0', 'claude', 1, 'running', 1)`)
	if err != nil {
		t.Fatalf("tool_states table missing after migration: %v", err)
	}
}

func TestMigrateV3_AlertsTableGone(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	_, err = d.sql.Exec(`INSERT INTO alerts (target, reason, level) VALUES ('x', 'r', 'info')`)
	if err == nil {
		t.Error("alerts table should not exist after v3 migration")
	}
}

func TestMigrateV6_AddsPaneIDColumn(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	ok, err := tableHasColumn(d.sql, "tool_states", "pane_id")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("pane_id column not found in tool_states after migration")
	}
}

func TestMigrateV3_UserVersion(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	var version int
	if err := d.sql.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != 6 {
		t.Fatalf("expected user_version=6, got %d", version)
	}
}
