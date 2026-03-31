package tui

import (
    "testing"
    "time"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/rtalex/demux/internal/config"
    "github.com/rtalex/demux/internal/db"
    "github.com/rtalex/demux/internal/tmux"
)

func focusTestModel(focusOnOpen string) Model {
    cfg := config.Default()
    cfg.Sidebar.FocusOnOpen = focusOnOpen
    database, _ := db.Open(":memory:")
    return New(cfg, database)
}

func applyPanesMsg(m Model, currentSession string) (Model, tea.Cmd) {
    panes := []tmux.Pane{
        {Session: "alpha", WindowIndex: 0},
        {Session: "alpha", WindowIndex: 1},
        {Session: "beta", WindowIndex: 0},
    }
    msg := panesMsg{panes: panes, currentSession: currentSession}
    updated, cmd := m.Update(msg)
    return updated.(Model), cmd
}

func applyAlertsMsg(m Model, alerts []db.Alert) (Model, tea.Cmd) {
    updated, cmd := m.Update(alertsMsg{alerts: alerts})
    return updated.(Model), cmd
}

func TestFocusOnOpen_CurrentSession(t *testing.T) {
    m := focusTestModel("current_session")
    m.height = 40
    m, _ = applyPanesMsg(m, "beta")
    node := m.sidebar.Selected()
    if node == nil || node.Session != "beta" {
        t.Errorf("expected session node beta, got %+v", node)
    }
}

func TestFocusOnOpen_AlertSession(t *testing.T) {
    m := focusTestModel("alert_session")
    m.height = 40
    m, _ = applyPanesMsg(m, "alpha")
    alerts := []db.Alert{
        {Target: "beta:0", Level: "warn", CreatedAt: time.Now()},
    }
    m, _ = applyAlertsMsg(m, alerts)
    node := m.sidebar.Selected()
    if node == nil || node.Session != "beta" {
        t.Errorf("expected session node beta, got %+v", node)
    }
}

func TestFocusOnOpen_FirstSession(t *testing.T) {
    m := focusTestModel("first_session")
    m.height = 40
    m, _ = applyPanesMsg(m, "alpha")
    node := m.sidebar.Selected()
    if node == nil {
        t.Errorf("expected a session node, got nil")
    }
}

func TestFocusSearchOnOpen_true(t *testing.T) {
    cfg := config.Default()
    cfg.Sidebar.FocusSearchOnOpen = true
    database, _ := db.Open(":memory:")
    m := New(cfg, database)
    m.height = 40
    m, _ = applyPanesMsg(m, "alpha")
    m, _ = applyAlertsMsg(m, nil)
    if !m.searchInput.IsInsert() {
        t.Errorf("expected searchInput to be in insert mode when FocusSearchOnOpen = true")
    }
}

func TestFocusSearchOnOpen_false(t *testing.T) {
    cfg := config.Default()
    cfg.Sidebar.FocusSearchOnOpen = false
    database, _ := db.Open(":memory:")
    m := New(cfg, database)
    m.height = 40
    m, _ = applyPanesMsg(m, "alpha")
    m, _ = applyAlertsMsg(m, nil)
    if m.searchInput.IsInsert() {
        t.Errorf("expected searchInput to not be in insert mode when FocusSearchOnOpen = false")
    }
}
