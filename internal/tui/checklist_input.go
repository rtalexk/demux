package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type checklistInputMode int

const (
	checklistInputIdle checklistInputMode = iota
	checklistInputAdd
	checklistInputEdit
)

// ChecklistInputModel is the bottom input bar used to add or edit checklist items.
type ChecklistInputModel struct {
	input  textinput.Model
	mode   checklistInputMode
	editID int64 // set when mode == checklistInputEdit
}

func newChecklistInputModel() ChecklistInputModel {
	ti := textinput.New()
	ti.Placeholder = "New item..."
	return ChecklistInputModel{input: ti}
}

// EnterAddMode prepares the input for adding a new item.
func (c *ChecklistInputModel) EnterAddMode() {
	c.mode = checklistInputAdd
	c.editID = 0
	c.input.SetValue("")
	c.input.Focus()
}

// EnterEditMode prepares the input for editing an existing item.
func (c *ChecklistInputModel) EnterEditMode(id int64, body string) {
	c.mode = checklistInputEdit
	c.editID = id
	c.input.SetValue(body)
	c.input.Focus()
}

// Exit clears the input and returns to idle mode.
func (c *ChecklistInputModel) Exit() {
	c.mode = checklistInputIdle
	c.editID = 0
	c.input.SetValue("")
	c.input.Blur()
}

// IsActive returns true when the input is in add or edit mode.
func (c ChecklistInputModel) IsActive() bool { return c.mode != checklistInputIdle }

// IsAdd returns true when adding a new item.
func (c ChecklistInputModel) IsAdd() bool { return c.mode == checklistInputAdd }

// IsEdit returns true when editing an existing item.
func (c ChecklistInputModel) IsEdit() bool { return c.mode == checklistInputEdit }

// EditID returns the ID of the item being edited.
func (c ChecklistInputModel) EditID() int64 { return c.editID }

// Value returns the current input text.
func (c ChecklistInputModel) Value() string { return c.input.Value() }

// Update processes a key message against the text input.
func (c ChecklistInputModel) Update(msg tea.Msg) (ChecklistInputModel, tea.Cmd) {
	var cmd tea.Cmd
	c.input, cmd = c.input.Update(msg)
	return c, cmd
}

// View renders the text input widget.
func (c ChecklistInputModel) View() string {
	return c.input.View()
}
