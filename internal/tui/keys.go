package tui

import "github.com/charmbracelet/bubbles/key"

// keyDef wraps a key.Binding with section membership and display order for
// the help overlay. section="" means the binding is not shown in help.
type keyDef struct {
    key.Binding
    section string
    order   int
}

type keyMap struct {
    FocusSidebar   keyDef
    FocusProcList  keyDef
    Yank           keyDef
    Refresh        keyDef
    Help           keyDef
    Quit           keyDef
    Up             keyDef
    Down           keyDef
    Enter          keyDef
    Esc            keyDef
    Kill           keyDef
    Restart        keyDef
    Log            keyDef
    JumpUp         keyDef
    JumpDown       keyDef
    Tab            keyDef
    ShiftTab       keyDef
    GotoTop        keyDef
    GotoBottom     keyDef
    FilterTmux     keyDef
    FilterAll      keyDef
    FilterConfig   keyDef
    FilterWorktree keyDef
    Open           keyDef
    AlertFilter    keyDef
    Expand         keyDef
    Collapse       keyDef
    ExpandAll      keyDef
    CollapseAll    keyDef
    Defer          keyDef
}

var keys = keyMap{
    // Global
    FocusSidebar:  keyDef{key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "sidebar")), "Global", 1},
    FocusProcList: keyDef{key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "procs")), "Global", 2},
    Yank:          keyDef{key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "yank")), "Global", 3},
    Refresh:       keyDef{key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "refresh")), "Global", 4},
    Help:          keyDef{key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")), "Global", 5},
    Quit:          keyDef{key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")), "Global", 6},

    // Navigation
    Up:         keyDef{key.NewBinding(key.WithKeys("k", "up", "ctrl+k", "ctrl+p"), key.WithHelp("j/k · ctrl+j/n · ctrl+k/p", "navigate")), "Navigation", 1},
    Down:       keyDef{key.NewBinding(key.WithKeys("j", "down", "ctrl+j", "ctrl+n"), key.WithHelp("j/k · ctrl+j/n · ctrl+k/p", "navigate")), "Navigation", 1},
    Tab:        keyDef{key.NewBinding(key.WithKeys("tab"), key.WithHelp("Tab/Shift+Tab", "cycle (wraps)")), "Navigation", 2},
    ShiftTab:   keyDef{key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("Tab/Shift+Tab", "cycle (wraps)")), "Navigation", 2},
    GotoTop:    keyDef{key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top")), "Navigation", 3},
    GotoBottom: keyDef{key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")), "Navigation", 4},

    // Sidebar
    Enter: keyDef{key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", "select")), "Sidebar", 1},
    Open:  keyDef{key.NewBinding(key.WithKeys("o", "ctrl+o"), key.WithHelp("o/ctrl+o", "open/attach")), "Sidebar", 2},
    Esc:   keyDef{key.NewBinding(key.WithKeys("esc"), key.WithHelp("Esc", "back")), "Sidebar", 3},
    Defer: keyDef{key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "defer")), "Sidebar", 4},

    // Filters
    FilterTmux:     keyDef{key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "tmux")), "Filters", 1},
    FilterAll:      keyDef{key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "all")), "Filters", 2},
    FilterConfig:   keyDef{key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "config")), "Filters", 3},
    FilterWorktree: keyDef{key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "worktree")), "Filters", 4},
    AlertFilter:    keyDef{key.NewBinding(key.WithKeys("!"), key.WithHelp("!", "alert filter")), "Filters", 5},

    // Process list
    JumpUp:      keyDef{key.NewBinding(key.WithKeys("K"), key.WithHelp("J", "jump down")), "Process list", 1},
    JumpDown:    keyDef{key.NewBinding(key.WithKeys("J"), key.WithHelp("K", "jump up")), "Process list", 2},
    Expand:      keyDef{key.NewBinding(key.WithKeys("]"), key.WithHelp("]", "expand")), "Process list", 3},
    Collapse:    keyDef{key.NewBinding(key.WithKeys("["), key.WithHelp("[", "collapse")), "Process list", 4},
    ExpandAll:   keyDef{key.NewBinding(key.WithKeys("}"), key.WithHelp("}", "expand all")), "Process list", 5},
    CollapseAll: keyDef{key.NewBinding(key.WithKeys("{"), key.WithHelp("{", "collapse all")), "Process list", 6},
    Kill:        keyDef{key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "kill")), "Process list", 7},
    Restart:     keyDef{key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "restart")), "Process list", 8},
    Log:         keyDef{key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "log")), "Process list", 9},
}

// allKeyDefs returns the ordered list of keyDefs for help menu rendering.
// Bindings with empty section are omitted.
func allKeyDefs() []keyDef {
    return []keyDef{
        keys.FocusSidebar, keys.FocusProcList, keys.Yank, keys.Refresh, keys.Help, keys.Quit,
        keys.Up, keys.Tab, keys.GotoTop, keys.GotoBottom,
        keys.Enter, keys.Open, keys.Esc, keys.Defer,
        keys.FilterTmux, keys.FilterAll, keys.FilterConfig, keys.FilterWorktree, keys.AlertFilter,
        keys.JumpUp, keys.JumpDown, keys.Expand, keys.Collapse, keys.ExpandAll, keys.CollapseAll,
        keys.Kill, keys.Restart, keys.Log,
    }
}
