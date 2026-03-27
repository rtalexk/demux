package tui

import (
    "strings"

)


type HelpModel struct{}

func (h HelpModel) Render() string {
    lines := []string{
        "Global",
        "  1 / 2    focus sidebar / process list",
        "  y        yank menu",
        "  f        filter",
        "  R        force refresh",
        "  ?        toggle help",
        "  q        quit",
        "",
        "Sidebar",
        "  j / k    navigate",
        "  Tab      navigate (wraps)",
        "  Enter    expand session / select window",
        "  o        attach to session / window",
        "  Esc      back to session level",
        "",
        "Process list",
        "  j / k    navigate",
        "  J / K    jump to next/prev pane",
        "  Enter    attach to session:window",
        "  o        attach to pane",
        "  x        kill process",
        "  r        restart process",
        "  l        open log popup",
    }
    return helpStyle.Render(strings.Join(lines, "\n"))
}
