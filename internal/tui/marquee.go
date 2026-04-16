package tui

import (
	"strings"

	runewidth "github.com/mattn/go-runewidth"
)

const marqueeGap = 8 // spaces inserted between end of text and its circular restart

// Marquee scrolls text horizontally in a circular loop.
// Embed in a model; call Tick() each marquee tick and Reset() when the target text changes.
// View() is a value receiver — safe to call from value-receiver render functions.
type Marquee struct {
	offset int
}

// Reset sets the scroll position back to 0.
func (m *Marquee) Reset() {
	m.offset = 0
}

// Tick advances the scroll position by one rune position.
// Cycle length is measured in rune count, not display columns.
func (m *Marquee) Tick() {
	m.offset++
}

// TickTracked advances the marquee, resetting if cursor moved since the last tick.
// Pass the current cursor and a pointer to the last-seen cursor value (owned by the caller).
func (m *Marquee) TickTracked(cursor int, last *int) {
	if cursor != *last {
		m.Reset()
		*last = cursor
		return
	}
	m.Tick()
}

// View returns exactly width display columns of text starting at the current offset.
// If text fits within width it is returned unchanged.
// The circular sequence is: text + marqueeGap spaces + text (repeating).
func (m Marquee) View(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if runewidth.StringWidth(text) <= width {
		return text
	}

	runes := []rune(text)
	// cycle is measured in rune count, not display columns (see Tick doc).
	cycle := len(runes) + marqueeGap

	// Build: text + gap + text  (sufficient for any offset+width window because width < len(runes))
	extended := make([]rune, 0, len(runes)*2+marqueeGap)
	extended = append(extended, runes...)
	for range marqueeGap {
		extended = append(extended, ' ')
	}
	extended = append(extended, runes...)

	start := m.offset % cycle

	var out []rune
	cols := 0
	for i := start; i < len(extended) && cols < width; i++ {
		w := runewidth.RuneWidth(extended[i])
		if cols+w > width {
			break
		}
		out = append(out, extended[i])
		cols += w
	}
	if cols < width {
		out = append(out, []rune(strings.Repeat(" ", width-cols))...)
	}
	return string(out)
}
