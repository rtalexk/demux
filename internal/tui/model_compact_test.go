package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rtalexk/demux/internal/config"
	"github.com/rtalexk/demux/internal/db"
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

func TestFullView_StatusBarHidden(t *testing.T) {
	cfg := config.Default()
	cfg.StatusBar.Show = false
	m := New(cfg, nil)
	m.width = 80
	m.height = 24

	view := m.View()

	// status bar contains "q:quit" — must be absent when hidden
	if strings.Contains(view, "q:quit") {
		t.Error("full View() must not render status bar when StatusBar.Show = false")
	}
}

func TestFullView_StatusBarVisible(t *testing.T) {
	cfg := config.Default()
	// cfg.StatusBar.Show defaults to true
	m := New(cfg, nil)
	m.width = 80
	m.height = 24

	view := m.View()

	if !strings.Contains(view, "q:quit") {
		t.Error("full View() must render status bar when StatusBar.Show = true")
	}
}

func TestCompactView_StatusBarHidden(t *testing.T) {
	cfg := config.Default()
	cfg.Mode = "compact"
	cfg.StatusBar.Show = false
	m := New(cfg, nil)
	m.width = 60 // wide enough that "q:quit" would appear if status bar rendered
	m.height = 20

	view := m.View()

	if strings.Contains(view, "q:quit") {
		t.Error("compact View() must not render status bar when StatusBar.Show = false")
	}
}

func TestCompactView_StatusBarVisible(t *testing.T) {
	cfg := config.Default()
	cfg.Mode = "compact"
	// cfg.StatusBar.Show defaults to true
	m := New(cfg, nil)
	m.width = 60 // wide enough that "q:quit" fits without lipgloss truncation
	m.height = 20

	view := m.View()

	if !strings.Contains(view, "q:quit") {
		t.Error("compact View() must render status bar when StatusBar.Show = true")
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

// setupModelWithSessionAndStates returns a Model with one sidebar session
// "work" ($1) and the given states loaded.
func setupModelWithSessionAndStates(states []db.ToolState) Model {
	m := New(config.Default(), nil)
	m.sidebar.nodes = []SidebarNode{{Session: "work"}}
	m.sidebar.cursor = 0
	m.sidebar.nameToID = map[string]string{"work": "$1"}
	m.sidebar.states = states
	return m
}

func TestBuildProcTitle_IgnoresPaneState(t *testing.T) {
	// Pane state in session "$1" — must NOT appear in the Proclist title.
	m := setupModelWithSessionAndStates([]db.ToolState{
		{
			Target:  db.Target{Type: db.TargetTypePane, ID: "%1", SessionID: "$1", PaneID: "%1"},
			Value:   db.StateError,
			Message: "PANE-STATE-MUST-NOT-APPEAR",
		},
	})

	title := m.buildProcTitle()
	if strings.Contains(title, "PANE-STATE-MUST-NOT-APPEAR") {
		t.Error("buildProcTitle must not include pane state in session title")
	}
}

func TestBuildProcTitle_ShowsSessionTargetState(t *testing.T) {
	// Session-level state — MUST appear in the Proclist title.
	m := setupModelWithSessionAndStates([]db.ToolState{
		{
			Target:  db.Target{Type: db.TargetTypeSession, ID: "$1", SessionID: "$1"},
			Value:   db.StateWorking,
			Message: "SESSION-STATE-MUST-APPEAR",
		},
	})

	title := m.buildProcTitle()
	if !strings.Contains(title, "SESSION-STATE-MUST-APPEAR") {
		t.Errorf("buildProcTitle must show session-level state badge; title=%q", title)
	}
}

func TestModelProcTitle_ToolIndicatorPrecedesLifecycleIcon(t *testing.T) {
	m := setupModelWithSessionAndStates([]db.ToolState{{
		Target: db.Target{Type: db.TargetTypeSession, ID: "$1", SessionID: "$1"},
		Tool:   "opencode",
		Value:  db.StateWaiting,
	}})
	initStyles(Theme{IconStateWaiting: "W"}, config.ProcessesConfig{}, nil)

	got := stripANSI(m.buildProcTitle())
	if !strings.Contains(got, "O W") {
		t.Errorf("process title must show tool marker immediately before lifecycle icon: %q", got)
	}
}

func TestBuildProcTitle_PaneStateTakesPrecedenceButSessionTitleIgnoresIt(t *testing.T) {
	// Pane has higher-priority state (error) than session (working).
	// Sidebar would show error; title must only show the session-level working state.
	m := setupModelWithSessionAndStates([]db.ToolState{
		{
			Target:  db.Target{Type: db.TargetTypePane, ID: "%1", SessionID: "$1", PaneID: "%1"},
			Value:   db.StateError,
			Message: "PANE-ERROR-MUST-NOT-APPEAR",
		},
		{
			Target:  db.Target{Type: db.TargetTypeSession, ID: "$1", SessionID: "$1"},
			Value:   db.StateWorking,
			Message: "SESSION-WORKING-MUST-APPEAR",
		},
	})

	title := m.buildProcTitle()
	if strings.Contains(title, "PANE-ERROR-MUST-NOT-APPEAR") {
		t.Error("buildProcTitle must not leak pane state into session title")
	}
	if !strings.Contains(title, "SESSION-WORKING-MUST-APPEAR") {
		t.Errorf("buildProcTitle must show session-level state; title=%q", title)
	}
}
