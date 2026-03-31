package tui

import (
    "strings"
)

const helpContentWidth = 44

type HelpModel struct{}

func helpSection(name string) string {
    prefix := paneSepStyle.Render("─── ")
    label := paneHeaderStyle.Render(name)
    fillLen := helpContentWidth - 4 - len(name) - 1
    if fillLen < 0 {
        fillLen = 0
    }
    suffix := paneSepStyle.Render(" " + strings.Repeat("─", fillLen))
    return prefix + label + suffix
}

func (h HelpModel) Render() string {
    lines := []string{
        helpSection("Global"),
        "  h / l        focus sidebar / process list",
        "  y            yank menu",
        "  f            filter",
        "  ctrl+u       clear filter",
        "  R            force refresh",
        "  ?            toggle help",
        "  q            quit",
        "",
        helpSection("Sidebar"),
        "  j / k        navigate",
        "  Tab          navigate (wraps)",
        "  Shift+Tab    navigate backward (wraps)",
        "  G            go to bottom",
        "  Enter        expand session / select window",
        "  o            attach to session / window",
        "  Esc          back to session level",
        "",
        helpSection("Filters"),
        "  t        tmux sessions only (default)",
        "  a        all sessions (tmux + config)",
        "  g        config sessions only",
        "  w        sessions in current worktree",
        "  !        alert filter",
        "",
        helpSection("Process list"),
        "  j / k    navigate",
        "  J / K    jump to next/prev pane",
        "  G        go to bottom",
        "  ] / [    expand / collapse group",
        "  } / {    expand / collapse all",
        "  Enter    attach to session:window",
        "  o        attach to pane",
        "  x        kill process",
        "  r        restart process",
        "  L        open log popup",
    }
    return helpStyle.Render(strings.Join(lines, "\n"))
}
