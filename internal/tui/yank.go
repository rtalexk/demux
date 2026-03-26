package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var yankStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("62")).
	Padding(1, 2)

type YankField struct {
	Key   string
	Label string
	Value string
}

type YankModel struct {
	fields []YankField
	cursor int
}

func (y *YankModel) SetFields(fields []YankField) {
	y.fields = fields
	y.cursor = 0
}

func (y YankModel) Render() string {
	var lines []string
	for i, f := range y.fields {
		line := fmt.Sprintf("[%s] %-12s %s", f.Key, f.Label, f.Value)
		if i == y.cursor {
			line = selectedBG.Render(line)
		}
		lines = append(lines, line)
	}
	return yankStyle.Render(strings.Join(lines, "\n"))
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
