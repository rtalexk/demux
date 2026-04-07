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
		{Value: db.StateFlagged},
		{Value: db.StateFlagged},
	}
	waiting, errs, flagged := countStatesByValue(states)
	if waiting != 2 {
		t.Errorf("waiting: want 2, got %d", waiting)
	}
	if errs != 1 {
		t.Errorf("errors: want 1, got %d", errs)
	}
	if flagged != 2 {
		t.Errorf("flagged: want 2, got %d", flagged)
	}
}

func TestTmuxStatusParts_NoStates(t *testing.T) {
	cfg := config.Default()
	out := tmuxStatusParts(0, 0, 0, cfg)
	if !strings.Contains(out, cfg.Theme.IconStateDone) {
		t.Errorf("expected done icon %q, got: %q", cfg.Theme.IconStateDone, out)
	}
}

func TestTmuxStatusParts_OrderErrorFlaggedWaiting(t *testing.T) {
	cfg := config.Default()
	out := tmuxStatusParts(1, 1, 1, cfg)
	errIdx := strings.Index(out, cfg.Theme.IconStateError)
	flagIdx := strings.Index(out, cfg.Theme.IconStateFlagged)
	waitIdx := strings.Index(out, cfg.Theme.IconStateWaiting)
	if errIdx == -1 || flagIdx == -1 || waitIdx == -1 {
		t.Fatalf("missing icon in: %q", out)
	}
	if !(errIdx < flagIdx && flagIdx < waitIdx) {
		t.Errorf("expected order error < flagged < waiting, got positions %d %d %d", errIdx, flagIdx, waitIdx)
	}
}

func TestTmuxStatusParts_WaitingOnly(t *testing.T) {
	cfg := config.Default()
	out := tmuxStatusParts(2, 0, 0, cfg)
	if !strings.Contains(out, cfg.Theme.IconStateWaiting) {
		t.Errorf("expected waiting icon: %q", out)
	}
	if strings.Contains(out, cfg.Theme.IconStateError) {
		t.Errorf("unexpected error icon: %q", out)
	}
}

func TestTmuxStatusParts_FlaggedOnly(t *testing.T) {
	cfg := config.Default()
	out := tmuxStatusParts(0, 0, 1, cfg)
	if !strings.Contains(out, cfg.Theme.IconStateFlagged) {
		t.Errorf("expected flagged icon: %q", out)
	}
	if strings.Contains(out, cfg.Theme.IconStateDone) {
		t.Errorf("unexpected done icon when states present: %q", out)
	}
}

func TestTmuxStatusParts_UsesThemeColors(t *testing.T) {
	cfg := config.Default()
	out := tmuxStatusParts(0, 1, 0, cfg)
	if !strings.Contains(out, cfg.Theme.ColorStateError) {
		t.Errorf("expected error color %q in output: %q", cfg.Theme.ColorStateError, out)
	}
}
