package tui

import (
    "strings"
    "testing"

    "github.com/rtalexk/demux/internal/config"
)

func initSearchStyles() {
    initStyles(Theme{}, config.ProcessesConfig{}, nil)
}

// --- border shape ---

func TestSearchView_RoundedTopCorners(t *testing.T) {
    initSearchStyles()
    out := stripANSI(NewSearchInputModel().View(40))
    if !strings.Contains(out, "╭") || !strings.Contains(out, "╮") {
        t.Errorf("expected rounded top corners ╭ ╮, got: %q", out)
    }
}

func TestSearchView_RoundedBottomCorners(t *testing.T) {
    initSearchStyles()
    out := stripANSI(NewSearchInputModel().View(40))
    if !strings.Contains(out, "╰") || !strings.Contains(out, "╯") {
        t.Errorf("expected rounded bottom corners ╰ ╯, got: %q", out)
    }
}

// --- title ---

func TestSearchView_TitlePresent(t *testing.T) {
    initSearchStyles()
    out := stripANSI(NewSearchInputModel().View(40))
    if !strings.Contains(out, "[f] Search") {
        t.Errorf("expected '[f] Search' in output, got: %q", out)
    }
}

func TestSearchView_TitleNotWrappedInBorderColor(t *testing.T) {
    initSearchStyles()
    // Title must appear verbatim in the raw output (not inside an ANSI span).
    raw := NewSearchInputModel().View(40)
    if !strings.Contains(raw, "[f] Search") {
        t.Errorf("expected '[f] Search' to appear uncolored in raw output, got: %q", raw)
    }
}

// --- placeholder ---

func TestSearchView_DefaultPlaceholder(t *testing.T) {
    initSearchStyles()
    out := stripANSI(NewSearchInputModel().View(40))
    if !strings.Contains(out, "Press f to search") {
        t.Errorf("expected default placeholder, got: %q", out)
    }
}

func TestSearchView_InsertModePlaceholder(t *testing.T) {
    initSearchStyles()
    m := NewSearchInputModel()
    m.EnterInsertMode()
    out := stripANSI(m.View(40))
    if !strings.Contains(out, "Type to search") {
        t.Errorf("expected insert-mode placeholder, got: %q", out)
    }
}

func TestSearchView_ExitInsertModeRestoresPlaceholder(t *testing.T) {
    initSearchStyles()
    m := NewSearchInputModel()
    m.EnterInsertMode()
    m.ExitInsertMode()
    out := stripANSI(m.View(40))
    if !strings.Contains(out, "Press f to search") {
        t.Errorf("expected default placeholder after exiting insert mode, got: %q", out)
    }
}
