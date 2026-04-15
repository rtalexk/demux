package tui

import (
	"fmt"
	"strings"
)

// ActionKind identifies an action in the action menu.
type ActionKind int

const (
	ActionKill ActionKind = iota
	ActionRestart
	ActionViewLogs
	ActionCopyPID
	ActionCopyCommand
	ActionCopyCWD
	ActionOpenBrowser
	ActionShowEnv
	ActionShowFullCmd
)

// SubPopupKind identifies which sub-popup is currently open inside the action menu.
type SubPopupKind int

const (
	SubPopupNone SubPopupKind = iota
	SubPopupKillConfirm
	SubPopupFullCmd
)

// ActionItem is a single entry in the action menu.
type ActionItem struct {
	Kind     ActionKind
	Label    string
	Shortcut string // single-key shortcut shown in the menu (empty = none)
}

// ActionMenuModel holds the state for the action menu and its sub-popups.
type ActionMenuModel struct {
	items        []ActionItem
	cursor       int
	target       ProcListNode
	subPopup     SubPopupKind
	subLines     []string // content lines for scrollable sub-popups
	subScrollOff int
}

// buildActionItems constructs the dynamic action list for a given process node.
// Only actions applicable to the node's context are included.
func buildActionItems(node ProcListNode) []ActionItem {
	items := []ActionItem{
		{ActionKill, fmt.Sprintf("Kill process (%s)", node.Proc.FriendlyName()), "x"},
		{ActionRestart, "Restart process", "r"},
		{ActionViewLogs, "View logs", "l"},
		{ActionCopyPID, fmt.Sprintf("Copy PID (%d)", node.Proc.PID), "p"},
	}
	if node.Proc.Cmdline != "" {
		items = append(items, ActionItem{ActionCopyCommand, "Copy command", "c"})
	}
	if node.Pane.CWD != "" {
		items = append(items, ActionItem{ActionCopyCWD, "Copy working directory", "w"})
	}
	if node.Port > 0 {
		items = append(items, ActionItem{ActionOpenBrowser, fmt.Sprintf("Open in browser (:%d)", node.Port), ""})
	}
	items = append(items, ActionItem{ActionShowEnv, "Show environment", "d"})
	if len([]rune(node.Proc.Cmdline)) > 60 {
		items = append(items, ActionItem{ActionShowFullCmd, "Show full command", ""})
	}
	return items
}

// MoveDown moves the cursor down by one, clamped at the last item.
func (a *ActionMenuModel) MoveDown() {
	if a.cursor < len(a.items)-1 {
		a.cursor++
	}
}

// MoveUp moves the cursor up by one, clamped at zero.
func (a *ActionMenuModel) MoveUp() {
	if a.cursor > 0 {
		a.cursor--
	}
}

// Selected returns the currently highlighted ActionItem, or nil when the list is empty.
func (a *ActionMenuModel) Selected() *ActionItem {
	if a.cursor < 0 || a.cursor >= len(a.items) {
		return nil
	}
	it := a.items[a.cursor]
	return &it
}

// SubScrollDown scrolls the sub-popup content down by one line.
func (a *ActionMenuModel) SubScrollDown() {
	if a.subScrollOff < len(a.subLines)-1 {
		a.subScrollOff++
	}
}

// SubScrollUp scrolls the sub-popup content up by one line.
func (a *ActionMenuModel) SubScrollUp() {
	if a.subScrollOff > 0 {
		a.subScrollOff--
	}
}

// Render returns the styled action menu popup string.
func (a ActionMenuModel) Render() string {
	title := fmt.Sprintf("Actions: %s (%d)", a.target.Proc.FriendlyName(), a.target.Proc.PID)
	var sb strings.Builder
	sb.WriteString(title)
	sb.WriteString("\n\n")
	for i, it := range a.items {
		prefix := "    " // 4 chars to match "[x] " width
		if it.Shortcut != "" {
			prefix = "[" + it.Shortcut + "] "
		}
		line := prefix + it.Label
		if i == a.cursor {
			line = selectedBG.Render("  " + line)
		} else {
			line = "  " + line
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
	sb.WriteString(hintStyle.Render("Enter / shortcut") + " select   " + hintStyle.Render("Esc / q") + " close")
	return confirmStyle.Render(sb.String())
}

// RenderKillConfirm returns the styled kill-confirmation sub-popup.
func (a ActionMenuModel) RenderKillConfirm() string {
	prompt := fmt.Sprintf("Kill %s (PID %d)?", a.target.Proc.FriendlyName(), a.target.Proc.PID)
	return ConfirmModel{prompt: prompt}.Render()
}

// RenderSubPopup returns the styled sub-popup overlay for env vars or full command.
// height is the number of visible content lines to show (0 = show all).
func (a ActionMenuModel) RenderSubPopup(height int) string {
	var title string
	switch a.subPopup {
	case SubPopupFullCmd:
		title = fmt.Sprintf("Command: %s (%d)", a.target.Proc.FriendlyName(), a.target.Proc.PID)
	default:
		return ""
	}

	visible := a.subLines
	if height > 0 && len(visible) > 0 {
		start := a.subScrollOff
		if start >= len(visible) {
			start = 0
		}
		end := start + height
		if end > len(visible) {
			end = len(visible)
		}
		visible = visible[start:end]
	}

	var sb strings.Builder
	sb.WriteString(title)
	sb.WriteString("\n\n")
	for _, l := range visible {
		sb.WriteString("  ")
		sb.WriteString(l)
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
	sb.WriteString(hintStyle.Render("j / k") + " scroll   " + hintStyle.Render("Esc / q") + " back")
	return confirmStyle.Render(sb.String())
}
