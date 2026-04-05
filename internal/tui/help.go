package tui

import (
	"fmt"
	"strings"
)

const (
	helpContentWidth = 44
	// helpOverhead is the number of rows consumed by the overlay border and padding.
	helpOverhead = 4 // border top+bottom (2) + padding top+bottom (2)
	// helpWideKeyThreshold is the key-column width above which the gap between
	// key and description is reduced to 0 (the section already looks wide enough).
	helpWideKeyThreshold = 20
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

// helpKV formats a keybinding line: key column padded to keyW, then gap spaces, then description grayed.
func helpKV(keyW, gap int, k, description string) string {
	return fmt.Sprintf("  %-*s", keyW, k) + strings.Repeat(" ", gap) + helpDescStyle.Render(description)
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
	defs := allKeyDefs()

	// Group by section, preserving first-appearance order.
	type entry struct{ k, desc string }
	sections := map[string][]entry{}
	var sectionOrder []string
	seen := map[string]bool{}
	for _, d := range defs {
		if d.Help().Key == "" {
			continue
		}
		sec := d.section
		if !seen[sec] {
			seen[sec] = true
			sectionOrder = append(sectionOrder, sec)
		}
		sections[sec] = append(sections[sec], entry{d.Help().Key, d.Help().Desc})
	}

	// Compute max key width per section for alignment.
	keyW := map[string]int{}
	for sec, entries := range sections {
		for _, e := range entries {
			if len(e.k) > keyW[sec] {
				keyW[sec] = len(e.k)
			}
		}
	}

	var lines []string
	for i, sec := range sectionOrder {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, helpSection(sec))
		w := keyW[sec]
		gap := 2
		if w > helpWideKeyThreshold {
			gap = 0
		}
		for _, e := range sections[sec] {
			lines = append(lines, helpKV(w, gap, e.k, e.desc))
		}
	}
	return lines
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
