package db

import (
	"database/sql"
	"errors"
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

// AlertLevel represents the severity of an alert. Higher values are more severe.
type AlertLevel = string

const (
	LevelInfo  AlertLevel = "info"
	LevelWarn  AlertLevel = "warn"
	LevelError AlertLevel = "error"
	LevelDefer AlertLevel = "defer"
)

type Alert struct {
	ID        int
	Target    string
	Reason    string
	Level     AlertLevel
	CreatedAt time.Time
}

func (d *DB) AlertSet(target, reason, level string) error {
	_, err := d.sql.Exec(`
        INSERT INTO alerts (target, reason, level, created_at)
        VALUES (?, ?, ?, CURRENT_TIMESTAMP)
        ON CONFLICT(target) DO UPDATE SET
            reason     = excluded.reason,
            level      = excluded.level,
            created_at = excluded.created_at
        WHERE (CASE excluded.level WHEN 'error' THEN 3 WHEN 'warn' THEN 2 WHEN 'info' THEN 1 ELSE 0 END)
            >= (CASE alerts.level WHEN 'error' THEN 3 WHEN 'warn' THEN 2 WHEN 'info' THEN 1 ELSE 0 END)
    `, target, reason, level)
	return err
}

func (d *DB) AlertRemove(target string) error {
	_, err := d.sql.Exec(`DELETE FROM alerts WHERE target = ?`, target)
	return err
}

func (d *DB) AlertList() ([]Alert, error) {
	rows, err := d.sql.Query(`
        SELECT id, target, reason, level, created_at
        FROM alerts ORDER BY created_at ASC
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []Alert
	for rows.Next() {
		var a Alert
		var createdAt string
		if err := rows.Scan(&a.ID, &a.Target, &a.Reason, &a.Level, &createdAt); err != nil {
			return nil, err
		}
		a.CreatedAt = parseTS(createdAt)
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

// AlertByTarget returns the alert for a target, or nil if not found.
func (d *DB) AlertByTarget(target string) (*Alert, error) {
	row := d.sql.QueryRow(`
        SELECT id, target, reason, level, created_at
        FROM alerts WHERE target = ?
    `, target)
	var a Alert
	var createdAt string
	err := row.Scan(&a.ID, &a.Target, &a.Reason, &a.Level, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	a.CreatedAt = parseTS(createdAt)
	return &a, nil
}
