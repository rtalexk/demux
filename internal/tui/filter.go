package tui

import (
    "github.com/charmbracelet/bubbles/textinput"
    tea "github.com/charmbracelet/bubbletea"
)


type FilterModel struct {
    input textinput.Model
}

func NewFilterModel() FilterModel {
    ti := textinput.New()
    ti.Placeholder = "filter by name or port…"
    ti.Focus()
    return FilterModel{input: ti}
}

func (f FilterModel) Value() string {
    return f.input.Value()
}

func (f FilterModel) Render() string {
    return filterStyle.Render(f.input.View())
}

func (f FilterModel) Update(msg tea.Msg) (FilterModel, tea.Cmd) {
    var cmd tea.Cmd
    f.input, cmd = f.input.Update(msg)
    return f, cmd
}
