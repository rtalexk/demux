package tui

import (
	"fmt"
	"strings"
)

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
