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

// TargetType identifies the kind of tmux entity a state record is attached to.
type TargetType string

const (
	TargetTypePane    TargetType = "pane"
	TargetTypeWindow  TargetType = "window"
	TargetTypeSession TargetType = "session"
)

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

// Target represents the identity of a state record.
type Target struct {
	Type      TargetType // TargetTypePane, TargetTypeWindow, or TargetTypeSession
	ID        string     // $N, @N, %N
	SessionID string     // $N — all record types
	WindowID  string     // @N — window and pane records
	PaneID    string     // %N — pane records only
}

// Format resolves the target to a compact human-readable display string using
// live tmux ID→label maps. Falls back to the raw ID when no map entry exists.
//
// paneMap   maps %N → "session:windowIndex.paneIndex"
// windowMap maps @N → "session:windowIndex"
// sessionMap maps $N → session_name
func (t Target) Format(paneMap, windowMap, sessionMap map[string]string) string {
	// Note: pane rendering strips the session prefix and produces "winN·pM"
	// (compact form). This intentionally differs from formatTarget in cmd/state.go,
	// which uses the full resolved path for tabular CLI output.
	switch t.Type {
	case TargetTypePane:
		if full := paneMap[t.ID]; full != "" {
			if idx := strings.Index(full, ":"); idx >= 0 {
				rest := full[idx+1:]
				parts := strings.SplitN(rest, ".", 2)
				if len(parts) == 2 {
					return "win" + parts[0] + "·p" + parts[1]
				}
				return rest
			}
			return full
		}
		// Pane not in live map (orphaned or moved); use stored parent IDs as fallback.
		if t.WindowID != "" {
			if full := windowMap[t.WindowID]; full != "" {
				if idx := strings.Index(full, ":"); idx >= 0 {
					return "win" + full[idx+1:] + "·?"
				}
			}
		}
		if t.SessionID != "" {
			if name := sessionMap[t.SessionID]; name != "" {
				return name + ":?"
			}
		}
		return t.ID
	case TargetTypeWindow:
		if full := windowMap[t.ID]; full != "" {
			if idx := strings.Index(full, ":"); idx >= 0 {
				return "win" + full[idx+1:]
			}
			return full
		}
		// Window not in live map; fall back to session name.
		if t.SessionID != "" {
			if name := sessionMap[t.SessionID]; name != "" {
				return name + ":?"
			}
		}
		return t.ID
	case TargetTypeSession:
		if name := sessionMap[t.ID]; name != "" {
			return name
		}
		return t.ID
	default:
		return t.ID
	}
}

// ParseTargetID infers the target type from the tmux ID prefix.
// % → pane, @ → window, $ → session
func ParseTargetID(id string) (Target, error) {
	if len(id) == 0 {
		return Target{}, fmt.Errorf("target ID cannot be empty")
	}
	switch id[0] {
	case '%':
		return Target{Type: TargetTypePane, ID: id, PaneID: id}, nil
	case '@':
		return Target{Type: TargetTypeWindow, ID: id, WindowID: id}, nil
	case '$':
		return Target{Type: TargetTypeSession, ID: id, SessionID: id}, nil
	default:
		return Target{}, fmt.Errorf("invalid target ID prefix %q: must start with %%, @, or $", id[:1])
	}
}

// ToolState represents a row in tool_states.
type ToolState struct {
	ID        int
	Target    Target // embedded struct with Type, ID, SessionID, WindowID, PaneID
	Tool      string
	Value     StateValue
	Message   string
	Source    StateSource
	UpdatedAt time.Time
}

// StateSet writes a state record for target, applying write-lock rules.
// Returns ErrStateLocked (wrapped) if the write is rejected.
func (d *DB) StateSet(t Target, tool string, value StateValue, message string, source StateSource, force bool, ifState *StateValue) error {
	tx, err := d.sql.Begin()
	if err != nil {
		return fmt.Errorf("begin StateSet: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	// Read current record within the transaction.
	var current *ToolState
	row := tx.QueryRow(`SELECT tool, value, source, session_id FROM tool_states WHERE target_type = ? AND target_id = ?`, t.Type, t.ID)
	var cur ToolState
	err = row.Scan(&cur.Tool, &cur.Value, &cur.Source, &cur.Target.SessionID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read current state: %w", err)
	}
	if err == nil {
		current = &cur
	}

	// No-op if value and tool are unchanged (avoids redundant DB writes from
	// high-frequency hooks like PreToolUse firing working→working repeatedly).
	// Exception: allow the write through when session_id is unset in the DB but
	// the caller provides one — the upsert's COALESCE logic will populate it.
	parentIDMissing := current != nil && current.Target.SessionID == "" && t.SessionID != ""
	if ifState == nil && current != nil && current.Value == value && current.Tool == tool && !parentIDMissing {
		demuxlog.Debug("state set skipped: no change", "target_type", t.Type, "target_id", t.ID, "tool", tool, "state", value, "message", message)
		return tx.Rollback()
	}

	// Apply --if-state condition: no-op if current state doesn't match.
	if ifState != nil {
		currentValue := StateValue(0)
		if current != nil {
			currentValue = current.Value
		}
		if currentValue != *ifState {
			return tx.Rollback()
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
		INSERT INTO tool_states (target_type, target_id, session_id, window_id, pane_id, tool, value, message, source, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(target_type, target_id) DO UPDATE SET
			tool       = excluded.tool,
			value      = excluded.value,
			message    = excluded.message,
			source     = excluded.source,
			session_id = COALESCE(NULLIF(excluded.session_id, ''), tool_states.session_id),
			window_id  = COALESCE(NULLIF(excluded.window_id,  ''), tool_states.window_id),
			pane_id    = COALESCE(NULLIF(excluded.pane_id,    ''), tool_states.pane_id),
			updated_at = excluded.updated_at
	`, t.Type, t.ID, t.SessionID, t.WindowID, t.PaneID, tool, int(value), message, int(source))
	if err != nil {
		return fmt.Errorf("upsert state: %w", err)
	}

	return tx.Commit()
}

// StateClear removes the state record for target, returning it to idle.
// Always succeeds regardless of current state.
func (d *DB) StateClear(t Target) error {
	_, err := d.sql.Exec(`DELETE FROM tool_states WHERE target_type = ? AND target_id = ?`, t.Type, t.ID)
	return err
}

// StateDeleteIfResting removes the state record when value is done or idle.
// Used by event pane_focus to clear resting states on navigation.
func (d *DB) StateDeleteIfResting(t Target) error {
	_, err := d.sql.Exec(
		`DELETE FROM tool_states WHERE target_type = ? AND target_id = ? AND value IN (?, ?)`,
		t.Type, t.ID, int(StateDone), int(StateIdle),
	)
	return err
}

// StateGCOrphaned deletes state records whose target is no longer live.
// Takes maps of live pane IDs, window IDs, and session IDs.
// Returns the number of records deleted.
// Uses one DELETE per target type rather than one per orphaned row.
func (d *DB) StateGCOrphaned(livePaneIDs, liveWindowIDs, liveSessionIDs map[string]bool) (int64, error) {
	tx, err := d.sql.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin StateGCOrphaned: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	var total int64
	for _, item := range []struct {
		targetType TargetType
		live       map[string]bool
	}{
		{TargetTypePane, livePaneIDs},
		{TargetTypeWindow, liveWindowIDs},
		{TargetTypeSession, liveSessionIDs},
	} {
		n, err := gcDeleteOrphaned(tx, item.targetType, item.live)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, tx.Commit()
}

// gcDeleteOrphaned removes records of the given target type whose ID is not in liveIDs.
func gcDeleteOrphaned(tx *sql.Tx, targetType TargetType, liveIDs map[string]bool) (int64, error) {
	var res sql.Result
	var err error
	if len(liveIDs) == 0 {
		res, err = tx.Exec(`DELETE FROM tool_states WHERE target_type = ?`, string(targetType))
	} else {
		args := make([]any, 0, len(liveIDs)+1)
		args = append(args, string(targetType))
		placeholders := make([]string, 0, len(liveIDs))
		for id := range liveIDs {
			placeholders = append(placeholders, "?")
			args = append(args, id)
		}
		query := fmt.Sprintf(
			`DELETE FROM tool_states WHERE target_type = ? AND target_id NOT IN (%s)`,
			strings.Join(placeholders, ", "),
		)
		res, err = tx.Exec(query, args...)
	}
	if err != nil {
		return 0, fmt.Errorf("gc orphaned %s: %w", targetType, err)
	}
	return res.RowsAffected()
}

// StateRefreshParentIDs updates session_id and window_id for a pane record when
// the pane has moved (e.g., break-pane, move-pane to a different session or
// window). Called on every pane_focus event so the DB stays in sync with the
// live tmux layout without requiring an explicit state set.
//
// updated_at is intentionally left unchanged so state age display is unaffected.
// No-op when no record exists for paneID or when the stored IDs already match.
func (d *DB) StateRefreshParentIDs(paneID, windowID, sessionID string) error {
	_, err := d.sql.Exec(`
		UPDATE tool_states
		SET session_id = ?, window_id = ?
		WHERE target_type = 'pane' AND target_id = ?
		  AND (session_id IS NOT ? OR window_id IS NOT ?)
	`, sessionID, windowID, paneID, sessionID, windowID)
	return err
}

// StateDeleteBySession removes all state records for the given session.
// Returns the total number of rows deleted.
func (d *DB) StateDeleteBySession(sessionID string) (int64, error) {
	res, err := d.sql.Exec(`DELETE FROM tool_states WHERE session_id = ?`, sessionID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// StateDeleteByWindow removes the window's own state row and all pane rows
// belonging to it. Window-target rows and pane rows both carry window_id, so a
// single delete cascades window + its panes. Returns the rows deleted.
func (d *DB) StateDeleteByWindow(windowID string) (int64, error) {
	res, err := d.sql.Exec(`DELETE FROM tool_states WHERE window_id = ?`, windowID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// StateByID returns the ToolState for the given target identity, or nil if no record exists (idle).
func (d *DB) StateByID(t Target) (*ToolState, error) {
	row := d.sql.QueryRow(`
		SELECT id, target_type, target_id, session_id, window_id, pane_id, tool, value, message, source, updated_at
		FROM tool_states WHERE target_type = ? AND target_id = ?
	`, t.Type, t.ID)
	var st ToolState
	var updatedAt string
	err := row.Scan(&st.ID, &st.Target.Type, &st.Target.ID, &st.Target.SessionID, &st.Target.WindowID, &st.Target.PaneID, &st.Tool, &st.Value, &st.Message, &st.Source, &updatedAt)
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
	q := `SELECT id, target_type, target_id, session_id, window_id, pane_id, tool, value, message, source, updated_at FROM tool_states WHERE 1=1`
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
		if err := rows.Scan(&st.ID, &st.Target.Type, &st.Target.ID, &st.Target.SessionID, &st.Target.WindowID, &st.Target.PaneID, &st.Tool, &st.Value, &st.Message, &st.Source, &updatedAt); err != nil {
			return nil, err
		}
		st.UpdatedAt = parseTS(updatedAt)
		states = append(states, st)
	}
	return states, rows.Err()
}
