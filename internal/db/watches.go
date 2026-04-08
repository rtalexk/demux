package db

import "fmt"

// WatchSet adds session to the watch set (upsert — safe to call twice).
func (d *DB) WatchSet(session string) error {
	_, err := d.sql.Exec(
		`INSERT INTO session_watches (session) VALUES (?) ON CONFLICT(session) DO NOTHING`,
		session,
	)
	if err != nil {
		return fmt.Errorf("watch set: %w", err)
	}
	return nil
}

// WatchClear removes session from the watch set. No-op if not present.
func (d *DB) WatchClear(session string) error {
	_, err := d.sql.Exec(`DELETE FROM session_watches WHERE session = ?`, session)
	if err != nil {
		return fmt.Errorf("watch clear: %w", err)
	}
	return nil
}

// WatchList returns all watched session names. Returns an empty (non-nil) slice when none.
func (d *DB) WatchList() ([]string, error) {
	rows, err := d.sql.Query(`SELECT session FROM session_watches ORDER BY added_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("watch list: %w", err)
	}
	defer rows.Close()

	sessions := []string{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, fmt.Errorf("watch list scan: %w", err)
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}
