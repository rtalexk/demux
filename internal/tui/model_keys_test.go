package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rtalexk/demux/internal/db"
)

func TestHighestPriorityPaneTarget_ReturnsHighestPriorityPane(t *testing.T) {
	states := []db.ToolState{
		{Target: "sess:0.0", Value: db.StateWorking},
		{Target: "sess:1.0", Value: db.StateError},   // highest priority
		{Target: "sess:2.0", Value: db.StateFlagged},
	}
	got := highestPriorityPaneTarget("sess", states)
	if got != "sess:1.0" {
		t.Errorf("want sess:1.0 (error=highest), got %q", got)
	}
}

func TestHighestPriorityPaneTarget_SkipsDone(t *testing.T) {
	states := []db.ToolState{
		{Target: "sess:0.0", Value: db.StateDone},
		{Target: "sess:1.0", Value: db.StateWorking},
	}
	got := highestPriorityPaneTarget("sess", states)
	if got != "sess:1.0" {
		t.Errorf("want sess:1.0 (done skipped), got %q", got)
	}
}

func TestHighestPriorityPaneTarget_ReturnsEmptyWhenNoState(t *testing.T) {
	states := []db.ToolState{
		{Target: "sess:0.0", Value: db.StateDone},
		{Target: "sess:1.0", Value: db.StateIdle},
	}
	got := highestPriorityPaneTarget("sess", states)
	if got != "" {
		t.Errorf("want empty (no qualifying state), got %q", got)
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
