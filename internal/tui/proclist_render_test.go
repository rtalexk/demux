package tui

import (
	"strings"
	"testing"

	"github.com/rtalexk/demux/internal/config"
	"github.com/rtalexk/demux/internal/db"
	"github.com/rtalexk/demux/internal/tmux"
)

func makeRenderedLines(n int) []renderedLine {
	lines := make([]renderedLine, n)
	for i := range lines {
		lines[i] = renderedLine{nodeIdx: i, text: "line"}
	}
	return lines
}

func TestComputeViewport(t *testing.T) {
	tests := []struct {
		name      string
		lines     []renderedLine
		cursor    int
		offset    int
		maxRows   int
		wantCount int
		wantAbove bool
		wantBelow bool
	}{
		{"empty", nil, 0, 0, 10, 0, false, false},
		{"all fit", makeRenderedLines(5), 0, 0, 10, 5, false, false},
		{"scroll below", makeRenderedLines(15), 0, 0, 5, 4, false, true},
		{"scroll above", makeRenderedLines(15), 14, 5, 5, 3, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			visible, hasAbove, hasBelow := computeViewport(tt.lines, tt.cursor, tt.offset, tt.maxRows)
			if len(visible) != tt.wantCount {
				t.Errorf("visible count: got %d, want %d", len(visible), tt.wantCount)
			}
			if hasAbove != tt.wantAbove {
				t.Errorf("hasAbove: got %v, want %v", hasAbove, tt.wantAbove)
			}
			if hasBelow != tt.wantBelow {
				t.Errorf("hasBelow: got %v, want %v", hasBelow, tt.wantBelow)
			}
		})
	}
}

func TestProcListSelectedToolIndicatorPrecedesLifecycleIcon(t *testing.T) {
	initStyles(Theme{IconStateWaiting: "W"}, config.ProcessesConfig{}, nil)
	p := ProcListModel{
		cfg: config.Default(),
		states: []db.ToolState{{
			Target: db.Target{Type: db.TargetTypePane, ID: "%1"},
			Tool:   "opencode",
			Value:  db.StateWaiting,
		}},
	}

	got := stripANSI(p.renderPaneHeader(ProcListNode{Pane: tmux.Pane{PaneIndex: 1, PaneID: "%1"}}, true, 40, false))
	if !strings.Contains(got, "O W") {
		t.Errorf("selected pane header must show tool marker immediately before lifecycle icon: %q", got)
	}
}

func TestProcListUnselectedToolIndicatorPrecedesLifecycleIcon(t *testing.T) {
	initStyles(Theme{IconStateWaiting: "W"}, config.ProcessesConfig{}, nil)
	p := ProcListModel{
		cfg: config.Default(),
		states: []db.ToolState{{
			Target: db.Target{Type: db.TargetTypePane, ID: "%1"},
			Tool:   "opencode",
			Value:  db.StateWaiting,
		}},
	}

	got := stripANSI(p.renderPaneHeader(ProcListNode{Pane: tmux.Pane{PaneIndex: 1, PaneID: "%1"}}, false, 40, false))
	if !strings.Contains(got, "O W") {
		t.Errorf("unselected pane header must show tool marker immediately before lifecycle icon: %q", got)
	}
}
