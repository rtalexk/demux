package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	demuxlog "github.com/rtalexk/demux/internal/log"
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
	StateIdle    StateValue = 6
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
	case StateIdle:
		return "idle"
	default:
		return "idle"
	}
}

// IsDisplayable reports whether the state should be shown in UI and considered
// for navigation. It returns true for all named states except the two "resting"
// representations: the zero value (no DB row) and StateIdle (explicit idle).
func (v StateValue) IsDisplayable() bool {
	return v != 0 && v != StateIdle
}

// Priority returns a numeric urgency score for sorting and filtering.
// Higher values are more urgent. Waiting is the most urgent because it
// blocks on human input; Working and Idle are least urgent.
func (v StateValue) Priority() int {
	switch v {
	case StateWaiting:
		return 4
	case StateError:
		return 3
	case StateDone:
		return 2
	case StateFlagged:
		return 1
	case StateIdle:
		return -1 // explicit idle loses to working when both present in a session
	default: // StateWorking, unknown
		return 0
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
	case "idle":
		return StateIdle, nil
	default:
		return 0, fmt.Errorf("invalid state %q: must be working|waiting|done|error|flagged|idle", s)
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
	PaneID    string
	Source    StateSource
	UpdatedAt time.Time
}

// StateSet writes a state record for target, applying write-lock rules.
// Returns ErrStateLocked (wrapped) if the write is rejected.
func (d *DB) StateSet(target, tool string, value StateValue, message string, source StateSource, force bool, ifState *StateValue, paneID string) error {
	tx, err := d.sql.Begin()
	if err != nil {
		return fmt.Errorf("begin StateSet: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	// Read current record within the transaction.
	var current *ToolState
	row := tx.QueryRow(`SELECT tool, value, source FROM tool_states WHERE target = ?`, target)
	var cur ToolState
	err = row.Scan(&cur.Tool, &cur.Value, &cur.Source)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read current state: %w", err)
	}
	if err == nil {
		current = &cur
	}

	// No-op if value and tool are unchanged (avoids redundant DB writes from
	// high-frequency hooks like PreToolUse firing working→working repeatedly).
	if ifState == nil && current != nil && current.Value == value && current.Tool == tool {
		demuxlog.Debug("state set skipped: no change", "target", target, "tool", tool, "state", value)
		return tx.Rollback()
	}

	// Apply --if-state condition: no-op if current state doesn't match.
	// A missing row is treated as StateValue(0), not StateIdle; callers
	// wanting "write if currently idle" must pass StateValue(0), not &StateIdle.
	if ifState != nil {
		currentValue := StateValue(0)
		if current != nil {
			currentValue = current.Value
		}
		if currentValue != *ifState {
			return tx.Rollback()
		}
	}

	// When a stable pane_id is provided, delete any stale record for this pane
	// at a different target (handles panes that moved since last write).
	if paneID != "" {
		if _, err := tx.Exec(
			`DELETE FROM tool_states WHERE pane_id = ? AND target != ?`,
			paneID, target,
		); err != nil {
			return fmt.Errorf("delete stale pane record: %w", err)
		}
	}

	// Apply lock rules for tool writes (skipped when force=true).
	if !force && source == SourceTool && current != nil {
		switch current.Value {
		case StateFlagged:
			return fmt.Errorf("%w (current: flagged, user-owned)", ErrStateLocked)
		case StateWorking, StateWaiting, StateError:
			if current.Tool != tool {
				return fmt.Errorf("%w (current: %s by %s)", ErrStateLocked, current.Value, current.Tool)
			}
		}
	}

	_, err = tx.Exec(`
		INSERT INTO tool_states (target, tool, value, message, source, pane_id, updated_at)
		VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), CURRENT_TIMESTAMP)
		ON CONFLICT(target) DO UPDATE SET
			tool       = excluded.tool,
			value      = excluded.value,
			message    = excluded.message,
			source     = excluded.source,
			pane_id    = COALESCE(excluded.pane_id, tool_states.pane_id),
			updated_at = excluded.updated_at
	`, target, tool, int(value), message, int(source), paneID)
	if err != nil {
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

// StateDeleteIfResting removes the state record when value is done or idle.
// Used by event pane_focus to clear resting states on navigation.
func (d *DB) StateDeleteIfResting(target string) error {
	_, err := d.sql.Exec(
		`DELETE FROM tool_states WHERE target = ? AND value IN (?, ?)`,
		target, int(StateDone), int(StateIdle),
	)
	return err
}

// StateClearByPaneID removes the state record whose pane_id matches.
// No-op if no record has that pane_id.
func (d *DB) StateClearByPaneID(paneID string) error {
	_, err := d.sql.Exec(`DELETE FROM tool_states WHERE pane_id = ?`, paneID)
	return err
}

// StateDeleteIfRestingByPaneID removes the state record when value is done or idle,
// looking up by pane_id. Used by pane_focus when pane_id is available — handles
// panes that have moved positions since the state was written.
func (d *DB) StateDeleteIfRestingByPaneID(paneID string) error {
	_, err := d.sql.Exec(
		`DELETE FROM tool_states WHERE pane_id = ? AND value IN (?, ?)`,
		paneID, int(StateDone), int(StateIdle),
	)
	return err
}

// looksLikePaneTarget reports whether target is in "session:window.pane" format.
// Used by StateGCOrphaned to skip session-level and window-level targets.
func looksLikePaneTarget(target string) bool {
	return strings.Contains(target, ":") && strings.Contains(target, ".")
}

// StateGCOrphaned deletes state records whose pane is no longer live.
// For records with a pane_id: orphaned when pane_id is not in livePaneIDs.
// For legacy records (pane_id empty) that look like "session:window.pane":
// orphaned when target is not in livePaneTargets.
// Session-level and window-level targets (no ".") are never deleted.
// Returns the number of records deleted.
func (d *DB) StateGCOrphaned(livePaneIDs map[string]bool, livePaneTargets map[string]bool) (int64, error) {
	states, err := d.StateList(0, "")
	if err != nil {
		return 0, err
	}
	var deleted int64
	for _, st := range states {
		orphaned := false
		if st.PaneID != "" {
			orphaned = !livePaneIDs[st.PaneID]
		} else if looksLikePaneTarget(st.Target) {
			orphaned = !livePaneTargets[st.Target]
		}
		if orphaned {
			if err := d.StateClear(st.Target); err != nil {
				return deleted, fmt.Errorf("gc: clear %q: %w", st.Target, err)
			}
			deleted++
		}
	}
	return deleted, nil
}

// StateDeleteBySession removes all state records for the given session.
// It matches targets equal to name (bare) or prefixed with "name:" (session:window).
// Returns the number of rows deleted.
func (d *DB) StateDeleteBySession(name string) (int64, error) {
	res, err := d.sql.Exec(
		`DELETE FROM tool_states WHERE target = ? OR target LIKE ?`,
		name, name+":%",
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// StateByTarget returns the ToolState for target, or nil if no record exists (idle).
func (d *DB) StateByTarget(target string) (*ToolState, error) {
	row := d.sql.QueryRow(`
		SELECT id, target, tool, value, message, pane_id, source, updated_at
		FROM tool_states WHERE target = ?
	`, target)
	var st ToolState
	var updatedAt string
	var paneID sql.NullString
	err := row.Scan(&st.ID, &st.Target, &st.Tool, &st.Value, &st.Message, &paneID, &st.Source, &updatedAt)
	st.PaneID = paneID.String
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
	q := `SELECT id, target, tool, value, message, pane_id, source, updated_at FROM tool_states WHERE 1=1`
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
		var paneID sql.NullString
		if err := rows.Scan(&st.ID, &st.Target, &st.Tool, &st.Value, &st.Message, &paneID, &st.Source, &updatedAt); err != nil {
			return nil, err
		}
		st.PaneID = paneID.String
		st.UpdatedAt = parseTS(updatedAt)
		states = append(states, st)
	}
	return states, rows.Err()
}
