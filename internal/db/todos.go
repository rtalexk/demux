package db

import (
	"fmt"
	"time"
)

// Todo represents a single checklist item for a session.
type Todo struct {
	ID        int64
	Session   string
	Position  int
	Body      string
	Checked   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TodoAdd appends a new unchecked item to the session's checklist.
func (d *DB) TodoAdd(session, body string) error {
	_, err := d.sql.Exec(`
		INSERT INTO session_todos (session, position, body)
		VALUES (?, COALESCE((SELECT MAX(position) + 1 FROM session_todos WHERE session = ?), 0), ?)
	`, session, session, body)
	if err != nil {
		return fmt.Errorf("todo add: %w", err)
	}
	return nil
}

// TodoList returns all items for a session ordered by position ascending.
// Returns an empty (non-nil) slice when none.
func (d *DB) TodoList(session string) ([]Todo, error) {
	rows, err := d.sql.Query(
		`SELECT id, session, position, body, checked, created_at, updated_at
		 FROM session_todos WHERE session = ? ORDER BY position ASC`,
		session,
	)
	if err != nil {
		return nil, fmt.Errorf("todo list: %w", err)
	}
	defer rows.Close()

	items := []Todo{}
	for rows.Next() {
		var t Todo
		var checked int
		if err := rows.Scan(&t.ID, &t.Session, &t.Position, &t.Body, &checked, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("todo list scan: %w", err)
		}
		t.Checked = checked != 0
		items = append(items, t)
	}
	return items, rows.Err()
}

// TodoUpdate replaces the body of the item with the given id.
func (d *DB) TodoUpdate(id int64, body string) error {
	_, err := d.sql.Exec(
		`UPDATE session_todos SET body = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		body, id,
	)
	if err != nil {
		return fmt.Errorf("todo update: %w", err)
	}
	return nil
}

// TodoToggle flips the checked state of the item with the given id.
func (d *DB) TodoToggle(id int64) error {
	_, err := d.sql.Exec(
		`UPDATE session_todos SET checked = NOT checked, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		id,
	)
	if err != nil {
		return fmt.Errorf("todo toggle: %w", err)
	}
	return nil
}

// TodoDelete removes the item with the given id.
func (d *DB) TodoDelete(id int64) error {
	_, err := d.sql.Exec(`DELETE FROM session_todos WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("todo delete: %w", err)
	}
	return nil
}

// TodoDeleteSession removes all items for the given session.
func (d *DB) TodoDeleteSession(session string) error {
	_, err := d.sql.Exec(`DELETE FROM session_todos WHERE session = ?`, session)
	if err != nil {
		return fmt.Errorf("todo delete session: %w", err)
	}
	return nil
}

// TodoListOrphaned returns items keyed by session name for sessions not in activeSessions.
// Session names are collected first (closing the query cursor), then each session's items
// are fetched individually, to avoid holding the single SQLite connection open across nested queries.
func (d *DB) TodoListOrphaned(activeSessions []string) (map[string][]Todo, error) {
	active := make(map[string]struct{}, len(activeSessions))
	for _, s := range activeSessions {
		active[s] = struct{}{}
	}

	// Collect all distinct session names first, then close the cursor.
	rows, err := d.sql.Query(`SELECT DISTINCT session FROM session_todos`)
	if err != nil {
		return nil, fmt.Errorf("todo orphaned sessions: %w", err)
	}
	var allSessions []string
	for rows.Next() {
		var sess string
		if err := rows.Scan(&sess); err != nil {
			rows.Close()
			return nil, fmt.Errorf("todo orphaned scan: %w", err)
		}
		allSessions = append(allSessions, sess)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("todo orphaned rows close: %w", err)
	}

	// Now fetch items for each orphaned session (cursor is closed).
	result := map[string][]Todo{}
	for _, sess := range allSessions {
		if _, ok := active[sess]; ok {
			continue
		}
		items, err := d.TodoList(sess)
		if err != nil {
			return nil, err
		}
		result[sess] = items
	}
	return result, nil
}

// TodoSessionsWithOpen returns session names that have at least one unchecked item.
func (d *DB) TodoSessionsWithOpen() ([]string, error) {
	rows, err := d.sql.Query(
		`SELECT DISTINCT session FROM session_todos WHERE checked = 0 ORDER BY session ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("todo sessions with open: %w", err)
	}
	defer rows.Close()

	sessions := []string{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, fmt.Errorf("todo sessions with open scan: %w", err)
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}
