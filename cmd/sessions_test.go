package cmd

import (
	"testing"

	"github.com/rtalexk/demux/internal/db"
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
