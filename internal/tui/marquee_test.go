package tui

import (
	"testing"

	runewidth "github.com/mattn/go-runewidth"
)

func TestMarqueeView_fits(t *testing.T) {
	var m Marquee
	got := m.View("hello", 10)
	if got != "hello" {
		t.Errorf("want %q, got %q", "hello", got)
	}
}

func TestMarqueeView_offset0(t *testing.T) {
	var m Marquee
	got := m.View("ABCDEF", 3)
	if got != "ABC" {
		t.Errorf("want %q, got %q", "ABC", got)
	}
}

func TestMarqueeView_scroll(t *testing.T) {
	m := Marquee{offset: 1}
	got := m.View("ABCDEF", 3)
	if got != "BCD" {
		t.Errorf("want %q, got %q", "BCD", got)
	}
}

func TestMarqueeView_gap(t *testing.T) {
	// "ABCDEF" = 6 runes, gap = 4, cycle = 10
	// offset=6 → extended[6..8] are spaces
	m := Marquee{offset: 6}
	got := m.View("ABCDEF", 3)
	if got != "   " {
		t.Errorf("want 3 spaces, got %q", got)
	}
}

func TestMarqueeView_wrap(t *testing.T) {
	// offset=10 = one full cycle → back to start
	m := Marquee{offset: 10}
	got := m.View("ABCDEF", 3)
	if got != "ABC" {
		t.Errorf("want %q, got %q", "ABC", got)
	}
}

func TestMarqueeView_exactWidth(t *testing.T) {
	// offset=5 puts us into the gap; result must still be exactly width cols
	m := Marquee{offset: 5}
	got := m.View("HELLO", 4)
	if cols := runewidth.StringWidth(got); cols != 4 {
		t.Errorf("want 4 display cols, got %d (%q)", cols, got)
	}
}

func TestMarqueeView_multibyte(t *testing.T) {
	// '中' is 2 display columns; "中文字" = 6 cols, width=4 → 2 chars fit exactly
	var m Marquee
	got := m.View("中文字", 4)
	if cols := runewidth.StringWidth(got); cols != 4 {
		t.Errorf("want 4 display cols, got %d (%q)", cols, got)
	}
}

func TestMarqueeReset(t *testing.T) {
	m := Marquee{offset: 5}
	m.Reset()
	got := m.View("ABCDEF", 3)
	if got != "ABC" {
		t.Errorf("after Reset, want %q, got %q", "ABC", got)
	}
}

func TestMarqueeTick(t *testing.T) {
	var m Marquee
	m.Tick()
	got := m.View("ABCDEF", 3)
	if got != "BCD" {
		t.Errorf("after one Tick, want %q, got %q", "BCD", got)
	}
}
