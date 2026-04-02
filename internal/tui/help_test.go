package tui

import (
    "strings"
    "testing"

    "github.com/rtalex/demux/internal/config"
)

func TestHelpSection_ContainsSectionName(t *testing.T) {
    initStyles(Theme{}, config.ProcessesConfig{}, nil)
    out := stripANSI(helpSection("MySection"))
    if !strings.Contains(out, "MySection") {
        t.Errorf("expected section name in output, got: %q", out)
    }
}

func TestHelpSection_HasSeparatorChars(t *testing.T) {
    initStyles(Theme{}, config.ProcessesConfig{}, nil)
    out := stripANSI(helpSection("Test"))
    if !strings.Contains(out, "─") {
        t.Errorf("expected separator char ─, got: %q", out)
    }
}

func TestHelpSection_StartsWithSeparatorPrefix(t *testing.T) {
    initStyles(Theme{}, config.ProcessesConfig{}, nil)
    out := stripANSI(helpSection("Test"))
    if !strings.HasPrefix(out, "─── ") {
        t.Errorf("expected '─── ' prefix, got: %q", out)
    }
}

func TestHelpSection_TotalWidth(t *testing.T) {
    initStyles(Theme{}, config.ProcessesConfig{}, nil)
    out := stripANSI(helpSection("Global"))
    runes := []rune(out)
    if len(runes) != helpContentWidth {
        t.Errorf("expected width %d, got %d: %q", helpContentWidth, len(runes), out)
    }
}

func TestHelpRender_AllSectionHeaders(t *testing.T) {
    initStyles(Theme{}, config.ProcessesConfig{}, nil)
    out := stripANSI(HelpModel{}.Render(0))
    for _, section := range []string{"Global", "Navigation", "Sidebar", "Filters", "Process list"} {
        if !strings.Contains(out, section) {
            t.Errorf("expected section %q in help output", section)
        }
    }
}

func TestHelpRender_KeyBindings(t *testing.T) {
    initStyles(Theme{}, config.ProcessesConfig{}, nil)
    out := stripANSI(HelpModel{}.Render(0))
    for _, want := range []string{
        "h / l",   // corrected focus keys (was 1/2)
        "ctrl+u",  // clear filter
        "Shift+Tab", // backward navigate
        "G",       // goto bottom
        "] / [",   // expand/collapse group
        "} / {",   // expand/collapse all
        "L",       // log popup (uppercase, was lowercase l)
    } {
        if !strings.Contains(out, want) {
            t.Errorf("expected %q in help output:\n%s", want, out)
        }
    }
}
