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
        dsn = path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
    }
    sqldb, err := sql.Open("sqlite", dsn)
    if err != nil {
        return nil, err
    }
    sqldb.SetMaxOpenConns(1)
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
        version = 1
    }

    if version < 2 {
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
    }

    return nil
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
