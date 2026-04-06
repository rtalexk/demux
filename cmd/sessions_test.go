package cmd

import (
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
