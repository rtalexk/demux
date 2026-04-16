package tui

import "time"

const (
	// minDetailH is the minimum height in rows for the detail panel.
	minDetailH = 4

	// Status bar display durations.
	statusExpShort  = 2 * time.Second // transient confirmations (yank)
	statusExpMedium = 3 * time.Second // async action results (procActionMsg)
	statusExpLong   = 4 * time.Second // item errors (delete, editor)
	statusExpError  = 5 * time.Second // launch/config errors

	// Background timing.
	marqueeTick            = 200 * time.Millisecond
	searchDebounceInterval = 150 * time.Millisecond
	procPollInterval       = 2 * time.Second
)
