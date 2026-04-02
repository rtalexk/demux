package tui

import (
    "fmt"
    "strings"
)

const (
    helpContentWidth = 44
    // helpOverhead is the number of rows consumed by the overlay border and padding.
    helpOverhead = 4 // border top+bottom (2) + padding top+bottom (2)
)

type HelpModel struct {
    scrollOffset int
}

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

// helpKV formats a keybinding line: key column padded to keyW, description grayed.
func helpKV(keyW int, k, description string) string {
    return fmt.Sprintf("  %-*s", keyW, k) + helpDescStyle.Render(description)
}

// helpNavLine builds a Navigation prose line. Even-indexed parts (key tokens) render
// in normal color; odd-indexed parts (separators / trailing description) are grayed.
func helpNavLine(parts ...string) string {
    var b strings.Builder
    b.WriteString("  ")
    for i, p := range parts {
        if i%2 == 0 {
            b.WriteString(p)
        } else {
            b.WriteString(helpDescStyle.Render(p))
        }
    }
    return b.String()
}

func (h *HelpModel) ScrollUp() {
    if h.scrollOffset > 0 {
        h.scrollOffset--
    }
}

func (h *HelpModel) ScrollDown(availH int) {
    lines := h.buildLines()
    visible := max(availH-helpOverhead, 1)
    maxOffset := max(len(lines)-visible, 0)
    if h.scrollOffset < maxOffset {
        h.scrollOffset++
    }
}

func (h HelpModel) buildLines() []string {
    return []string{
        helpSection("Global"),
        helpKV(8, "h / l", "focus sidebar / process list"),
        helpKV(8, "y", "yank menu"),
        helpKV(8, "f", "filter"),
        helpKV(8, "ctrl+u", "clear filter"),
        helpKV(8, "R", "force refresh"),
        helpKV(8, "?", "toggle help"),
        helpKV(8, "q", "quit"),
        "",
        helpSection("Navigation"),
        helpNavLine("j/k", " · ", "ctrl+j/n", " · ", "ctrl+k/p", " navigate."),
        helpNavLine("Tab/Shift+Tab", " cycles (wraps).  ", "g/G", " jumps."),
        "",
        helpSection("Sidebar"),
        helpKV(7, "Enter", "expand session / select window"),
        helpKV(7, "o", "attach to session / window"),
        helpKV(7, "Esc", "back to session level"),
        "",
        helpSection("Filters"),
        helpKV(3, "t", "tmux sessions only (default)"),
        helpKV(3, "a", "all sessions (tmux + config)"),
        helpKV(3, "c", "config sessions only"),
        helpKV(3, "w", "sessions in current worktree"),
        helpKV(3, "!", "alert filter"),
        "",
        helpSection("Process list"),
        helpKV(7, "J / K", "jump to next/prev pane"),
        helpKV(7, "] / [", "expand / collapse group"),
        helpKV(7, "} / {", "expand / collapse all"),
        helpKV(7, "Enter", "attach to session:window"),
        helpKV(7, "o", "attach to pane"),
        helpKV(7, "x", "kill process"),
        helpKV(7, "r", "restart process"),
        helpKV(7, "L", "open log popup"),
    }
}

// Render returns the styled help overlay, clipped to maxH terminal rows.
// Pass maxH=0 to render without clipping (for tests).
func (h HelpModel) Render(maxH int) string {
    lines := h.buildLines()

    if maxH > 0 {
        visible := max(maxH-helpOverhead, 1)
        if len(lines) > visible {
            start := min(h.scrollOffset, len(lines)-visible)
            lines = lines[start : start+visible]
        }
    }

    return helpStyle.Render(strings.Join(lines, "\n"))
}
