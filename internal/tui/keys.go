package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
    FocusSidebar  key.Binding
    FocusProcList key.Binding
    Yank          key.Binding
    Filter        key.Binding
    Refresh       key.Binding
    Help          key.Binding
    Quit          key.Binding
    Up            key.Binding
    Down          key.Binding
    Enter         key.Binding
    Esc           key.Binding
    Kill          key.Binding
    Restart       key.Binding
    Log           key.Binding
    JumpUp        key.Binding
    JumpDown      key.Binding
    Tab           key.Binding
    ShiftTab      key.Binding
    GotoTop       key.Binding
    GotoBottom    key.Binding
    Open          key.Binding
}

var keys = keyMap{
    FocusSidebar:  key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "sidebar")),
    FocusProcList: key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "procs")),
    Yank:          key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "yank")),
    Filter:        key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "filter")),
    Refresh:       key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "refresh")),
    Help:          key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
    Quit:          key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
    Up:            key.NewBinding(key.WithKeys("k", "up")),
    Down:          key.NewBinding(key.WithKeys("j", "down")),
    Enter:         key.NewBinding(key.WithKeys("enter")),
    Esc:           key.NewBinding(key.WithKeys("esc")),
    Kill:          key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "kill")),
    Restart:       key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "restart")),
    Log:           key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "log")),
    JumpUp:        key.NewBinding(key.WithKeys("K")),
    JumpDown:      key.NewBinding(key.WithKeys("J")),
    Tab:           key.NewBinding(key.WithKeys("tab")),
    ShiftTab:      key.NewBinding(key.WithKeys("shift+tab")),
    GotoTop:       key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top")),
    GotoBottom:    key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
    Open:          key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open/attach")),
}
