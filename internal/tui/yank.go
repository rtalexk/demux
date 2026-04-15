package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/charmbracelet/lipgloss"
	runewidth "github.com/mattn/go-runewidth"
)

type YankField struct {
	Key   string
	Label string
	Value string
}

type YankModel struct {
	fields            []YankField
	cursor            int
	title             string
	marquee           Marquee
	lastMarqueeCursor int
}

func (y *YankModel) SetFields(fields []YankField, title string) {
	y.fields = fields
	y.cursor = 0
	y.title = title
	y.marquee.Reset()
	y.lastMarqueeCursor = 0
}

// TickMarquee advances the yank marquee by one step, resetting on cursor change.
func (y *YankModel) TickMarquee() {
	if y.cursor != y.lastMarqueeCursor {
		y.marquee.Reset()
		y.lastMarqueeCursor = y.cursor
		return
	}
	y.marquee.Tick()
}

func (y YankModel) FieldByKey(k string) (YankField, bool) {
	for _, f := range y.fields {
		if f.Key == k {
			return f, true
		}
	}
	return YankField{}, false
}

const yankValueMaxWidth = 60

// displayValue returns the display string for field f at position fieldIdx.
// Values within yankValueMaxWidth are returned as-is.
// For the cursor row, an overflowing value is shown as a scrolling marquee window.
// For non-cursor rows, an overflowing value is statically truncated.
func (y YankModel) displayValue(fieldIdx int, f YankField) string {
	if runewidth.StringWidth(f.Value) <= yankValueMaxWidth {
		return f.Value
	}
	if fieldIdx == y.cursor {
		return y.marquee.View(f.Value, yankValueMaxWidth)
	}
	runes := []rune(f.Value)
	for runewidth.StringWidth(string(runes)) > yankValueMaxWidth-1 {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + ellipsis
}

func (y YankModel) Render() string {
	maxW := 0
	for i, f := range y.fields {
		dv := y.displayValue(i, f)
		if w := lipgloss.Width(menuItemIndent + fmt.Sprintf("[%s] %-12s %s", f.Key, f.Label, dv)); w > maxW {
			maxW = w
		}
	}

	var sb strings.Builder
	sb.WriteString(y.title)
	sb.WriteString("\n\n")
	for i, f := range y.fields {
		dv := y.displayValue(i, f)
		line := fmt.Sprintf("[%s] %-12s %s", f.Key, f.Label, dv)
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

func (y *YankModel) MoveUp() {
	if y.cursor > 0 {
		y.cursor--
	}
}

func (y *YankModel) MoveDown() {
	if y.cursor < len(y.fields)-1 {
		y.cursor++
	}
}

func (y YankModel) SelectedValue() string {
	if y.cursor < len(y.fields) {
		return y.fields[y.cursor].Value
	}
	return ""
}

func CopyToClipboard(text string) error {
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
