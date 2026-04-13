package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rtalexk/demux/internal/db"
)

func TestHighestPriorityPaneTarget_ReturnsHighestPriorityPane(t *testing.T) {
	states := []db.ToolState{
		{Target: "sess:0.0", Value: db.StateWorking},
		{Target: "sess:1.0", Value: db.StateError}, // highest priority
		{Target: "sess:2.0", Value: db.StateFlagged},
	}
	got := highestPriorityPaneTarget("sess", states)
	if got != "sess:1.0" {
		t.Errorf("want sess:1.0 (error=highest), got %q", got)
	}
}

func TestHighestPriorityPaneTarget_IncludesDone(t *testing.T) {
	// Done has priority=2; Working has priority=0; Done wins.
	states := []db.ToolState{
		{Target: "sess:0.0", Value: db.StateDone},
		{Target: "sess:1.0", Value: db.StateWorking},
	}
	got := highestPriorityPaneTarget("sess", states)
	if got != "sess:0.0" {
		t.Errorf("want sess:0.0 (done beats working), got %q", got)
	}
}

func TestHighestPriorityPaneTarget_ReturnsEmptyWhenOnlyIdle(t *testing.T) {
	states := []db.ToolState{
		{Target: "sess:0.0", Value: db.StateIdle},
		{Target: "sess:1.0", Value: db.StateIdle},
	}
	got := highestPriorityPaneTarget("sess", states)
	if got != "" {
		t.Errorf("want empty (only idle states), got %q", got)
	}
}

func TestHighestPriorityPaneTarget_IgnoresOtherSessions(t *testing.T) {
	states := []db.ToolState{
		{Target: "other:0.0", Value: db.StateWaiting},
		{Target: "sess:0.0", Value: db.StateWorking},
	}
	got := highestPriorityPaneTarget("sess", states)
	if got != "sess:0.0" {
		t.Errorf("want sess:0.0 (other session ignored), got %q", got)
	}
}

func TestHighestPriorityPaneTarget_ReturnsEmptyForEmptyStates(t *testing.T) {
	got := highestPriorityPaneTarget("sess", nil)
	if got != "" {
		t.Errorf("want empty, got %q", got)
	}
}

func TestClearCurrentState_ClearsByPaneID(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	// State was written when pane was at old position.
	_ = d.StateSet("work:1.0", "claude", db.StateWorking, "", db.SourceTool, false, nil, "%5")

	m := Model{db: d}
	cmd := m.clearCurrentState("work:2.0", "%5") // resolved target + pane_id
	cmd()

	st, _ := d.StateByTarget("work:1.0")
	if st != nil {
		t.Errorf("state should be cleared by pane_id, got: %+v", st)
	}
}

func TestClearCurrentState_FallsBackToTarget(t *testing.T) {
	d, _ := db.Open(":memory:")
	defer d.Close()

	_ = d.StateSet("work:1.0", "claude", db.StateWorking, "", db.SourceTool, false, nil, "")

	m := Model{db: d}
	cmd := m.clearCurrentState("work:1.0", "") // no pane_id, falls back to target
	cmd()

	st, _ := d.StateByTarget("work:1.0")
	if st != nil {
		t.Errorf("state should be cleared by target, got: %+v", st)
	}
}

func TestResolveFilterKey(t *testing.T) {
	tests := []struct {
		keyStr string
		want   SidebarFilter
		wantOk bool
	}{
		{"t", FilterTmux, true},
		{"a", FilterAll, true},
		{"c", FilterConfig, true},
		{"w", FilterWorktree, true},
		{"!", FilterPriority, true},
		{"x", "", false},
		{"q", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.keyStr, func(t *testing.T) {
			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.keyStr)}
			got, ok := resolveFilterKey(msg)
			if ok != tt.wantOk {
				t.Errorf("ok: got %v, want %v", ok, tt.wantOk)
			}
			if ok && got != tt.want {
				t.Errorf("filter: got %q, want %q", got, tt.want)
			}
		})
	}
}
