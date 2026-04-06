package cmd

import (
	"strings"
	"testing"

	"github.com/rtalexk/demux/internal/config"
	"github.com/rtalexk/demux/internal/db"
)

func TestCountStatesByValue(t *testing.T) {
	states := []db.ToolState{
		{Value: db.StateWorking},
		{Value: db.StateWaiting},
		{Value: db.StateWaiting},
		{Value: db.StateError},
	}
	waiting, errs := countStatesByValue(states)
	if waiting != 2 {
		t.Errorf("waiting: want 2, got %d", waiting)
	}
	if errs != 1 {
		t.Errorf("errors: want 1, got %d", errs)
	}
}

func TestTmuxStatusParts_NoStates(t *testing.T) {
	out := tmuxStatusParts(0, 0, config.Default())
	if !strings.Contains(out, "✔") {
		t.Errorf("expected clean check, got: %q", out)
	}
}

func TestTmuxStatusParts_ErrorFirst(t *testing.T) {
	cfg := config.Default()
	out := tmuxStatusParts(1, 1, cfg)
	errIdx := strings.Index(out, "✗")
	waitIdx := strings.Index(out, "●")
	if errIdx == -1 || waitIdx == -1 {
		t.Fatalf("missing icon: %q", out)
	}
	if errIdx > waitIdx {
		t.Errorf("error icon should appear before waiting icon")
	}
}

func TestTmuxStatusParts_WaitingOnly(t *testing.T) {
	out := tmuxStatusParts(2, 0, config.Default())
	if !strings.Contains(out, "●") {
		t.Errorf("expected waiting icon: %q", out)
	}
	if strings.Contains(out, "✗") {
		t.Errorf("unexpected error icon: %q", out)
	}
}

func TestTmuxStatusParts_ErrorOnly(t *testing.T) {
	out := tmuxStatusParts(0, 3, config.Default())
	if !strings.Contains(out, "✗") {
		t.Errorf("expected error icon: %q", out)
	}
	if strings.Contains(out, "●") {
		t.Errorf("unexpected waiting icon: %q", out)
	}
}
