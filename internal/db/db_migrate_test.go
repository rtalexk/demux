package db

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigrateV8_ToolStatesTableCreated(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	_, err = d.sql.Exec(`INSERT INTO tool_states (target_type, target_id, tool, value, message, source) VALUES ('pane', '%1', 'claude', 1, 'running', 1)`)
	if err != nil {
		t.Fatalf("tool_states table missing after migration: %v", err)
	}
}

func TestMigrateV3_AlertsTableGone(t *testing.T) { // v3 drops alerts; verified against final schema
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

func TestMigrateV8_PaneIDColumnPresent(t *testing.T) {
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

func TestMigrateV8_UserVersion(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	var version int
	if err := d.sql.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != 8 {
		t.Fatalf("expected user_version=8, got %d", version)
	}
}

func TestMigrateV8_TypeIDIndexIsUnique(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	// Insert two records with the same (target_type, target_id) — must fail.
	_, err1 := d.sql.Exec(`INSERT INTO tool_states (target_type, target_id, tool, value, message, source) VALUES ('pane', '%1', 'c', 1, '', 1)`)
	_, err2 := d.sql.Exec(`INSERT INTO tool_states (target_type, target_id, tool, value, message, source) VALUES ('pane', '%1', 'c', 1, '', 1)`)
	if err1 != nil {
		t.Fatalf("first insert should succeed: %v", err1)
	}
	if err2 == nil {
		t.Error("second insert with same (target_type, target_id) should fail (UNIQUE constraint)")
	}
}

// TestMigrateV7ToV8_DropsOldRowsAndUpgradesSchema verifies the upgrade path from
// a real v7 database: old string-target rows are discarded and the new schema
// (target_type, target_id) is installed correctly.
func TestMigrateV7ToV8_DropsOldRowsAndUpgradesSchema(t *testing.T) {
	rawDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}

	// Build v7 schema manually.
	v7stmts := []string{
		`CREATE TABLE tool_states (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			target     TEXT NOT NULL UNIQUE,
			tool       TEXT NOT NULL DEFAULT '',
			value      INTEGER NOT NULL,
			message    TEXT NOT NULL DEFAULT '',
			source     INTEGER NOT NULL DEFAULT 1,
			pane_id    TEXT,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE UNIQUE INDEX idx_tool_states_pane_id ON tool_states(pane_id) WHERE pane_id IS NOT NULL`,
		`CREATE INDEX idx_tool_states_value ON tool_states(value)`,
		`CREATE TABLE session_watches (
			session  TEXT NOT NULL PRIMARY KEY,
			added_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE session_items (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			session    TEXT NOT NULL,
			kind       TEXT NOT NULL,
			body       TEXT NOT NULL,
			checked    INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`PRAGMA user_version = 7`,
	}
	for _, stmt := range v7stmts {
		if _, err := rawDB.Exec(stmt); err != nil {
			t.Fatalf("setup v7 schema (%q): %v", stmt[:30], err)
		}
	}

	// Insert a v7-format row.
	if _, err := rawDB.Exec(
		`INSERT INTO tool_states (target, tool, value, message, source, pane_id) VALUES ('mysession:1.0', 'claude', 1, 'working', 1, '%5')`,
	); err != nil {
		t.Fatalf("insert v7 row: %v", err)
	}

	// Run migration via the DB wrapper (migrate() is package-internal).
	d := &DB{sql: rawDB}
	if err := d.migrate(); err != nil {
		t.Fatalf("migrate() failed: %v", err)
	}

	// user_version must be 8.
	var version int
	if err := rawDB.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != 8 {
		t.Fatalf("expected user_version=8, got %d", version)
	}

	// Old v7 row must be gone.
	var count int
	if err := rawDB.QueryRow(`SELECT COUNT(*) FROM tool_states`).Scan(&count); err != nil {
		t.Fatalf("count tool_states: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 rows after v8 migration, got %d", count)
	}

	// New schema must accept a v8-format insert.
	if _, err := rawDB.Exec(
		`INSERT INTO tool_states (target_type, target_id, session_id, pane_id, tool, value, message, source) VALUES ('pane', '%5', '$1', '%5', 'claude', 1, 'working', 1)`,
	); err != nil {
		t.Errorf("v8 insert failed after migration: %v", err)
	}
}

func TestMigrateV8_HasIndexes(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	indexes := []string{
		"idx_tool_states_type_id",
		"idx_tool_states_session",
		"idx_tool_states_window",
		"idx_tool_states_value",
	}
	for _, idxName := range indexes {
		var name string
		err := d.sql.QueryRow(`SELECT name FROM sqlite_master WHERE type='index' AND name=?`, idxName).Scan(&name)
		if err != nil {
			t.Errorf("index %q not found: %v", idxName, err)
		}
	}
}
