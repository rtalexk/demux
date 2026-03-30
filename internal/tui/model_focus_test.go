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
    m.cfg.FocusOnOpenFallback = "" // no fallback — cursor must not move
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

func TestFocusOnOpen_FirstWindow(t *testing.T) {
    m := focusTestModel("first_window")
    m.height = 40
    m, _ = applyPanesMsg(m, "alpha", 0)
    node := m.sidebar.Selected()
    if node == nil || node.IsSession {
        t.Errorf("expected a window node, got %+v", node)
    }
}

func TestFocusOnOpen_FirstSession(t *testing.T) {
    m := focusTestModel("first_session")
    m.height = 40
    m, _ = applyPanesMsg(m, "alpha", 0)
    node := m.sidebar.Selected()
    if node == nil || !node.IsSession {
        t.Errorf("expected a session node, got %+v", node)
    }
}

func TestFocusOnOpen_AlertWindow_FallsBackToCurrentWindow_WhenNoAlerts(t *testing.T) {
    m := focusTestModel("alert_window")
    m.height = 40
    m, _ = applyPanesMsg(m, "beta", 0)
    // No alerts — should fall back to current_window (beta:0)
    m, _ = applyAlertsMsg(m, nil)
    node := m.sidebar.Selected()
    if node == nil || node.IsSession || node.Session != "beta" || node.WindowIndex != 0 {
        t.Errorf("expected fallback to beta:0 (current_window), got %+v", node)
    }
}

func TestFocusOnOpen_AlertSession_FallsBackToCurrentWindow_WhenNoAlerts(t *testing.T) {
    m := focusTestModel("alert_session")
    m.height = 40
    m, _ = applyPanesMsg(m, "beta", 0)
    // No alerts — should fall back to current_window (beta:0)
    m, _ = applyAlertsMsg(m, nil)
    node := m.sidebar.Selected()
    if node == nil || node.IsSession || node.Session != "beta" || node.WindowIndex != 0 {
        t.Errorf("expected fallback to beta:0 (current_window), got %+v", node)
    }
}

func TestFocusOnOpen_AlertWindow_NoFallback_WhenFallbackEmpty(t *testing.T) {
    m := focusTestModel("alert_window")
    m.cfg.FocusOnOpenFallback = ""
    m.height = 40
    m, _ = applyPanesMsg(m, "beta", 0)
    // No alerts, no fallback — cursor stays at 0
    m, _ = applyAlertsMsg(m, nil)
    if m.sidebar.cursor != 0 {
        t.Errorf("expected cursor=0 with no fallback, got %d", m.sidebar.cursor)
    }
}

func TestFocusOnOpen_SubsequentAlerts_DoNotRefocus(t *testing.T) {
    m := focusTestModel("alert_window")
    m.height = 40
    m, _ = applyPanesMsg(m, "alpha", 0)
    // First alertsMsg: beta:0 has a warn alert — startup focus lands on beta:0
    alerts1 := []db.Alert{
        {Target: "beta:0", Level: "warn", CreatedAt: time.Now()},
    }
    m, _ = applyAlertsMsg(m, alerts1)
    // Second alertsMsg: alpha:1 now has an error alert (higher severity, newer) — this changes
    // sort order so alpha sorts before beta, shifting node indices.
    // The cursor should still point to beta:0 by identity, not by the original index.
    alerts2 := []db.Alert{
        {Target: "beta:0", Level: "warn", CreatedAt: time.Now().Add(-time.Second)},
        {Target: "alpha:1", Level: "error", CreatedAt: time.Now()},
    }
    m, _ = applyAlertsMsg(m, alerts2)
    node := m.sidebar.Selected()
    if node == nil || node.IsSession || node.Session != "beta" || node.WindowIndex != 0 {
        t.Errorf("expected cursor to remain on beta:0 after sort-order-changing alertsMsg, got %+v", node)
    }
}

func focusTestModelCollapsed(focusOnOpen string) Model {
    cfg := config.Default()
    cfg.FocusOnOpen = focusOnOpen
    cfg.SessionsCollapsed = true
    database, _ := db.Open(":memory:")
    return New(cfg, database)
}

// alert_window with all sessions collapsed → should land on the alert *session* node
func TestFocusOnOpen_AlertWindow_Collapsed_FallsBackToAlertSession(t *testing.T) {
    m := focusTestModelCollapsed("alert_window")
    m.cfg.FocusOnOpenFallback = ""
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

// current_window with all sessions collapsed → should land on the current *session* node
func TestFocusOnOpen_CurrentWindow_Collapsed_FallsBackToCurrentSession(t *testing.T) {
    m := focusTestModelCollapsed("current_window")
    m.height = 40
    m, _ = applyPanesMsg(m, "beta", 0)
    node := m.sidebar.Selected()
    if node == nil || !node.IsSession || node.Session != "beta" {
        t.Errorf("expected session node beta, got %+v", node)
    }
}

// first_window with all sessions collapsed → cursor stays at first session (index 0)
func TestFocusOnOpen_FirstWindow_Collapsed_FallsBackToFirstSession(t *testing.T) {
    m := focusTestModelCollapsed("first_window")
    m.height = 40
    m, _ = applyPanesMsg(m, "beta", 0)
    node := m.sidebar.Selected()
    if node == nil || !node.IsSession {
        t.Errorf("expected a session node, got %+v", node)
    }
}

// alert_window collapsed + no alerts → applies focus_on_open_fallback
func TestFocusOnOpen_AlertWindow_Collapsed_NoAlerts_AppliesFallback(t *testing.T) {
    m := focusTestModelCollapsed("alert_window")
    m.cfg.FocusOnOpenFallback = "current_window"
    m.height = 40
    m, _ = applyPanesMsg(m, "beta", 0)
    m, _ = applyAlertsMsg(m, nil)
    // No alerts + collapsed → fallback "current_window" also collapses → current session
    node := m.sidebar.Selected()
    if node == nil || !node.IsSession || node.Session != "beta" {
        t.Errorf("expected session node beta via fallback, got %+v", node)
    }
}
