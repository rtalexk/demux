package cmd

import (
	"strings"
	"testing"

	"github.com/rtalexk/demux/internal/config"
	"github.com/rtalexk/demux/internal/db"
)

func TestTmuxStatusParts_NoStates(t *testing.T) {
	cfg := config.Default()
	out := tmuxStatusParts(nil, cfg)
	if !strings.Contains(out, cfg.Theme.IconStatusClean) {
		t.Errorf("expected clean icon %q, got: %q", cfg.Theme.IconStatusClean, out)
	}
}

func TestTmuxStatusParts_GroupsConfiguredToolsByState(t *testing.T) {
	cfg := config.Default()
	states := []db.ToolState{
		{Tool: "opencode", Value: db.StateWaiting},
		{Tool: "claude", Value: db.StateError},
	}

	out := tmuxStatusParts(states, cfg)
	errGroup := strings.Index(out, cfg.Tools["claude"].Icon)
	errState := strings.Index(out, cfg.Theme.IconStateError)
	waitingGroup := strings.Index(out, cfg.Tools["opencode"].Icon)
	waitingState := strings.Index(out, cfg.Theme.IconStateWaiting)
	if errGroup == -1 || errState == -1 || waitingGroup == -1 || waitingState == -1 {
		t.Fatalf("expected configured tool and state icons in: %q", out)
	}
	if !(errGroup < errState && errState < waitingGroup && waitingGroup < waitingState) {
		t.Errorf("expected error group before waiting group in: %q", out)
	}
}

func TestTmuxStatusParts_UsesFallbackForUnknownTool(t *testing.T) {
	cfg := config.Default()
	out := tmuxStatusParts([]db.ToolState{{Tool: "local-runner", Value: db.StateWaiting}}, cfg)

	if !strings.Contains(out, "? local-…") {
		t.Errorf("expected bounded fallback marker in: %q", out)
	}
	if !strings.Contains(out, cfg.Theme.IconStateWaiting) {
		t.Errorf("expected waiting icon in: %q", out)
	}
}

func TestTmuxStatusParts_UserFlagHasNoToolMarker(t *testing.T) {
	cfg := config.Default()
	out := tmuxStatusParts([]db.ToolState{{
		Tool:   "opencode",
		Value:  db.StateFlagged,
		Source: db.SourceUser,
	}}, cfg)

	if !strings.Contains(out, cfg.Theme.IconStateFlagged) {
		t.Errorf("expected flagged icon in: %q", out)
	}
	if strings.Contains(out, cfg.Tools["opencode"].Icon) || strings.Contains(out, "?") {
		t.Errorf("expected no tool marker for user flag in: %q", out)
	}
}

func TestTextStatusParts_UsesStableToolStateTokens(t *testing.T) {
	cfg := config.Default()
	states := []db.ToolState{
		{Tool: "opencode", Value: db.StateWaiting},
		{Tool: "claude", Value: db.StateError},
		{Tool: "opencode", Value: db.StateWaiting},
	}

	if got, want := textStatusParts(states, cfg), "claude.error=1 opencode.waiting=2"; got != want {
		t.Errorf("textStatusParts() = %q, want %q", got, want)
	}
}

func TestTextStatusParts_AggregatesUserFlagsAcrossToolIDs(t *testing.T) {
	cfg := config.Default()
	states := []db.ToolState{
		{Tool: "claude", Value: db.StateFlagged, Source: db.SourceUser},
		{Tool: "opencode", Value: db.StateFlagged, Source: db.SourceUser},
	}

	if got, want := textStatusParts(states, cfg), "user.flagged=2"; got != want {
		t.Errorf("textStatusParts() = %q, want %q", got, want)
	}
}
