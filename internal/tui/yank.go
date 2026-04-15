package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type YankField struct {
	Key   string
	Label string
	Value string
}

type YankModel struct {
	fields []YankField
	cursor int
	title  string
}

func (y *YankModel) SetFields(fields []YankField, title string) {
	y.fields = fields
	y.cursor = 0
	y.title = title
}

func (y YankModel) FieldByKey(k string) (YankField, bool) {
	for _, f := range y.fields {
		if f.Key == k {
			return f, true
		}
	}
	return YankField{}, false
}

func (y YankModel) Render() string {
	maxW := 0
	for _, f := range y.fields {
		if w := lipgloss.Width(menuItemIndent + fmt.Sprintf("[%s] %-12s %s", f.Key, f.Label, f.Value)); w > maxW {
			maxW = w
		}
	}

	var sb strings.Builder
	sb.WriteString(y.title)
	sb.WriteString("\n\n")
	for i, f := range y.fields {
		line := fmt.Sprintf("[%s] %-12s %s", f.Key, f.Label, f.Value)
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
