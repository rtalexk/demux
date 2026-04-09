package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	demuxlog "github.com/rtalexk/demux/internal/log"
)

// openChecklistMode switches the proclist panel to checklist mode for the given session.
func (m Model) openChecklistMode(session string) (Model, tea.Cmd) {
	m.checklistMode = true
	m.checklistSession = session
	m.checklistCursor = 0
	m.checklistInput.Exit()
	demuxlog.Info("checklist mode on", "session", session)
	return m, m.fetchTodos(session)
}

// handleChecklistKey handles key events when checklistMode is true, the proclist
// panel is focused, and the input bar is idle. Global keys (including C) are
// handled before this in handleKey and are not repeated here.
func (m Model) handleChecklistKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.checklistMode = false
		m.checklistInput.Exit()
		demuxlog.Debug("checklist mode off via esc", "session", m.checklistSession)
		return m, nil

	case "j", "down":
		if m.checklistCursor < len(m.checklistItems)-1 {
			m.checklistCursor++
		}
		return m, nil

	case "k", "up":
		if m.checklistCursor > 0 {
			m.checklistCursor--
		}
		return m, nil

	case "g":
		m.checklistCursor = 0
		return m, nil

	case "G":
		if len(m.checklistItems) > 0 {
			m.checklistCursor = len(m.checklistItems) - 1
		}
		return m, nil

	case "x":
		return m.checklistToggle()

	case "d":
		return m.checklistDeletePrompt()

	case "a":
		m.checklistInput.EnterAddMode()
		return m, nil

	case "e":
		if len(m.checklistItems) == 0 {
			return m, nil
		}
		item := m.checklistItems[m.checklistCursor]
		m.checklistInput.EnterEditMode(item.ID, item.Body)
		return m, nil

	case "E":
		return m.checklistOpenEditor()
	}
	return m, nil
}

// handleChecklistInputKey handles key events when checklistMode is true and the input bar is active.
func (m Model) handleChecklistInputKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		val := strings.TrimSpace(m.checklistInput.Value())
		if val == "" {
			m.checklistInput.Exit()
			return m, nil
		}
		if m.checklistInput.IsAdd() {
			if err := m.db.TodoAdd(m.checklistSession, val); err != nil {
				demuxlog.Error("todo add failed", "err", err)
				m.statusMsg = "error adding item: " + err.Error()
				m.statusExp = time.Now().Add(4 * time.Second)
			} else {
				demuxlog.Info("todo added", "session", m.checklistSession, "body", val)
			}
		} else {
			if err := m.db.TodoUpdate(m.checklistInput.EditID(), val); err != nil {
				demuxlog.Error("todo update failed", "err", err)
				m.statusMsg = "error updating item: " + err.Error()
				m.statusExp = time.Now().Add(4 * time.Second)
			} else {
				demuxlog.Info("todo updated", "id", m.checklistInput.EditID(), "body", val)
			}
		}
		m.checklistInput.Exit()
		return m, tea.Batch(m.fetchTodos(m.checklistSession), m.fetchTodoSessions())

	case "esc":
		m.checklistInput.Exit()
		return m, nil

	default:
		var cmd tea.Cmd
		m.checklistInput, cmd = m.checklistInput.Update(msg)
		return m, cmd
	}
}

// checklistToggle flips the checked state of the focused item.
func (m Model) checklistToggle() (Model, tea.Cmd) {
	if len(m.checklistItems) == 0 {
		return m, nil
	}
	item := m.checklistItems[m.checklistCursor]
	if err := m.db.TodoToggle(item.ID); err != nil {
		demuxlog.Error("todo toggle failed", "id", item.ID, "err", err)
		m.statusMsg = "error toggling item: " + err.Error()
		m.statusExp = time.Now().Add(4 * time.Second)
		return m, nil
	}
	demuxlog.Info("todo toggled", "id", item.ID, "was_checked", item.Checked)
	return m, tea.Batch(m.fetchTodos(m.checklistSession), m.fetchTodoSessions())
}

// checklistDeletePrompt shows a confirm overlay before deleting the focused item.
func (m Model) checklistDeletePrompt() (Model, tea.Cmd) {
	if len(m.checklistItems) == 0 {
		return m, nil
	}
	item := m.checklistItems[m.checklistCursor]
	id := item.ID
	session := m.checklistSession
	body := item.Body

	m.showConfirm = true
	m.confirm = ConfirmModel{
		prompt: "Delete todo item?",
		body:   truncateTarget(body, 50),
	}
	m.confirmCmd = func() tea.Msg {
		// This is invoked when the user confirms.
		return checklistDeleteConfirmedMsg{id: id, session: session}
	}
	return m, nil
}
