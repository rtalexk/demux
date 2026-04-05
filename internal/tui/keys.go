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
    FocusSidebar        keyDef
    FocusProcList       keyDef
    FocusSidebarList    keyDef // display-only combined entry
    Yank                keyDef
    FilterSearch        keyDef // display-only: f (search/filter)
    ClearFilter         keyDef // display-only: ctrl+u (clear filter)
    Refresh             keyDef
    Help                keyDef
    Quit                keyDef
    Navigate            keyDef // display-only combined navigation entry
    Up                  keyDef
    Down                keyDef
    Enter               keyDef
    Esc                 keyDef
    Kill                keyDef
    Restart             keyDef
    Log                 keyDef
    JumpUpDown          keyDef // display-only combined entry
    JumpUp              keyDef
    JumpDown            keyDef
    Tab                 keyDef
    ShiftTab            keyDef
    GotoTop             keyDef
    GotoBottom          keyDef
    FilterTmux          keyDef
    FilterAll           keyDef
    FilterConfig        keyDef
    FilterWorktree      keyDef
    Open                keyDef
    AlertFilter         keyDef
    ExpandCollapse      keyDef // display-only combined entry
    Expand              keyDef
    Collapse            keyDef
    ExpandCollapseAll   keyDef // display-only combined entry
    ExpandAll           keyDef
    CollapseAll         keyDef
    Defer               keyDef
    DeferSticky         keyDef
    ProcEnter           keyDef // display-only: Enter for process list
    ProcOpen            keyDef // display-only: o/ctrl+o for process list
}

var keys = keyMap{
    // Global
    FocusSidebar:     keyDef{key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "sidebar")), "", 0},
    FocusProcList:    keyDef{key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "procs")), "", 0},
    FocusSidebarList: keyDef{key.NewBinding(key.WithHelp("h / l", "focus sidebar / process list")), "Global", 1},
    Yank:             keyDef{key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "yank menu")), "Global", 2},
    FilterSearch:     keyDef{key.NewBinding(key.WithHelp("f", "filter")), "Global", 3},
    ClearFilter:      keyDef{key.NewBinding(key.WithHelp("ctrl+u", "clear filter")), "Global", 4},
    Refresh:          keyDef{key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "force refresh")), "Global", 5},
    Help:             keyDef{key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "toggle help")), "Global", 6},
    Quit:             keyDef{key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")), "Global", 7},

    // Navigation
    Navigate:   keyDef{key.NewBinding(key.WithHelp("j/k · ctrl+j/n · ctrl+k/p", "navigate")), "Navigation", 1},
    Up:         keyDef{key.NewBinding(key.WithKeys("k", "up", "ctrl+k", "ctrl+p"), key.WithHelp("j/k · ctrl+j/n · ctrl+k/p", "navigate")), "", 0},
    Down:       keyDef{key.NewBinding(key.WithKeys("j", "down", "ctrl+j", "ctrl+n"), key.WithHelp("j/k · ctrl+j/n · ctrl+k/p", "navigate")), "", 0},
    Tab:        keyDef{key.NewBinding(key.WithKeys("tab"), key.WithHelp("Tab/Shift+Tab", "cycle (wraps)")), "Navigation", 2},
    ShiftTab:   keyDef{key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("Tab/Shift+Tab", "cycle (wraps)")), "", 0},
    GotoTop:    keyDef{key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top")), "Navigation", 3},
    GotoBottom: keyDef{key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")), "Navigation", 4},

    // Sidebar
    Enter: keyDef{key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", "attach to session")), "Sidebar", 1},
    Open:  keyDef{key.NewBinding(key.WithKeys("o", "ctrl+o"), key.WithHelp("o / ctrl+o", "attach to session / window")), "Sidebar", 2},
    Esc:   keyDef{key.NewBinding(key.WithKeys("esc"), key.WithHelp("Esc", "back to session level")), "Sidebar", 3},
    Defer:       keyDef{key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "defer")), "Sidebar", 4},
    DeferSticky: keyDef{key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "defer (sticky)")), "Sidebar", 5},

    // Filters
    FilterTmux:     keyDef{key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "tmux sessions only (default)")), "Filters", 1},
    FilterAll:      keyDef{key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "all sessions (tmux + config)")), "Filters", 2},
    FilterConfig:   keyDef{key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "config sessions only")), "Filters", 3},
    FilterWorktree: keyDef{key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "sessions in current worktree")), "Filters", 4},
    AlertFilter:    keyDef{key.NewBinding(key.WithKeys("!"), key.WithHelp("!", "alert filter")), "Filters", 5},

    // Process list
    JumpUpDown:        keyDef{key.NewBinding(key.WithHelp("J / K", "jump to next/prev pane")), "Process list", 1},
    JumpUp:            keyDef{key.NewBinding(key.WithKeys("K"), key.WithHelp("K", "jump up")), "", 0},
    JumpDown:          keyDef{key.NewBinding(key.WithKeys("J"), key.WithHelp("J", "jump down")), "", 0},
    ExpandCollapse:    keyDef{key.NewBinding(key.WithHelp("] / [", "expand / collapse group")), "Process list", 2},
    Expand:            keyDef{key.NewBinding(key.WithKeys("]"), key.WithHelp("]", "expand")), "", 0},
    Collapse:          keyDef{key.NewBinding(key.WithKeys("["), key.WithHelp("[", "collapse")), "", 0},
    ExpandCollapseAll: keyDef{key.NewBinding(key.WithHelp("} / {", "expand / collapse all")), "Process list", 3},
    ExpandAll:         keyDef{key.NewBinding(key.WithKeys("}"), key.WithHelp("}", "expand all")), "", 0},
    CollapseAll:       keyDef{key.NewBinding(key.WithKeys("{"), key.WithHelp("{", "collapse all")), "", 0},
    ProcEnter:         keyDef{key.NewBinding(key.WithHelp("Enter", "toggle expand / collapse")), "Process list", 4},
    ProcOpen:          keyDef{key.NewBinding(key.WithHelp("o / ctrl+o", "attach to pane")), "Process list", 5},
    Kill:              keyDef{key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "kill process")), "Process list", 6},
    Restart:           keyDef{key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "restart process")), "Process list", 7},
    Log:               keyDef{key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "open log popup")), "Process list", 8},
}

// allKeyDefs returns the ordered list of keyDefs for help menu rendering.
// Bindings with empty section are omitted.
func allKeyDefs() []keyDef {
    return []keyDef{
        // Global
        keys.FocusSidebarList, keys.Yank, keys.FilterSearch, keys.ClearFilter,
        keys.Refresh, keys.Help, keys.Quit,
        // Navigation
        keys.Navigate, keys.Tab, keys.GotoTop, keys.GotoBottom,
        // Sidebar
        keys.Enter, keys.Open, keys.Esc, keys.Defer, keys.DeferSticky,
        // Filters
        keys.FilterTmux, keys.FilterAll, keys.FilterConfig, keys.FilterWorktree, keys.AlertFilter,
        // Process list
        keys.JumpUpDown, keys.ExpandCollapse, keys.ExpandCollapseAll,
        keys.ProcEnter, keys.ProcOpen, keys.Kill, keys.Restart, keys.Log,
    }
}
