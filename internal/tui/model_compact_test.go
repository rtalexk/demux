package tui

import (
    "strings"
    "testing"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/rtalex/demux/internal/config"
)

func TestCompactView_OmitsProcList(t *testing.T) {
    cfg := config.Default()
    cfg.Mode = "compact"
    m := New(cfg, nil)
    m.width = 40
    m.height = 20

    view := m.View()

    // proclist border title always contains "[l]"
    if strings.Contains(view, "[l]") {
        t.Error("compact View() must not render the proc list panel")
    }
}

func TestCompactView_ContainsSidebar(t *testing.T) {
    cfg := config.Default()
    cfg.Mode = "compact"
    m := New(cfg, nil)
    m.width = 40
    m.height = 20

    view := m.View()

    // sidebar border title always contains "[h]"
    if !strings.Contains(view, "[h]") {
        t.Error("compact View() must render the sidebar panel")
    }
}

func TestFullView_ContainsProcList(t *testing.T) {
    cfg := config.Default()
    // cfg.Mode defaults to "full"
    m := New(cfg, nil)
    m.width = 80
    m.height = 24

    view := m.View()

    if !strings.Contains(view, "[l]") {
        t.Error("full View() must render the proc list panel")
    }
}

func TestCompactUpdate_FocusProcListIsNoop(t *testing.T) {
    cfg := config.Default()
    cfg.Mode = "compact"
    m := New(cfg, nil)
    m.width = 40
    m.height = 20

    // simulate pressing 'l' (FocusProcList)
    msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")}
    newModel, _ := m.Update(msg)
    updated := newModel.(Model)

    if updated.focus != panelSidebar {
        t.Errorf("compact mode: pressing 'l' should not move focus off panelSidebar, got %v", updated.focus)
    }
}
