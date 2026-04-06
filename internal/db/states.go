package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// parseTS handles the varying timestamp formats returned by modernc.org/sqlite.
func parseTS(s string) time.Time {
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

// StateValue is the integer enum stored in tool_states.value.
type StateValue int

const (
	StateWorking StateValue = 1
	StateWaiting StateValue = 2
	StateDone    StateValue = 3
	StateError   StateValue = 4
	StateFlagged StateValue = 5
)

func (v StateValue) String() string {
	switch v {
	case StateWorking:
		return "working"
	case StateWaiting:
		return "waiting"
	case StateDone:
		return "done"
	case StateError:
		return "error"
	case StateFlagged:
		return "flagged"
	default:
		return "idle"
	}
}

// ParseStateValue converts a user-supplied string to a StateValue.
func ParseStateValue(s string) (StateValue, error) {
	switch s {
	case "working":
		return StateWorking, nil
	case "waiting":
		return StateWaiting, nil
	case "done":
		return StateDone, nil
	case "error":
		return StateError, nil
	case "flagged":
		return StateFlagged, nil
	default:
		return 0, fmt.Errorf("invalid state %q: must be working|waiting|done|error|flagged", s)
	}
}

// StateSource is the integer enum stored in tool_states.source.
type StateSource int

const (
	SourceTool StateSource = 1
	SourceUser StateSource = 2
)

func (s StateSource) String() string {
	if s == SourceUser {
		return "user"
	}
	return "tool"
}

// ErrStateLocked is returned by StateSet when a write is rejected due to lock rules.
var ErrStateLocked = errors.New("target locked")

// ToolState represents a row in tool_states.
type ToolState struct {
	ID        int
	Target    string
	Tool      string
	Value     StateValue
	Message   string
	Source    StateSource
	UpdatedAt time.Time
}

// StateSet writes a state record for target, applying write-lock rules.
// Returns ErrStateLocked (wrapped) if the write is rejected.
func (d *DB) StateSet(target, tool string, value StateValue, message string, source StateSource) error {
	tx, err := d.sql.Begin()
	if err != nil {
		return fmt.Errorf("begin StateSet: %w", err)
	}

	// Read current record within the transaction.
	var current *ToolState
	row := tx.QueryRow(`SELECT tool, value, source FROM tool_states WHERE target = ?`, target)
	var cur ToolState
	err = row.Scan(&cur.Tool, &cur.Value, &cur.Source)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		tx.Rollback()
		return fmt.Errorf("read current state: %w", err)
	}
	if err == nil {
		current = &cur
	}

	// Apply lock rules for tool writes.
	if source == SourceTool && current != nil {
		switch current.Value {
		case StateFlagged:
			tx.Rollback()
			return fmt.Errorf("%w (current: flagged, user-owned)", ErrStateLocked)
		case StateWorking, StateWaiting, StateError:
			if current.Tool != tool {
				tx.Rollback()
				return fmt.Errorf("%w (current: %s by %s)", ErrStateLocked, current.Value, current.Tool)
			}
		}
	}

	_, err = tx.Exec(`
		INSERT INTO tool_states (target, tool, value, message, source, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(target) DO UPDATE SET
			tool       = excluded.tool,
			value      = excluded.value,
			message    = excluded.message,
			source     = excluded.source,
			updated_at = excluded.updated_at
	`, target, tool, int(value), message, int(source))
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("upsert state: %w", err)
	}

	return tx.Commit()
}

// StateClear removes the state record for target, returning it to idle.
// Always succeeds regardless of current state.
func (d *DB) StateClear(target string) error {
	_, err := d.sql.Exec(`DELETE FROM tool_states WHERE target = ?`, target)
	return err
}

// StateDeleteIfDone removes the state record only if the current value is done.
// Used by event pane_focus to silently clear done states on navigation.
func (d *DB) StateDeleteIfDone(target string) error {
	_, err := d.sql.Exec(`DELETE FROM tool_states WHERE target = ? AND value = ?`, target, int(StateDone))
	return err
}

// StateByTarget returns the ToolState for target, or nil if no record exists (idle).
func (d *DB) StateByTarget(target string) (*ToolState, error) {
	row := d.sql.QueryRow(`
		SELECT id, target, tool, value, message, source, updated_at
		FROM tool_states WHERE target = ?
	`, target)
	var st ToolState
	var updatedAt string
	err := row.Scan(&st.ID, &st.Target, &st.Tool, &st.Value, &st.Message, &st.Source, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	st.UpdatedAt = parseTS(updatedAt)
	return &st, nil
}

// StateList returns all non-idle state records. Pass value=0 or tool="" to skip that filter.
func (d *DB) StateList(value StateValue, tool string) ([]ToolState, error) {
	q := `SELECT id, target, tool, value, message, source, updated_at FROM tool_states WHERE 1=1`
	var args []any
	if value != 0 {
		q += ` AND value = ?`
		args = append(args, int(value))
	}
	if tool != "" {
		q += ` AND tool = ?`
		args = append(args, tool)
	}
	q += ` ORDER BY updated_at ASC`

	rows, err := d.sql.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var states []ToolState
	for rows.Next() {
		var st ToolState
		var updatedAt string
		if err := rows.Scan(&st.ID, &st.Target, &st.Tool, &st.Value, &st.Message, &st.Source, &updatedAt); err != nil {
			return nil, err
		}
		st.UpdatedAt = parseTS(updatedAt)
		states = append(states, st)
	}
	return states, rows.Err()
}
