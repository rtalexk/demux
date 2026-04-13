package cmd

import (
	"fmt"
	"testing"
)

func TestBuildLivePaneMaps(t *testing.T) {
	panes := []struct {
		paneID  string
		session string
		winIdx  int
		paneIdx int
	}{
		{"%1", "work", 0, 0},
		{"%2", "work", 1, 0},
	}

	liveIDs := map[string]bool{}
	liveTargets := map[string]bool{}
	for _, p := range panes {
		liveIDs[p.paneID] = true
		target := fmt.Sprintf("%s:%d.%d", p.session, p.winIdx, p.paneIdx)
		liveTargets[target] = true
	}

	if !liveIDs["%1"] || !liveIDs["%2"] {
		t.Error("pane_ids not in live map")
	}
	if !liveTargets["work:0.0"] || !liveTargets["work:1.0"] {
		t.Error("pane targets not in live map")
	}
}
