package db

import (
	"fmt"
	"time"
)

// Item kind constants.
const (
	KindTodo = "todo"
	KindNote = "note"
)

// Item represents a single checklist item for a session.
// Kind is either KindTodo (checkable) or KindNote (informational, never checked).
type Item struct {
	ID        int64
	Session   string
	Kind      string
	Body      string
	Checked   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ItemAdd appends a new item of the given kind to the session.
func (d *DB) ItemAdd(session, kind, body string) error {
	_, err := d.sql.Exec(
		`INSERT INTO session_items (session, kind, body) VALUES (?, ?, ?)`,
		session, kind, body,
	)
	if err != nil {
		return fmt.Errorf("item add: %w", err)
	}
	return nil
}

// ItemList returns all items for a session ordered by created_at ascending.
// Returns an empty (non-nil) slice when none.
func (d *DB) ItemList(session string) ([]Item, error) {
	rows, err := d.sql.Query(
		`SELECT id, session, kind, body, checked, created_at, updated_at
		 FROM session_items WHERE session = ? ORDER BY created_at ASC`,
		session,
	)
	if err != nil {
		return nil, fmt.Errorf("item list: %w", err)
	}
	defer rows.Close()

	items := []Item{}
	for rows.Next() {
		var it Item
		var checked int
		if err := rows.Scan(&it.ID, &it.Session, &it.Kind, &it.Body, &checked, &it.CreatedAt, &it.UpdatedAt); err != nil {
			return nil, fmt.Errorf("item list scan: %w", err)
		}
		it.Checked = checked != 0
		items = append(items, it)
	}
	return items, rows.Err()
}

// ItemUpdate replaces the body of the item with the given id.
func (d *DB) ItemUpdate(id int64, body string) error {
	_, err := d.sql.Exec(
		`UPDATE session_items SET body = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		body, id,
	)
	if err != nil {
		return fmt.Errorf("item update: %w", err)
	}
	return nil
}

// ItemToggle flips the checked state of a todo item.
// It is a no-op for notes (kind = 'note').
func (d *DB) ItemToggle(id int64) error {
	_, err := d.sql.Exec(
		`UPDATE session_items SET checked = NOT checked, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND kind = 'todo'`,
		id,
	)
	if err != nil {
		return fmt.Errorf("item toggle: %w", err)
	}
	return nil
}

// ItemDelete removes the item with the given id.
func (d *DB) ItemDelete(id int64) error {
	_, err := d.sql.Exec(`DELETE FROM session_items WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("item delete: %w", err)
	}
	return nil
}

// ItemDeleteSession removes all items for the given session.
func (d *DB) ItemDeleteSession(session string) error {
	_, err := d.sql.Exec(`DELETE FROM session_items WHERE session = ?`, session)
	if err != nil {
		return fmt.Errorf("item delete session: %w", err)
	}
	return nil
}

// ItemListOrphaned returns items keyed by session name for sessions not in activeSessions.
func (d *DB) ItemListOrphaned(activeSessions []string) (map[string][]Item, error) {
	active := make(map[string]struct{}, len(activeSessions))
	for _, s := range activeSessions {
		active[s] = struct{}{}
	}

	rows, err := d.sql.Query(`SELECT DISTINCT session FROM session_items`)
	if err != nil {
		return nil, fmt.Errorf("item orphaned sessions: %w", err)
	}
	var allSessions []string
	for rows.Next() {
		var sess string
		if err := rows.Scan(&sess); err != nil {
			rows.Close()
			return nil, fmt.Errorf("item orphaned scan: %w", err)
		}
		allSessions = append(allSessions, sess)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("item orphaned rows close: %w", err)
	}

	result := map[string][]Item{}
	for _, sess := range allSessions {
		if _, ok := active[sess]; ok {
			continue
		}
		items, err := d.ItemList(sess)
		if err != nil {
			return nil, err
		}
		result[sess] = items
	}
	return result, nil
}

// ItemSessionsWithOpen returns session names that have at least one unchecked todo.
func (d *DB) ItemSessionsWithOpen() ([]string, error) {
	rows, err := d.sql.Query(
		`SELECT DISTINCT session FROM session_items
		 WHERE kind = 'todo' AND checked = 0 ORDER BY session ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("item sessions with open: %w", err)
	}
	defer rows.Close()

	sessions := []string{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, fmt.Errorf("item sessions with open scan: %w", err)
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}
