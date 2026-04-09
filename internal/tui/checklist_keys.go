package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	demuxlog "github.com/rtalexk/demux/internal/log"
)

// openItemsMode switches the proclist panel to checklist mode for the given session.
func (m Model) openItemsMode(session string) (Model, tea.Cmd) {
	m.itemsMode = true
	m.itemSession = session
	m.itemCursor = 0
	m.itemInput.Exit()
	demuxlog.Info("items mode on", "session", session)
	return m, m.fetchItems(session)
}

// handleItemsKey handles key events when itemsMode is true, the proclist
// panel is focused, and the input bar is idle. Global keys (including C) are
// handled before this in handleKey and are not repeated here.
func (m Model) handleItemsKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.itemsMode = false
		m.itemInput.Exit()
		demuxlog.Debug("items mode off via esc", "session", m.itemSession)
		return m, nil

	case "j", "down":
		if m.itemCursor < len(m.itemList)-1 {
			m.itemCursor++
		}
		return m, nil

	case "k", "up":
		if m.itemCursor > 0 {
			m.itemCursor--
		}
		return m, nil

	case "g":
		m.itemCursor = 0
		return m, nil

	case "G":
		if len(m.itemList) > 0 {
			m.itemCursor = len(m.itemList) - 1
		}
		return m, nil

	case "n":
		m.itemInput.EnterAddMode("note")
		return m, nil

	case "x":
		if len(m.itemList) == 0 {
			return m, nil
		}
		if m.itemList[m.itemCursor].Kind == "note" {
			return m, nil // toggle is a no-op for notes
		}
		return m.itemToggle()

	case "d":
		return m.itemDeletePrompt()

	case "a":
		m.itemInput.EnterAddMode("todo")
		return m, nil

	case "e":
		if len(m.itemList) == 0 {
			return m, nil
		}
		item := m.itemList[m.itemCursor]
		m.itemInput.EnterEditMode(item.ID, item.Kind, item.Body)
		return m, nil

	case "E":
		return m.openItemEditor()
	}
	return m, nil
}

// handleItemInputKey handles key events when itemsMode is true and the input bar is active.
func (m Model) handleItemInputKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		val := strings.TrimSpace(m.itemInput.Value())
		if val == "" {
			m.itemInput.Exit()
			return m, nil
		}
		if m.itemInput.IsAdd() {
			if err := m.db.ItemAdd(m.itemSession, m.itemInput.Kind(), val); err != nil {
				demuxlog.Error("item add failed", "err", err)
				m.statusMsg = "error adding item: " + err.Error()
				m.statusExp = time.Now().Add(4 * time.Second)
			} else {
				demuxlog.Info("item added", "session", m.itemSession, "kind", m.itemInput.Kind(), "body", val)
			}
		} else {
			if err := m.db.ItemUpdate(m.itemInput.EditID(), val); err != nil {
				demuxlog.Error("item update failed", "err", err)
				m.statusMsg = "error updating item: " + err.Error()
				m.statusExp = time.Now().Add(4 * time.Second)
			} else {
				demuxlog.Info("item updated", "id", m.itemInput.EditID(), "body", val)
			}
		}
		m.itemInput.Exit()
		return m, tea.Batch(m.fetchItems(m.itemSession), m.fetchItemSessions())

	case "esc":
		m.itemInput.Exit()
		return m, nil

	default:
		var cmd tea.Cmd
		m.itemInput, cmd = m.itemInput.Update(msg)
		return m, cmd
	}
}

// itemToggle flips the checked state of the focused item.
func (m Model) itemToggle() (Model, tea.Cmd) {
	if len(m.itemList) == 0 {
		return m, nil
	}
	item := m.itemList[m.itemCursor]
	if err := m.db.ItemToggle(item.ID); err != nil {
		demuxlog.Error("item toggle failed", "id", item.ID, "err", err)
		m.statusMsg = "error toggling item: " + err.Error()
		m.statusExp = time.Now().Add(4 * time.Second)
		return m, nil
	}
	demuxlog.Info("item toggled", "id", item.ID, "was_checked", item.Checked)
	return m, tea.Batch(m.fetchItems(m.itemSession), m.fetchItemSessions())
}

// itemDeletePrompt shows a confirm overlay before deleting the focused item.
func (m Model) itemDeletePrompt() (Model, tea.Cmd) {
	if len(m.itemList) == 0 {
		return m, nil
	}
	item := m.itemList[m.itemCursor]
	id := item.ID
	session := m.itemSession
	body := item.Body

	m.showConfirm = true
	m.confirm = ConfirmModel{
		prompt: "Delete item?",
		body:   truncateTarget(body, 50),
	}
	m.confirmCmd = func() tea.Msg {
		// This is invoked when the user confirms.
		return itemDeleteConfirmedMsg{id: id, session: session}
	}
	return m, nil
}
