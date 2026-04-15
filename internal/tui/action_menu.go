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
)

// ActionItem is a single entry in the action menu.
type ActionItem struct {
	Kind     ActionKind
	Label    string
	Shortcut string // single-key shortcut shown in the menu (empty = none)
}

// ActionMenuModel holds the state for the action menu and its sub-popups.
type ActionMenuModel struct {
	items    []ActionItem
	cursor   int
	target   ProcListNode
	subPopup SubPopupKind
}

// buildActionItems constructs the dynamic action list for a given process node.
// Only actions applicable to the node's context are included.
func buildActionItems(node ProcListNode) []ActionItem {
	items := []ActionItem{
		{ActionKill, fmt.Sprintf("Kill process (%s)", node.Proc.FriendlyName()), "x"},
		{ActionRestart, "Re-run last cmd (Up+Enter)", "r"},
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
		items = append(items, ActionItem{ActionOpenBrowser, fmt.Sprintf("Open in browser (:%d)", node.Port), "o"})
	}
	items = append(items, ActionItem{ActionShowEnv, "Show environment", "d"})
	if len([]rune(node.Proc.Cmdline)) > 60 {
		items = append(items, ActionItem{ActionShowFullCmd, "Show full command", "f"})
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
