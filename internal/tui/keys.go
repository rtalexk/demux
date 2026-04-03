package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	FocusSidebar   key.Binding
	FocusProcList  key.Binding
	Yank           key.Binding
	Refresh        key.Binding
	Help           key.Binding
	Quit           key.Binding
	Up             key.Binding
	Down           key.Binding
	Enter          key.Binding
	Esc            key.Binding
	Kill           key.Binding
	Restart        key.Binding
	Log            key.Binding
	JumpUp         key.Binding
	JumpDown       key.Binding
	Tab            key.Binding
	ShiftTab       key.Binding
	GotoTop        key.Binding
	GotoBottom     key.Binding
	FilterTmux     key.Binding
	FilterAll      key.Binding
	FilterConfig   key.Binding
	FilterWorktree key.Binding
	Open           key.Binding
	AlertFilter    key.Binding
	Expand         key.Binding
	Collapse       key.Binding
	ExpandAll      key.Binding
	CollapseAll    key.Binding
	Defer          key.Binding
}

var keys = keyMap{
	FocusSidebar:   key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "sidebar")),
	FocusProcList:  key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "procs")),
	Yank:           key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "yank")),
	Refresh:        key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "refresh")),
	Help:           key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Quit:           key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Up:             key.NewBinding(key.WithKeys("k", "up", "ctrl+k", "ctrl+p")),
	Down:           key.NewBinding(key.WithKeys("j", "down", "ctrl+j", "ctrl+n")),
	Enter:          key.NewBinding(key.WithKeys("enter")),
	Esc:            key.NewBinding(key.WithKeys("esc")),
	Kill:           key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "kill")),
	Restart:        key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "restart")),
	Log:            key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "log")),
	JumpUp:         key.NewBinding(key.WithKeys("K")),
	JumpDown:       key.NewBinding(key.WithKeys("J")),
	Tab:            key.NewBinding(key.WithKeys("tab")),
	ShiftTab:       key.NewBinding(key.WithKeys("shift+tab")),
	GotoTop:        key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top")),
	GotoBottom:     key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
	FilterTmux:     key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "tmux")),
	FilterAll:      key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "all")),
	FilterConfig:   key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "config")),
	FilterWorktree: key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "worktree")),
	Open:           key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open/attach")),
	AlertFilter:    key.NewBinding(key.WithKeys("!"), key.WithHelp("!", "alert filter")),
	Expand:         key.NewBinding(key.WithKeys("]")),
	Collapse:       key.NewBinding(key.WithKeys("[")),
	ExpandAll:      key.NewBinding(key.WithKeys("}")),
	CollapseAll:    key.NewBinding(key.WithKeys("{")),
	Defer:          key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "defer")),
}
