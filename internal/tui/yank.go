package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/charmbracelet/lipgloss"
	runewidth "github.com/mattn/go-runewidth"
)

type yankField struct {
	Key   string
	Label string
	Value string
}

type yankModel struct {
	fields            []yankField
	cursor            int
	title             string
	marquee           Marquee
	lastMarqueeCursor int
}

func (y *yankModel) SetFields(fields []yankField, title string) {
	y.fields = fields
	y.cursor = 0
	y.title = title
	y.marquee.Reset()
	y.lastMarqueeCursor = 0
}

// TickMarquee advances the yank marquee by one step, resetting on cursor change.
func (y *yankModel) TickMarquee() {
	y.marquee.TickTracked(y.cursor, &y.lastMarqueeCursor)
}

func (y yankModel) FieldByKey(k string) (yankField, bool) {
	for _, f := range y.fields {
		if f.Key == k {
			return f, true
		}
	}
	return yankField{}, false
}

const yankValueMaxWidth = 60

// displayValue returns the display string for field f at position fieldIdx.
// Values within yankValueMaxWidth are returned as-is.
// For the cursor row, an overflowing value is shown as a scrolling marquee window.
// For non-cursor rows, an overflowing value is statically truncated.
func (y yankModel) displayValue(fieldIdx int, f yankField) string {
	if runewidth.StringWidth(f.Value) <= yankValueMaxWidth {
		return f.Value
	}
	if fieldIdx == y.cursor {
		return y.marquee.View(f.Value, yankValueMaxWidth)
	}
	runes := []rune(f.Value)
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if runewidth.StringWidth(string(runes[:mid])) <= yankValueMaxWidth-1 {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return string(runes[:lo]) + ellipsis
}

func (y yankModel) Render() string {
	dvs := make([]string, len(y.fields))
	maxW := 0
	for i, f := range y.fields {
		dvs[i] = y.displayValue(i, f)
		if w := lipgloss.Width(menuItemIndent + fmt.Sprintf("[%s] %-12s %s", f.Key, f.Label, dvs[i])); w > maxW {
			maxW = w
		}
	}

	var sb strings.Builder
	sb.WriteString(y.title)
	sb.WriteString("\n\n")
	for i, f := range y.fields {
		line := fmt.Sprintf("[%s] %-12s %s", f.Key, f.Label, dvs[i])
		if i == y.cursor {
			line = selectedBG.Width(maxW + menuItemTrailingPad).Render(menuItemIndent + line)
		} else {
			line = menuItemIndent + line
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
	sb.WriteString(hintStyle.Render("Enter / shortcut") + " copy   " + hintStyle.Render("Esc / q") + " close")
	return confirmStyle.Render(sb.String())
}

func (y *yankModel) MoveUp() {
	if y.cursor > 0 {
		y.cursor--
	}
}

func (y *yankModel) MoveDown() {
	if y.cursor < len(y.fields)-1 {
		y.cursor++
	}
}

func (y yankModel) SelectedValue() string {
	if y.cursor < len(y.fields) {
		return y.fields[y.cursor].Value
	}
	return ""
}

// clipboardFunc is the function used to copy text to the clipboard.
// Overridable in tests.
var clipboardFunc = copyToClipboard

func copyToClipboard(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	default:
		if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd = exec.Command("wl-copy")
		} else {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		}
	}
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}
