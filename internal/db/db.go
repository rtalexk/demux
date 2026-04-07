package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type DB struct {
	sql *sql.DB
}

func Open(path string) (*DB, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return nil, fmt.Errorf("db dir: %w", err)
		}
	}
	dsn := path
	if path != ":memory:" {
		dsn = path + "?_pragma=journal_mode(WAL)"
	}
	sqldb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	sqldb.SetMaxOpenConns(1)
	if _, err := sqldb.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
		return nil, fmt.Errorf("set busy_timeout: %w", err)
	}
	d := &DB{sql: sqldb}
	if err := d.migrate(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *DB) Close() error {
	return d.sql.Close()
}

func (d *DB) migrate() error {
	var version int
	if err := d.sql.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if version < 1 {
		if err := d.migrateV1(); err != nil {
			return err
		}
		version = 1
	}
	if version < 2 {
		if err := d.migrateV2(); err != nil {
			return err
		}
		version = 2
	}
	if version < 3 {
		if err := d.migrateV3(); err != nil {
			return err
		}
	}
	return nil
}

func (d *DB) migrateV1() error {
	tx, err := d.sql.Begin()
	if err != nil {
		return fmt.Errorf("begin v1: %w", err)
	}
	if _, err := tx.Exec(`
            CREATE TABLE IF NOT EXISTS alerts (
                id         INTEGER PRIMARY KEY AUTOINCREMENT,
                target     TEXT NOT NULL UNIQUE,
                reason     TEXT NOT NULL,
                level      TEXT NOT NULL,
                created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
            )
        `); err != nil {
		tx.Rollback()
		return fmt.Errorf("migrate v1: %w", err)
	}
	if _, err := tx.Exec(`PRAGMA user_version = 1`); err != nil {
		tx.Rollback()
		return fmt.Errorf("set version 1: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit v1: %w", err)
	}
	return nil
}

func (d *DB) migrateV2() error {
	tx, err := d.sql.Begin()
	if err != nil {
		return fmt.Errorf("begin v2: %w", err)
	}

	// Check if sticky column already exists (handles partial migrations).
	rows, err := tx.Query(`PRAGMA table_info(alerts)`)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("table info v2: %w", err)
	}
	var hasSticky bool
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var dfltVal sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dfltVal, &pk); err == nil && name == "sticky" {
			hasSticky = true
		}
	}
	rows.Close()

	if !hasSticky {
		if _, err := tx.Exec(`ALTER TABLE alerts ADD COLUMN sticky BOOLEAN NOT NULL DEFAULT 0`); err != nil {
			tx.Rollback()
			return fmt.Errorf("migrate v2: %w", err)
		}
	}
	if _, err := tx.Exec(`PRAGMA user_version = 2`); err != nil {
		tx.Rollback()
		return fmt.Errorf("set version 2: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit v2: %w", err)
	}
	return nil
}

func (d *DB) migrateV3() error {
	tx, err := d.sql.Begin()
	if err != nil {
		return fmt.Errorf("begin v3: %w", err)
	}
	if _, err := tx.Exec(`DROP TABLE IF EXISTS alerts`); err != nil {
		tx.Rollback()
		return fmt.Errorf("drop alerts v3: %w", err)
	}
	if _, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS tool_states (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			target     TEXT NOT NULL UNIQUE,
			tool       TEXT NOT NULL DEFAULT '',
			value      INTEGER NOT NULL,
			message    TEXT NOT NULL DEFAULT '',
			source     INTEGER NOT NULL DEFAULT 1,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		tx.Rollback()
		return fmt.Errorf("create tool_states v3: %w", err)
	}
	if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_tool_states_value ON tool_states(value)`); err != nil {
		tx.Rollback()
		return fmt.Errorf("index value v3: %w", err)
	}
	if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_tool_states_updated ON tool_states(updated_at)`); err != nil {
		tx.Rollback()
		return fmt.Errorf("index updated v3: %w", err)
	}
	if _, err := tx.Exec(`PRAGMA user_version = 3`); err != nil {
		tx.Rollback()
		return fmt.Errorf("set version 3: %w", err)
	}
	return tx.Commit()
}

// DefaultPath returns ~/.local/share/demux/state.db, or an error if the
// home directory cannot be determined.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".local", "share", "demux", "state.db"), nil
}
