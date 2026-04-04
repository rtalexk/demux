package tui

import (
	"testing"
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
