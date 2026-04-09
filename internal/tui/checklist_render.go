package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	xansi "github.com/charmbracelet/x/ansi"
	runewidth "github.com/mattn/go-runewidth"
	"github.com/rtalexk/demux/internal/db"
)

const (
	checkboxUnchecked = "☐"
	checkboxChecked   = "☑"
)

// renderChecklistPanel renders the full bordered checklist panel (same
// external dimensions as a proclist panel), including title bar, items,
// separator, and input bar. baseTitle is the normal proclist title; " · TODOs"
// is appended to it so the breadcrumb context is preserved.
func (m Model) renderChecklistPanel(width, height int, focused bool, baseTitle string) string {
	innerW := width - borderOverhead
	innerH := height - borderOverhead

	// Reserve 2 inner rows for separator + input bar.
	listH := innerH - 2
	if listH < 1 {
		listH = 1
	}

	// Build item rows.
	var rows []string
	for i, item := range m.checklistItems {
		cb := checkboxUnchecked
		if item.Checked {
			cb = checkboxChecked
		}
		cbW := runewidth.StringWidth(cb)
		maxBody := innerW - cbW - 3 // 2 leading spaces + 1 gap
		body := truncateToWidth(item.Body, maxBody)

		line := fmt.Sprintf("  %s  %s", cb, body)
		lineW := runewidth.StringWidth(xansi.Strip(line))
		if lineW < innerW {
			line += strings.Repeat(" ", innerW-lineW)
		}

		if i == m.checklistCursor {
			line = selectedBG.Bold(true).Width(innerW).Render(line)
		}
		rows = append(rows, line)
	}

	// Viewport: scroll so cursor is always visible.
	start := 0
	if m.checklistCursor >= listH {
		start = m.checklistCursor - listH + 1
	}
	end := start + listH
	if end > len(rows) {
		end = len(rows)
	}
	visible := rows[start:end]
	// Pad with blank lines to fill listH.
	for len(visible) < listH {
		visible = append(visible, strings.Repeat(" ", innerW))
	}

	// Separator line.
	borderColor := activeTheme.ColorBorder
	if focused {
		borderColor = activeTheme.ColorSession
	}
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)
	sep := borderStyle.Render(strings.Repeat("─", innerW))

	// Input bar.
	var inputLine string
	if m.checklistInput.IsActive() {
		prompt := hintStyle.Render("[a]") + " "
		if m.checklistInput.IsEdit() {
			prompt = hintStyle.Render("[e]") + " "
		}
		inputView := m.checklistInput.View()
		inputLine = prompt + inputView
	} else {
		inputLine = hintStyle.Render("[a]") + " Add item"
	}
	inputW := runewidth.StringWidth(xansi.Strip(inputLine))
	if inputW < innerW {
		inputLine += strings.Repeat(" ", innerW-inputW)
	}

	content := strings.Join(visible, "\n") + "\n" + sep + "\n" + inputLine

	// Append " · TODOs" to the existing proc title so the breadcrumb is preserved.
	title := baseTitle + "· TODOs "

	// Wrap in a border using the same style proclist uses.
	titleWidth := runewidth.StringWidth(xansi.Strip(title))
	dashCount := innerW - titleWidth - 1
	if dashCount < 0 {
		dashCount = 0
	}
	top := borderStyle.Render("╭─") + title + borderStyle.Render(strings.Repeat("─", dashCount)+"╮")
	bot := borderStyle.Render("╰" + strings.Repeat("─", innerW) + "╯")

	var sb strings.Builder
	sb.WriteString(top + "\n")
	for _, row := range strings.Split(content, "\n") {
		sb.WriteString(borderStyle.Render("│") + row + borderStyle.Render("│") + "\n")
	}
	sb.WriteString(bot)

	return sb.String()
}

// truncateToWidth truncates s to fit within maxW display columns.
func truncateToWidth(s string, maxW int) string {
	if runewidth.StringWidth(s) <= maxW {
		return s
	}
	runes := []rune(s)
	for runewidth.StringWidth(string(runes)) > maxW-1 {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + ellipsis
}

// checklistIndicatorWidth returns the display width of the todo indicator slot.
// Used by sidebar render to reserve consistent space.
func checklistIndicatorWidth() int {
	if activeTheme.IconTodo == "" {
		return 0
	}
	return runewidth.StringWidth(activeTheme.IconTodo)
}

// todoItemsForSession is a convenience wrapper used in tests.
func todoItemsForSession(items []db.Todo) []string {
	out := make([]string, len(items))
	for i, it := range items {
		cb := checkboxUnchecked
		if it.Checked {
			cb = checkboxChecked
		}
		out[i] = fmt.Sprintf("%s %s", cb, it.Body)
	}
	return out
}
