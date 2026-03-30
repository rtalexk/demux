package tui

import (
    "strings"

    "github.com/charmbracelet/bubbles/textinput"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "github.com/mattn/go-runewidth"
)

// SearchInputModel wraps textinput with vim-style insert/normal mode.
type SearchInputModel struct {
    input  textinput.Model
    insert bool // true = insert (editing) mode
}

func NewSearchInputModel() SearchInputModel {
    ti := textinput.New()
    ti.Placeholder = "search…"
    return SearchInputModel{input: ti}
}

// EnterInsertMode focuses the input for typing.
func (s *SearchInputModel) EnterInsertMode() {
    s.insert = true
    s.input.Focus()
}

// ExitInsertMode blurs the input; query text is preserved.
func (s *SearchInputModel) ExitInsertMode() {
    s.insert = false
    s.input.Blur()
}

func (s SearchInputModel) IsInsert() bool { return s.insert }

// Value returns the current query string.
func (s SearchInputModel) Value() string { return s.input.Value() }

// IsActive returns true when a non-empty query is set.
func (s SearchInputModel) IsActive() bool { return s.input.Value() != "" }

func (s SearchInputModel) Update(msg tea.Msg) (SearchInputModel, tea.Cmd) {
    var cmd tea.Cmd
    s.input, cmd = s.input.Update(msg)
    return s, cmd
}

// View renders the 3-line bordered search box at the given width.
func (s SearchInputModel) View(width int) string {
    const title = "[/] Search"

    inputView := s.input.View()
    innerWidth := width - 2 // subtract left + right border chars

    // Style the inner content line.
    innerStyle := lipgloss.NewStyle().Width(innerWidth)
    if s.IsActive() && !s.insert {
        innerStyle = innerStyle.Foreground(activeTheme.ColorSession) // accent when active
    }
    mid := "│" + innerStyle.Render(inputView) + "│"

    // Build top border with title.
    titleWidth := runewidth.StringWidth(title)
    dashCount := innerWidth - titleWidth - 1
    if dashCount < 0 {
        dashCount = 0
    }
    top := "┌─" + title + strings.Repeat("─", dashCount) + "┐"
    bot := "└" + strings.Repeat("─", innerWidth) + "┘"

    return top + "\n" + mid + "\n" + bot
}
