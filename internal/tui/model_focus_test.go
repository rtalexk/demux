package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rtalexk/demux/internal/config"
	"github.com/rtalexk/demux/internal/db"
	"github.com/rtalexk/demux/internal/tmux"
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

func applyStatesMsg(m Model, states []db.ToolState) (Model, tea.Cmd) {
	updated, cmd := m.Update(statesMsg{states: states})
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
	m, _ = applyStatesMsg(m, nil)
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
	m, _ = applyStatesMsg(m, nil)
	if m.searchInput.IsInsert() {
		t.Errorf("expected searchInput to not be in insert mode when FocusSearchOnOpen = false")
	}
}

// TestStateSession_StatesBeforePanes verifies that state_session focus and
// correct priority-based sort are applied even when the statesMsg arrives
// before the panesMsg (the race introduced by fetching both in Init).
func TestStateSession_StatesBeforePanes(t *testing.T) {
	states := []db.ToolState{
		{Target: db.Target{Type: "session", ID: "beta"}, Value: db.StateWaiting},
	}
	m := focusTestModel("state_session")
	m.height = 40
	// States arrive first (before panes).
	m, _ = applyStatesMsg(m, states)
	if m.startupFocusDone {
		t.Error("startupFocusDone must not be set before panes arrive")
	}
	// Panes arrive — sidebar is now populated with correct state sort and focus applied.
	m, _ = applyPanesMsg(m, "alpha")
	if !m.startupFocusDone {
		t.Error("startupFocusDone must be set after panes arrive with pre-loaded states")
	}
	node := m.sidebar.Selected()
	if node == nil || node.Session != "beta" {
		t.Errorf("expected state_session focus on beta (has waiting state), got %+v", node)
	}
}

// TestStateSession_PanesBeforeStates verifies the existing behaviour: when
// panes arrive before states (the common case), state_session focus is applied
// in handleStatesMsg once m.ready is true.
func TestStateSession_PanesBeforeStates(t *testing.T) {
	states := []db.ToolState{
		{Target: db.Target{Type: "session", ID: "beta"}, Value: db.StateWaiting},
	}
	m := focusTestModel("state_session")
	m.height = 40
	m, _ = applyPanesMsg(m, "alpha")
	if m.startupFocusDone {
		t.Error("startupFocusDone must not be set before states arrive")
	}
	m, _ = applyStatesMsg(m, states)
	if !m.startupFocusDone {
		t.Error("startupFocusDone must be set after states arrive with panes ready")
	}
	node := m.sidebar.Selected()
	if node == nil || node.Session != "beta" {
		t.Errorf("expected state_session focus on beta (has waiting state), got %+v", node)
	}
}

// TestStartupSort_StatesBeforePanes verifies that sessions are sorted by
// state priority on the first panes render when states arrived first.
func TestStartupSort_StatesBeforePanes(t *testing.T) {
	states := []db.ToolState{
		{Target: db.Target{Type: "session", ID: "beta"}, Value: db.StateError},
	}
	m := focusTestModel("")
	m.height = 40
	m, _ = applyStatesMsg(m, states)
	m, _ = applyPanesMsg(m, "alpha")
	nodes := m.sidebar.nodes
	if len(nodes) < 2 {
		t.Fatalf("expected at least 2 nodes, got %d", len(nodes))
	}
	if nodes[0].Session != "beta" {
		t.Errorf("expected beta (error state) to sort first, got %q", nodes[0].Session)
	}
}
