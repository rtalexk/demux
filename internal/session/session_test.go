package session

import (
	"testing"

	"github.com/rtalexk/demux/internal/tmux"
)

func TestConfigEntry_DisplayName(t *testing.T) {
	e := ConfigEntry{Group: "dotf", Name: "dotf-main"}
	if got := e.DisplayName(); got != "dotf-main" {
		t.Errorf("expected dotf-main, got %s", got)
	}
}

func TestMerge_LiveOnly(t *testing.T) {
	panes := []tmux.Pane{{Session: "myapp"}}
	sessions := Merge(panes, nil)
	if len(sessions) != 1 {
		t.Fatalf("expected 1, got %d", len(sessions))
	}
	if !sessions[0].IsLive || sessions[0].IsConfig {
		t.Error("expected IsLive=true, IsConfig=false")
	}
	if sessions[0].DisplayName != "myapp" {
		t.Errorf("expected myapp, got %s", sessions[0].DisplayName)
	}
}

func TestMerge_ConfigOnly(t *testing.T) {
	entries := []ConfigEntry{{Name: "dotf-main", Group: "dotf", Path: "/foo"}}
	sessions := Merge(nil, entries)
	if len(sessions) != 1 {
		t.Fatalf("expected 1, got %d", len(sessions))
	}
	s := sessions[0]
	if s.IsLive || !s.IsConfig {
		t.Error("expected IsLive=false, IsConfig=true")
	}
	if s.DisplayName != "dotf-main" {
		t.Errorf("expected dotf-main, got %s", s.DisplayName)
	}
}

func TestMerge_ExactMatch_MergesIntoOne(t *testing.T) {
	panes := []tmux.Pane{{Session: "dotf-main"}}
	entries := []ConfigEntry{{Name: "dotf-main", Group: "dotf", Path: "/foo"}}
	sessions := Merge(panes, entries)
	if len(sessions) != 1 {
		t.Fatalf("expected 1 merged session, got %d", len(sessions))
	}
	s := sessions[0]
	if !s.IsLive || !s.IsConfig {
		t.Error("expected IsLive=true, IsConfig=true")
	}
}

func TestMerge_NoPartialMatch(t *testing.T) {
	// Tmux "dotfiles" does NOT match config "dotf-main"
	panes := []tmux.Pane{{Session: "dotfiles"}}
	entries := []ConfigEntry{{Name: "dotf-main", Group: "dotf", Path: "/foo"}}
	sessions := Merge(panes, entries)
	if len(sessions) != 2 {
		t.Fatalf("expected 2 separate sessions, got %d", len(sessions))
	}
}
