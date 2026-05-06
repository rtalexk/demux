package procmatch_test

import (
	"testing"

	"github.com/rtalexk/demux/internal/procmatch"
)

func TestPatternZeroValue(t *testing.T) {
	var p procmatch.Pattern
	if p.Match != "" || p.Label != "" || p.FG != "" || p.BG != "" {
		t.Fatalf("expected zero-valued Pattern, got %+v", p)
	}
}

func TestLabelZeroValue(t *testing.T) {
	var l procmatch.Label
	if l.Text != "" || l.FG != "" || l.BG != "" {
		t.Fatalf("expected zero-valued Label, got %+v", l)
	}
}
