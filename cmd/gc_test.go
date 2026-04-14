package cmd

import (
	"testing"

	"github.com/rtalexk/demux/internal/tmux"
)

func TestBuildLivePaneMaps(t *testing.T) {
	panes := []tmux.Pane{
		{PaneID: "%1", Session: "work", WindowIndex: 0, PaneIndex: 0},
		{PaneID: "%2", Session: "work", WindowIndex: 1, PaneIndex: 0},
	}

	liveIDs := map[string]bool{}
	liveTargets := map[string]bool{}
	for _, p := range panes {
		if p.PaneID != "" {
			liveIDs[p.PaneID] = true
		}
		liveTargets[p.DisplayLabel()] = true
	}

	if !liveIDs["%1"] || !liveIDs["%2"] {
		t.Error("pane_ids not in live map")
	}
	if !liveTargets["work:0.0"] || !liveTargets["work:1.0"] {
		t.Error("pane targets not in live map")
	}
}
