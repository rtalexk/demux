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

func TestSidebar_StatLinesUnderFocusedRow(t *testing.T) {
	initStyles(Theme{IconCaffeine: coffeeGlyph}, config.ProcessesConfig{}, nil)
	var s SidebarModel
	s.cfg.Sidebar.ShowSessionStats = true
	s.nodes = []SidebarNode{{Session: "work"}}
	s.cursor = 0
	s.SetSessionStats(map[string]SessionStat{
		"work": {CPUNow: 9, MemNow: 1024 * 1024 * 1024, CPUPeak: 45, MemPeak: 1610612736},
	})
	out := s.renderNode(s.nodes[0], true, true, 40)
	if !strings.Contains(out, "cpu") || !strings.Contains(out, "peak") {
		t.Errorf("expected cpu/peak stat lines under focused row, got %q", out)
	}
	if !strings.Contains(out, "1.0GB") {
		t.Errorf("expected humanized mem in stat lines, got %q", out)
	}
}

func TestSidebar_NoStatLinesOnUnfocusedRow(t *testing.T) {
	initStyles(Theme{IconCaffeine: coffeeGlyph}, config.ProcessesConfig{}, nil)
	var s SidebarModel
	s.cfg.Sidebar.ShowSessionStats = true
	s.nodes = []SidebarNode{{Session: "work"}, {Session: "other"}}
	s.cursor = 0
	s.SetSessionStats(map[string]SessionStat{
		"other": {CPUNow: 1, MemNow: 1024 * 1024},
	})
	out := s.renderNode(s.nodes[1], false, false, 40) // not the cursor row
	if strings.Contains(out, "cpu") {
		t.Errorf("did not expect stat lines on non-cursor row, got %q", out)
	}
}

func TestNodeHeight_ExpandsCursorRowWhenStatsEnabled(t *testing.T) {
	initStyles(Theme{}, config.ProcessesConfig{}, nil)
	var s SidebarModel
	s.cfg.Sidebar.ShowSessionStats = true
	s.nodes = []SidebarNode{{Session: "work"}}
	s.cursor = 0
	s.SetSessionStats(map[string]SessionStat{"work": {CPUNow: 1, MemNow: 1}})
	if got := s.nodeHeight(0, true); got != 3 { // list base 1 + 2 stat lines
		t.Errorf("nodeHeight(cursor) = %d, want 3 (base + 2 stat lines)", got)
	}
}

func TestNodeHeight_DisabledHasNoExtraRows(t *testing.T) {
	initStyles(Theme{}, config.ProcessesConfig{}, nil)
	var s SidebarModel
	s.cfg.Sidebar.ShowSessionStats = false
	s.nodes = []SidebarNode{{Session: "work"}}
	s.cursor = 0
	s.SetSessionStats(map[string]SessionStat{"work": {CPUNow: 1, MemNow: 1}})
	if got := s.nodeHeight(0, true); got != 1 {
		t.Errorf("nodeHeight = %d, want 1 when stats disabled", got)
	}
}
