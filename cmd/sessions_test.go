package cmd

import (
	"os"
	"testing"

	"github.com/rtalexk/demux/internal/db"
	"github.com/rtalexk/demux/internal/proc"
	"github.com/rtalexk/demux/internal/tmux"
)

func TestResolveSessionStatus(t *testing.T) {
	tests := []struct {
		name   string
		states map[string]db.ToolState
		want   string
	}{
		{"no states", nil, "ok"},
		{"working", map[string]db.ToolState{"s": {Value: db.StateWorking}}, "ok"},
		{"waiting", map[string]db.ToolState{"s": {Value: db.StateWaiting}}, "waiting"},
		{"error", map[string]db.ToolState{"s": {Value: db.StateError}}, "error"},
		{"flagged", map[string]db.ToolState{"s": {Value: db.StateFlagged}}, "flagged"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveSessionStatus(tt.states); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildSessionProcCounts(t *testing.T) {
	windows := map[int][]tmux.Pane{0: {{Session: "work", CWD: "/home/user/work"}}}
	grouped := map[string]map[int][]tmux.Pane{"work": windows}
	procs := []proc.Process{{PID: 1}, {PID: 2}}
	cwdByPID := map[int32]string{1: "/home/user/work", 2: "/other"}
	got := buildSessionProcCounts(grouped, procs, cwdByPID)
	if got["work"] != 1 {
		t.Errorf("got %d, want 1", got["work"])
	}
}

func TestClearSessionStatesWithID(t *testing.T) {
	// Use a temp file so we can open a second handle after clearSessionStatesWithID
	// closes the first one.
	f, err := os.CreateTemp("", "demux-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	dbPath := f.Name()
	t.Cleanup(func() { os.Remove(dbPath) })

	// Seed via the first handle.
	d, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	myapp1 := db.Target{Type: db.TargetTypePane, ID: "%1", SessionID: "$1", WindowID: "@1", PaneID: "%1"}
	myapp2 := db.Target{Type: db.TargetTypePane, ID: "%2", SessionID: "$1", WindowID: "@1", PaneID: "%2"}
	other := db.Target{Type: db.TargetTypePane, ID: "%3", SessionID: "$2", WindowID: "@2", PaneID: "%3"}
	d.StateSet(myapp1, "claude", db.StateWorking, "", db.SourceTool, false, nil)
	d.StateSet(myapp2, "make", db.StateError, "fail", db.SourceTool, false, nil)
	d.StateSet(other, "claude", db.StateDone, "ok", db.SourceTool, false, nil)
	d.Close()

	// Each clearSessionStatesWithID call opens and closes its own handle.
	openHandles := func() (*db.DB, error) { return db.Open(dbPath) }
	orig := openDB
	openDB = openHandles
	defer func() { openDB = orig }()

	got := clearSessionStatesWithID("myapp", "$1")
	if got != "2 state(s) cleared" {
		t.Errorf("got %q, want %q", got, "2 state(s) cleared")
	}

	// Verify other session is untouched by opening a fresh handle.
	d2, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer d2.Close()
	st, _ := d2.StateByID(other)
	if st == nil {
		t.Error("other session state should not be deleted")
	}
}

func TestClearSessionStatesWithID_NoneExist(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}

	orig := openDB
	openDB = func() (*db.DB, error) { return d, nil }
	defer func() { openDB = orig }()

	got := clearSessionStatesWithID("ghost", "$99")
	if got != "0 state(s) cleared" {
		t.Errorf("got %q, want %q", got, "0 state(s) cleared")
	}
}

func TestClearSessionStatesWithID_EmptySessionIDSkips(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}

	orig := openDB
	openDB = func() (*db.DB, error) { return d, nil }
	defer func() { openDB = orig }()

	got := clearSessionStatesWithID("ghost", "")
	if got != "session not found in tmux (skipped)" {
		t.Errorf("got %q, want \"session not found in tmux (skipped)\"", got)
	}
}
