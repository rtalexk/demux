package tui

import (
	"fmt"
	"strings"
)

// truncateTarget shortens target to maxLen runes, preserving the :window.pane
// suffix so it remains readable. The session name is truncated with "…".
func truncateTarget(target string, maxLen int) string {
	runes := []rune(target)
	if len(runes) <= maxLen {
		return target
	}
	// Preserve everything from the first ':' onwards.
	idx := strings.IndexByte(target, ':')
	if idx < 0 {
		// No suffix — truncate the whole thing.
		return string(runes[:maxLen-1]) + "…"
	}
	suffix := []rune(target[idx:])        // e.g. ":2.0"
	available := maxLen - len(suffix) - 1 // -1 for "…"
	if available <= 0 {
		return "…" + string(suffix)
	}
	session := []rune(target[:idx])
	if len(session) <= available {
		return target
	}
	return string(session[:available]) + "…" + string(suffix)
}

// ConfirmModel is a small confirmation popover.
// Set prompt to the question text and body to optional detail lines.
type ConfirmModel struct {
	prompt string
	body   string
}

// Render returns the styled confirmation box.
func (c ConfirmModel) Render() string {
	var sb strings.Builder
	sb.WriteString(c.prompt)
	if c.body != "" {
		sb.WriteString("\n\n")
		sb.WriteString(c.body)
	}
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("  %s    confirm\n  %s  cancel",
		hintStyle.Render("y / Enter"),
		hintStyle.Render("n / Esc / q"),
	))
	return confirmStyle.Render(sb.String())
}
