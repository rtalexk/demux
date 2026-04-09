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
	noteMarker        = "─"
	emptyHint         = "(empty)"
)

// renderItemsPanel renders the full bordered checklist panel (same
// external dimensions as a proclist panel), including title bar, items,
// separators, and input bar. baseTitle is the normal proclist title; "· Items"
// is appended to it so the breadcrumb context is preserved.
func (m Model) renderItemsPanel(width, height int, focused bool, baseTitle string) string {
	innerW := width - borderOverhead
	innerH := height - borderOverhead

	// Split items by kind, preserving original flat indices for cursor matching.
	var todoIdxs, noteIdxs []int
	for i, it := range m.itemList {
		if it.Kind == "todo" {
			todoIdxs = append(todoIdxs, i)
		} else {
			noteIdxs = append(noteIdxs, i)
		}
	}

	borderColor := activeTheme.ColorBorder
	if focused {
		borderColor = activeTheme.ColorSession
	}
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)
	dimStyle := lipgloss.NewStyle().Faint(true)
	sep := borderStyle.Render(strings.Repeat("─", innerW))

	renderItem := func(flatIdx int, marker string) string {
		item := m.itemList[flatIdx]
		if item.Kind == "todo" {
			if item.Checked {
				marker = checkboxChecked
			} else {
				marker = checkboxUnchecked
			}
		}
		markerW := runewidth.StringWidth(marker)
		maxBody := innerW - markerW - 3
		body := truncateToWidth(item.Body, maxBody)
		line := fmt.Sprintf("  %s  %s", marker, body)
		lineW := runewidth.StringWidth(xansi.Strip(line))
		if lineW < innerW {
			line += strings.Repeat(" ", innerW-lineW)
		}
		if flatIdx == m.itemCursor {
			line = selectedBG.Bold(true).Width(innerW).Render(line)
		}
		return line
	}

	sectionHeader := func(label string) string {
		line := " " + label
		lineW := runewidth.StringWidth(line)
		if lineW < innerW {
			line += strings.Repeat(" ", innerW-lineW)
		}
		return lipgloss.NewStyle().Bold(true).Render(line)
	}

	emptyLine := func() string {
		line := "  " + dimStyle.Render(emptyHint)
		lineW := runewidth.StringWidth(xansi.Strip(line))
		if lineW < innerW {
			line += strings.Repeat(" ", innerW-lineW)
		}
		return line
	}

	// Build all visible rows in order.
	var rows []string
	rows = append(rows, sectionHeader("TODOs"))
	if len(todoIdxs) == 0 {
		rows = append(rows, emptyLine())
	} else {
		for _, idx := range todoIdxs {
			rows = append(rows, renderItem(idx, checkboxUnchecked))
		}
	}
	rows = append(rows, sep)
	rows = append(rows, sectionHeader("Notes"))
	if len(noteIdxs) == 0 {
		rows = append(rows, emptyLine())
	} else {
		for _, idx := range noteIdxs {
			rows = append(rows, renderItem(idx, noteMarker))
		}
	}

	// Reserve 2 inner rows for final separator + input bar.
	listH := innerH - 2
	if listH < 1 {
		listH = 1
	}

	// Viewport: find which visual row contains the cursor and scroll accordingly.
	cursorRow := 0
	if m.itemCursor < len(m.itemList) {
		// Count visual rows up to the cursor item.
		// TODOs section: header(1) + items(len(todoIdxs) or 1 for empty) + sep(1) + Notes header(1)
		// then note items.
		counted := 1 // TODOs header
		for _, idx := range todoIdxs {
			if idx == m.itemCursor {
				cursorRow = counted
				break
			}
			counted++
		}
		if len(todoIdxs) == 0 {
			counted++ // empty placeholder
		}
		counted++ // sep
		counted++ // Notes header
		for _, idx := range noteIdxs {
			if idx == m.itemCursor {
				cursorRow = counted
				break
			}
			counted++
		}
	}

	start := 0
	if cursorRow >= listH {
		start = cursorRow - listH + 1
	}
	end := start + listH
	if end > len(rows) {
		end = len(rows)
	}
	visible := rows[start:end]
	for len(visible) < listH {
		visible = append(visible, strings.Repeat(" ", innerW))
	}

	// Input bar.
	var inputLine string
	switch {
	case m.itemInput.IsEdit():
		inputLine = hintStyle.Render("[e]") + " " + m.itemInput.View()
	case m.itemInput.IsAdd() && m.itemInput.Kind() == "note":
		inputLine = hintStyle.Render("[n]") + " " + m.itemInput.View()
	case m.itemInput.IsAdd():
		inputLine = hintStyle.Render("[a]") + " " + m.itemInput.View()
	default:
		inputLine = hintStyle.Render("[a]") + " to add TODO  " + hintStyle.Render("[n]") + " to add Note"
	}
	inputW := runewidth.StringWidth(xansi.Strip(inputLine))
	if inputW < innerW {
		inputLine += strings.Repeat(" ", innerW-inputW)
	}

	content := strings.Join(visible, "\n") + "\n" + sep + "\n" + inputLine

	title := baseTitle + "· Items "
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

// itemBodiesForSession is a convenience wrapper used in tests.
func itemBodiesForSession(items []db.Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		switch it.Kind {
		case "todo":
			cb := checkboxUnchecked
			if it.Checked {
				cb = checkboxChecked
			}
			out[i] = fmt.Sprintf("%s %s", cb, it.Body)
		default:
			out[i] = fmt.Sprintf("%s %s", noteMarker, it.Body)
		}
	}
	return out
}
