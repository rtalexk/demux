package tui

import (
    "strings"

    "github.com/charmbracelet/lipgloss"
)

var helpStyle = lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(lipgloss.Color("62")).
    Padding(1, 2)

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
        "  Esc      back to session level",
        "",
        "Process list",
        "  j / k    navigate",
        "  J / K    jump to next/prev pane",
        "  Enter    attach to session:window",
        "  x        kill process",
        "  r        restart process",
        "  l        open log popup",
    }
    return helpStyle.Render(strings.Join(lines, "\n"))
}
