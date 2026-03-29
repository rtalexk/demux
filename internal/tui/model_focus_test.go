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
    cfg.FocusOnOpen = focusOnOpen
    database, _ := db.Open(":memory:")
    return New(cfg, database)
}

func applyPanesMsg(m Model, currentSession string, currentWindow int) (Model, tea.Cmd) {
    panes := []tmux.Pane{
        {Session: "alpha", WindowIndex: 0},
        {Session: "alpha", WindowIndex: 1},
        {Session: "beta", WindowIndex: 0},
    }
    msg := panesMsg{panes: panes, currentSession: currentSession, currentWindow: currentWindow}
    updated, cmd := m.Update(msg)
    return updated.(Model), cmd
}

func applyAlertsMsg(m Model, alerts []db.Alert) (Model, tea.Cmd) {
    updated, cmd := m.Update(alertsMsg{alerts: alerts})
    return updated.(Model), cmd
}

func TestFocusOnOpen_CurrentWindow(t *testing.T) {
    m := focusTestModel("current_window")
    m.height = 40
    m, _ = applyPanesMsg(m, "beta", 0)
    node := m.sidebar.Selected()
    if node == nil || node.IsSession || node.Session != "beta" || node.WindowIndex != 0 {
        t.Errorf("expected window node beta:0, got %+v", node)
    }
}

func TestFocusOnOpen_CurrentSession(t *testing.T) {
    m := focusTestModel("current_session")
    m.height = 40
    m, _ = applyPanesMsg(m, "beta", 0)
    node := m.sidebar.Selected()
    if node == nil || !node.IsSession || node.Session != "beta" {
        t.Errorf("expected session node beta, got %+v", node)
    }
}

func TestFocusOnOpen_AlertWindow(t *testing.T) {
    m := focusTestModel("alert_window")
    m.height = 40
    m, _ = applyPanesMsg(m, "alpha", 0)
    alerts := []db.Alert{
        {Target: "beta:0", Level: "warn", CreatedAt: time.Now()},
    }
    m, _ = applyAlertsMsg(m, alerts)
    node := m.sidebar.Selected()
    if node == nil || node.IsSession || node.Session != "beta" || node.WindowIndex != 0 {
        t.Errorf("expected window node beta:0, got %+v", node)
    }
}

func TestFocusOnOpen_AlertSession(t *testing.T) {
    m := focusTestModel("alert_session")
    m.height = 40
    m, _ = applyPanesMsg(m, "alpha", 0)
    alerts := []db.Alert{
        {Target: "beta:0", Level: "warn", CreatedAt: time.Now()},
    }
    m, _ = applyAlertsMsg(m, alerts)
    node := m.sidebar.Selected()
    if node == nil || !node.IsSession || node.Session != "beta" {
        t.Errorf("expected session node beta, got %+v", node)
    }
}

func TestFocusOnOpen_AlertWindow_NoAlerts_StaysAt0(t *testing.T) {
    m := focusTestModel("alert_window")
    m.height = 40
    m, _ = applyPanesMsg(m, "alpha", 0)
    m, _ = applyAlertsMsg(m, nil)
    node := m.sidebar.Selected()
    if node == nil {
        t.Fatal("expected a node, got nil")
    }
    // cursor should be at 0 — whatever that node is
    if m.sidebar.cursor != 0 {
        t.Errorf("expected cursor=0, got %d", m.sidebar.cursor)
    }
}

func TestFocusOnOpen_SubsequentAlerts_DoNotRefocus(t *testing.T) {
    m := focusTestModel("alert_window")
    m.height = 40
    m, _ = applyPanesMsg(m, "alpha", 0)
    alerts := []db.Alert{
        {Target: "beta:0", Level: "warn", CreatedAt: time.Now()},
    }
    m, _ = applyAlertsMsg(m, alerts)
    firstCursor := m.sidebar.cursor
    // Simulate a second alerts tick with a different alert
    alerts2 := []db.Alert{
        {Target: "alpha:1", Level: "error", CreatedAt: time.Now()},
    }
    m, _ = applyAlertsMsg(m, alerts2)
    if m.sidebar.cursor != firstCursor {
        t.Errorf("expected cursor to stay at %d after second alertsMsg, got %d", firstCursor, m.sidebar.cursor)
    }
}
