package tui

import (
	"testing"

	"github.com/rtalexk/demux/internal/db"
	"github.com/rtalexk/demux/internal/tmux"
)

func TestResolveStateTargets_UpdatesMovedPane(t *testing.T) {
	states := []db.ToolState{
		{Target: "work:1.0", PaneID: "%3"},
		{Target: "other:0.0", PaneID: ""},
	}
	panes := []tmux.Pane{
		{Session: "work", WindowIndex: 2, PaneIndex: 0, PaneID: "%3"},
	}

	resolved := resolveStateTargets(states, panes)
	if resolved[0].Target != "work:2.0" {
		t.Errorf("want work:2.0, got %q", resolved[0].Target)
	}
	if resolved[1].Target != "other:0.0" {
		t.Errorf("legacy target should be unchanged, got %q", resolved[1].Target)
	}
}
