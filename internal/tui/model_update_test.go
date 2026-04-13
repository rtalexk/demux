package tui

import (
	"testing"

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
