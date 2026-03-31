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
    ti.Placeholder = "Press f to search..."
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

// Clear resets the query text without changing insert/normal mode.
func (s *SearchInputModel) Clear() { s.input.SetValue("") }

func (s SearchInputModel) Update(msg tea.Msg) (SearchInputModel, tea.Cmd) {
    var cmd tea.Cmd
    s.input, cmd = s.input.Update(msg)
    return s, cmd
}

// View renders the 3-line bordered search box at the given width.
func (s SearchInputModel) View(width int) string {
    const title = "[f] Search"

    if s.insert {
        s.input.Placeholder = "Type to search..."
    } else {
        s.input.Placeholder = "Press f to search..."
    }
    inputView := s.input.View()
    innerWidth := width - 2 // subtract left + right border chars

    // Determine border color.
    borderColor := activeTheme.ColorBorder
    if s.IsActive() {
        borderColor = activeTheme.ColorSession
    }
    borderStyle := lipgloss.NewStyle().Foreground(borderColor)

    // Style the inner content line.
    innerStyle := lipgloss.NewStyle().Width(innerWidth)
    mid := borderStyle.Render("│") + innerStyle.Render(inputView) + borderStyle.Render("│")

    // Build top border with title.
    titleWidth := runewidth.StringWidth(title)
    dashCount := innerWidth - titleWidth - 1
    if dashCount < 0 {
        dashCount = 0
    }
    top := borderStyle.Render("╭─") + title + borderStyle.Render(strings.Repeat("─", dashCount)+"╮")
    bot := borderStyle.Render("╰" + strings.Repeat("─", innerWidth) + "╯")

    return top + "\n" + mid + "\n" + bot
}
