package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type itemInputMode int

const (
	itemInputIdle itemInputMode = iota
	itemInputAdd
	itemInputEdit
)

// ItemInputModel is the bottom input bar used to add or edit session items.
type ItemInputModel struct {
	input  textinput.Model
	mode   itemInputMode
	kind   string // "todo" or "note", set when mode == itemInputAdd
	editID int64  // set when mode == itemInputEdit
}

func newItemInputModel() ItemInputModel {
	ti := textinput.New()
	ti.Placeholder = "New item..."
	return ItemInputModel{input: ti}
}

// EnterAddMode prepares the input for adding a new item of the given kind.
func (c *ItemInputModel) EnterAddMode(kind string) {
	c.mode = itemInputAdd
	c.kind = kind
	c.editID = 0
	c.input.SetValue("")
	c.input.Focus()
}

// Kind returns the kind of item being added ("todo" or "note").
func (c ItemInputModel) Kind() string { return c.kind }

// EnterEditMode prepares the input for editing an existing item.
func (c *ItemInputModel) EnterEditMode(id int64, kind, body string) {
	c.mode = itemInputEdit
	c.kind = kind
	c.editID = id
	c.input.SetValue(body)
	c.input.Focus()
}

// Exit clears the input and returns to idle mode.
func (c *ItemInputModel) Exit() {
	c.mode = itemInputIdle
	c.editID = 0
	c.input.SetValue("")
	c.input.Blur()
}

// IsActive returns true when the input is in add or edit mode.
func (c ItemInputModel) IsActive() bool { return c.mode != itemInputIdle }

// IsAdd returns true when adding a new item.
func (c ItemInputModel) IsAdd() bool { return c.mode == itemInputAdd }

// IsEdit returns true when editing an existing item.
func (c ItemInputModel) IsEdit() bool { return c.mode == itemInputEdit }

// EditID returns the ID of the item being edited.
func (c ItemInputModel) EditID() int64 { return c.editID }

// Value returns the current input text.
func (c ItemInputModel) Value() string { return c.input.Value() }

// Update processes a key message against the text input.
func (c ItemInputModel) Update(msg tea.Msg) (ItemInputModel, tea.Cmd) {
	var cmd tea.Cmd
	c.input, cmd = c.input.Update(msg)
	return c, cmd
}

// View renders the text input widget.
func (c ItemInputModel) View() string {
	return c.input.View()
}
