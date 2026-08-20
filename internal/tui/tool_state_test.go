package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/rtalexk/demux/internal/config"
	"github.com/rtalexk/demux/internal/db"
)

func TestToolStateIndicator(t *testing.T) {
	cfg := config.Default()
	cfg.Tools = map[string]config.ToolConfig{
		"opencode": {Name: "OpenCode", Icon: "O", Color: "#89b4fa"},
	}
	initStyles(ThemeFromConfig(cfg.Theme), config.ProcessesConfig{}, nil)

	tests := []struct {
		name  string
		state db.ToolState
		want  string
	}{
		{
			name:  "uses configured tool icon",
			state: db.ToolState{Tool: "opencode", Value: db.StateWorking},
			want:  "O " + cfg.Theme.IconStateWorking,
		},
		{
			name:  "uses generic marker and bounded label for unknown tool",
			state: db.ToolState{Tool: "αβγδεζηθ", Value: db.StateWorking},
			want:  "? αβγδεζ… " + cfg.Theme.IconStateWorking,
		},
		{
			name:  "omits marker for empty tool",
			state: db.ToolState{Value: db.StateWorking},
			want:  cfg.Theme.IconStateWorking,
		},
		{
			name:  "omits marker for user flag",
			state: db.ToolState{Tool: "opencode", Source: db.SourceUser, Value: db.StateFlagged},
			want:  cfg.Theme.IconStateFlagged,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripANSI(toolStateIndicator(tt.state, cfg, "")); got != tt.want {
				t.Errorf("toolStateIndicator() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestToolStateIndicator_DistinguishesUnknownTools(t *testing.T) {
	cfg := config.Default()
	cfg.Tools = map[string]config.ToolConfig{}
	initStyles(ThemeFromConfig(cfg.Theme), config.ProcessesConfig{}, nil)

	first := stripANSI(toolStateIndicator(db.ToolState{Tool: "local-runner", Value: db.StateWaiting}, cfg, ""))
	second := stripANSI(toolStateIndicator(db.ToolState{Tool: "remote-runner", Value: db.StateWaiting}, cfg, ""))
	if first != "? local-… "+cfg.Theme.IconStateWaiting {
		t.Errorf("first unknown tool indicator = %q", first)
	}
	if second != "? remote… "+cfg.Theme.IconStateWaiting {
		t.Errorf("second unknown tool indicator = %q", second)
	}
}

func TestToolStateIndicator_OnBackground(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

	cfg := config.Default()
	cfg.Tools = map[string]config.ToolConfig{
		"opencode": {Name: "OpenCode", Icon: "O", Color: "#89b4fa"},
	}
	initStyles(ThemeFromConfig(cfg.Theme), config.ProcessesConfig{}, nil)

	got := toolStateIndicator(db.ToolState{Tool: "opencode", Value: db.StateWorking}, cfg, lipgloss.Color("#2a2a4a"))
	if plain := stripANSI(got); plain != "O "+cfg.Theme.IconStateWorking {
		t.Errorf("toolStateIndicator() = %q, want %q", plain, "O "+cfg.Theme.IconStateWorking)
	}
	if !strings.Contains(got, ";48") {
		t.Errorf("toolStateIndicator() missing background ANSI code: %q", got)
	}
	if strings.Contains(got, "\x1b[0m ") {
		t.Errorf("toolStateIndicator() leaves the separator outside the selected background: %q", got)
	}
}

func TestToolStateIndicator_UsesMutedColorForConfiguredToolWithoutColor(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

	cfg := config.Default()
	cfg.Theme.ColorFgMuted = "#102030"
	cfg.Tools = map[string]config.ToolConfig{
		"no-color": {Name: "No Color", Icon: "N"},
	}
	initStyles(ThemeFromConfig(cfg.Theme), config.ProcessesConfig{}, nil)

	got := toolStateIndicator(db.ToolState{Tool: "no-color", Value: db.StateWorking}, cfg, "")
	if !strings.Contains(got, "\x1b[38;2;16;32;48mN") {
		t.Errorf("toolStateIndicator() = %q, want marker with muted foreground color", got)
	}
}

func TestToolStateLabel(t *testing.T) {
	cfg := config.Default()
	cfg.Tools = map[string]config.ToolConfig{
		"opencode": {Name: "OpenCode", Icon: "O", Color: "#89b4fa"},
		"unnamed":  {Icon: "U", Color: "#89b4fa"},
	}

	tests := []struct {
		name  string
		state db.ToolState
		want  string
	}{
		{
			name:  "uses configured tool name",
			state: db.ToolState{Tool: "opencode"},
			want:  "OpenCode",
		},
		{
			name:  "falls back to configured tool ID when name is empty",
			state: db.ToolState{Tool: "unnamed"},
			want:  "unnamed",
		},
		{
			name:  "uses unknown tool ID",
			state: db.ToolState{Tool: "local-runner"},
			want:  "local-…",
		},
		{
			name:  "truncates unknown tool ID by runes",
			state: db.ToolState{Tool: "αβγδεζηθ"},
			want:  "αβγδεζ…",
		},
		{
			name:  "omits empty tool",
			state: db.ToolState{},
			want:  "",
		},
		{
			name:  "omits user flag",
			state: db.ToolState{Tool: "opencode", Source: db.SourceUser, Value: db.StateFlagged},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toolStateLabel(tt.state, cfg); got != tt.want {
				t.Errorf("toolStateLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}
