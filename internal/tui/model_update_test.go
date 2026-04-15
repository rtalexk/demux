package tui

import (
	"testing"

	"github.com/rtalexk/demux/internal/db"
	"github.com/rtalexk/demux/internal/tmux"
)

func TestSessionNameToIDMap(t *testing.T) {
	panes := []tmux.Pane{
		{Session: "work", SessionID: "$1", WindowIndex: 0, PaneIndex: 0, PaneID: "%1"},
		{Session: "work", SessionID: "$1", WindowIndex: 0, PaneIndex: 1, PaneID: "%2"},
		{Session: "other", SessionID: "$2", WindowIndex: 0, PaneIndex: 0, PaneID: "%3"},
	}
	m := tmux.SessionNameToIDMap(panes)
	if got := m["work"]; got != "$1" {
		t.Errorf("work: want $1, got %q", got)
	}
	if got := m["other"]; got != "$2" {
		t.Errorf("other: want $2, got %q", got)
	}
}

func TestSessionNameToIDMap_Empty(t *testing.T) {
	m := tmux.SessionNameToIDMap(nil)
	if len(m) != 0 {
		t.Errorf("want empty map, got %v", m)
	}
}

func TestHealStateSessionIDs_UpdatesSessionID(t *testing.T) {
	states := []db.ToolState{
		{Target: db.Target{Type: db.TargetTypePane, ID: "%1", SessionID: "$0"}},
	}
	paneIDMap := map[string]string{"%1": "work:%1"}
	nameToIDMap := map[string]string{"work": "$1"}

	healStateSessionIDs(states, paneIDMap, nameToIDMap)

	if states[0].Target.SessionID != "$1" {
		t.Errorf("expected SessionID $1, got %q", states[0].Target.SessionID)
	}
}

func TestHealStateSessionIDs_NoopWhenAlreadyCorrect(t *testing.T) {
	states := []db.ToolState{
		{Target: db.Target{Type: db.TargetTypePane, ID: "%1", SessionID: "$1"}},
	}
	paneIDMap := map[string]string{"%1": "work:%1"}
	nameToIDMap := map[string]string{"work": "$1"}

	healStateSessionIDs(states, paneIDMap, nameToIDMap)

	if states[0].Target.SessionID != "$1" {
		t.Errorf("session ID should be unchanged, got %q", states[0].Target.SessionID)
	}
}

func TestHealStateSessionIDs_IgnoresNonPane(t *testing.T) {
	states := []db.ToolState{
		{Target: db.Target{Type: db.TargetTypeWindow, ID: "@1", SessionID: "$0"}},
	}
	paneIDMap := map[string]string{"@1": "work:@1"}
	nameToIDMap := map[string]string{"work": "$1"}

	healStateSessionIDs(states, paneIDMap, nameToIDMap)

	if states[0].Target.SessionID != "$0" {
		t.Errorf("non-pane entry should be unchanged, got %q", states[0].Target.SessionID)
	}
}

func TestHealStateSessionIDs_IgnoresUnknownPane(t *testing.T) {
	states := []db.ToolState{
		{Target: db.Target{Type: db.TargetTypePane, ID: "%99", SessionID: "$0"}},
	}
	paneIDMap := map[string]string{} // %99 not in live map
	nameToIDMap := map[string]string{"work": "$1"}

	healStateSessionIDs(states, paneIDMap, nameToIDMap)

	if states[0].Target.SessionID != "$0" {
		t.Errorf("unknown pane should be unchanged, got %q", states[0].Target.SessionID)
	}
}
