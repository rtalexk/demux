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
		alerts []db.Alert
		want   string
	}{
		{"no alerts", nil, "ok"},
		{"info only", []db.Alert{{Level: "info"}}, "ok"},
		{"warn", []db.Alert{{Level: "warn"}}, "warn"},
		{"error beats warn", []db.Alert{{Level: "warn"}, {Level: "error"}}, "error"},
		{"error alone", []db.Alert{{Level: "error"}}, "error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveSessionStatus(tt.alerts); got != tt.want {
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
