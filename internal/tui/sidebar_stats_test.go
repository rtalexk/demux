package tui

import (
	"strings"
	"testing"

	"github.com/rtalexk/demux/internal/config"
)

// coffeeGlyph is the Nerd-Font nf-fa-coffee (U+F0F4) used in these tests.
const coffeeGlyph = "\uf0f4"

func TestSidebar_CaffeineIndicatorShown(t *testing.T) {
	initStyles(Theme{IconCaffeine: coffeeGlyph}, config.ProcessesConfig{}, nil)
	var s SidebarModel
	s.nodes = []SidebarNode{{Session: "brewing"}}
	s.SetSessionStats(map[string]SessionStat{
		"brewing": {Caffeinated: true},
	})
	out := s.renderNode(s.nodes[0], false, false, 30)
	if !strings.Contains(out, coffeeGlyph) {
		t.Errorf("expected coffee glyph in row, got %q", out)
	}
}

func TestSidebar_NoCaffeineIndicatorWhenAbsent(t *testing.T) {
	initStyles(Theme{IconCaffeine: coffeeGlyph}, config.ProcessesConfig{}, nil)
	var s SidebarModel
	s.nodes = []SidebarNode{{Session: "plain"}}
	s.SetSessionStats(map[string]SessionStat{
		"plain": {Caffeinated: false},
	})
	out := s.renderNode(s.nodes[0], false, false, 30)
	if strings.Contains(out, coffeeGlyph) {
		t.Errorf("did not expect coffee glyph in row, got %q", out)
	}
}
