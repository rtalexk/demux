package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// cmdlineLongThreshold is the rune length above which "Show full command" appears in the action menu.
const cmdlineLongThreshold = 60

// menuItemIndent is the leading whitespace applied to every menu row so that
// selected rows (rendered with selectedBG) stay horizontally aligned.
const menuItemIndent = "  "

// menuItemTrailingPad is the extra width added to the selected-row highlight
// so the background extends slightly past the longest item text.
const menuItemTrailingPad = 2

// menuItemNoShortcutPad is used as a fallback prefix when an action item has no shortcut,
// to visually match the width of "[x] ".
const menuItemNoShortcutPad = "    " // matches len("[x] ") width

// actionKind identifies an action in the action menu.
type actionKind int

const (
	ActionKill actionKind = iota
	ActionRestart
	ActionViewLogs
	ActionOpenBrowser
	ActionShowEnv
	ActionShowFullCmd
)

// subPopupKind identifies which sub-popup is currently open inside the action menu.
type subPopupKind int

const (
	SubPopupNone subPopupKind = iota
	SubPopupKillConfirm
)

// actionItem is a single entry in the action menu.
type actionItem struct {
	Kind     actionKind
	Label    string
	Shortcut string // single-key shortcut shown in the menu (empty = none)
}

// actionMenuModel holds the state for the action menu and its sub-popups.
type actionMenuModel struct {
	items    []actionItem
	cursor   int
	target   ProcListNode
	subPopup subPopupKind
}

// buildActionItems constructs the dynamic action list for a given process node.
// Only actions applicable to the node's context are included.
func buildActionItems(node ProcListNode) []actionItem {
	items := []actionItem{
		{ActionKill, fmt.Sprintf("Kill process (%s)", node.Proc.FriendlyName()), "x"},
		{ActionRestart, "Re-run last cmd (Up+Enter)", "r"},
		{ActionViewLogs, "View logs", "l"},
	}
	if node.Port > 0 {
		items = append(items, actionItem{ActionOpenBrowser, fmt.Sprintf("Open in browser (:%d)", node.Port), "o"})
	}
	items = append(items, actionItem{ActionShowEnv, "Show environment", "d"})
	if len([]rune(node.Proc.Cmdline)) > cmdlineLongThreshold {
		items = append(items, actionItem{ActionShowFullCmd, "Show full command", "f"})
	}
	return items
}

// MoveDown moves the cursor down by one, clamped at the last item.
func (a *actionMenuModel) MoveDown() {
	if a.cursor < len(a.items)-1 {
		a.cursor++
	}
}

// MoveUp moves the cursor up by one, clamped at zero.
func (a *actionMenuModel) MoveUp() {
	if a.cursor > 0 {
		a.cursor--
	}
}

// Selected returns the currently highlighted actionItem, or nil when the list is empty.
func (a *actionMenuModel) Selected() *actionItem {
	if a.cursor < 0 || a.cursor >= len(a.items) {
		return nil
	}
	it := a.items[a.cursor]
	return &it
}

// Render returns the styled action menu popup string.
func (a actionMenuModel) Render() string {
	title := fmt.Sprintf("Actions: %s (%d)", a.target.Proc.FriendlyName(), a.target.Proc.PID)

	type row struct{ prefix, label string }
	rows := make([]row, len(a.items))
	maxW := 0
	for i, it := range a.items {
		prefix := menuItemNoShortcutPad
		if it.Shortcut != "" {
			prefix = "[" + it.Shortcut + "] "
		}
		rows[i] = row{prefix, it.Label}
		if w := lipgloss.Width(menuItemIndent + prefix + it.Label); w > maxW {
			maxW = w
		}
	}

	var sb strings.Builder
	sb.WriteString(title)
	sb.WriteString("\n\n")
	for i, r := range rows {
		line := r.prefix + r.label
		if i == a.cursor {
			line = selectedBG.Width(maxW + menuItemTrailingPad).Render(menuItemIndent + line)
		} else {
			line = menuItemIndent + line
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
	sb.WriteString(hintStyle.Render("Enter / shortcut") + " select   " + hintStyle.Render("Esc / q") + " close")
	return confirmStyle.Render(sb.String())
}

// RenderKillConfirm returns the styled kill-confirmation sub-popup.
func (a actionMenuModel) RenderKillConfirm() string {
	prompt := fmt.Sprintf("Kill %s (PID %d)?", a.target.Proc.FriendlyName(), a.target.Proc.PID)
	return ConfirmModel{prompt: prompt}.Render()
}
