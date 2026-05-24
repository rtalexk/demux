package cmd

import (
	"strings"
	"testing"

	"github.com/rtalexk/demux/internal/config"
	"github.com/rtalexk/demux/internal/sticky"
)

// Regression: ensureSlots (called by the after-new-window hook) must use the
// additive AddMissingSlots path, never ReconcileSlots. ReconcileSlots issues
// kill-pane for stale/duplicate slots; each kill fires after-kill-pane ->
// backgrounded `demux event pane_closed` -> SweepOrphanWindows, which may
// itself kill more panes. On a multi-window session launch the cascade
// saturates tmux's command queue and freezes input.
func TestEnsureSlots_NeverKillsPanes(t *testing.T) {
	tmux := &stubTmux{
		outputs: map[string]string{
			// AnySlotExists: 2 duplicate slots in @7, 1 stale slot in @99 (non-reserved).
			// Naive ReconcileSlots would kill all three extras.
			"list-panes -aF #{pane_id} #{@demux_slot}":      "%10 1\n%11 1\n%99 1\n",
			"display-message -p #{session_id}:#{window_id}": "$1:@7\n",
			// Reservation queries.
			"list-windows -aF #{window_id}":                          "@7\n",
			"list-windows -t $1 -F #{window_id} #{window_last_flag}": "@7 0\n",
			"display-message -p #{client_last_session}":              "\n",
			"show-environment -g " + sticky.MRUEnvKey:                "-" + sticky.MRUEnvKey + "\n",
			// AddMissingSlots -> EnsureSlotInWindow -> FindSlotInWindow per
			// reserved window. @7 already has a slot, so no split-window fires.
			"list-panes -t @7 -F #{pane_id} #{@demux_slot}": "%10 1\n",
		},
	}

	cfg := config.Default()
	cfg.Sidebar.Sticky.Slots = true
	cfg.Sidebar.Sticky.Width = 35
	oldCfg := loadConfig
	oldClient := stickyClient
	loadConfig = func() config.Config { return cfg }
	stickyClient = func() *sticky.Sticky {
		return &sticky.Sticky{T: tmux, Slots: true}
	}
	t.Cleanup(func() {
		loadConfig = oldCfg
		stickyClient = oldClient
	})

	if err := ensureSlots(); err != nil {
		t.Fatalf("ensureSlots: %v", err)
	}

	for _, r := range tmux.runs {
		j := strings.Join(r, " ")
		if strings.HasPrefix(j, "kill-pane") {
			t.Errorf("ensureSlots must not kill panes (cleanup belongs to `slots prune`), got: %v", r)
		}
	}
}
