package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/rtalex/demux/internal/db"
	"github.com/rtalex/demux/internal/git"
	"github.com/rtalex/demux/internal/proc"
	"github.com/rtalex/demux/internal/tmux"
)

var (
	detailBorder     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240"))
	detailLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Width(10)
	detailValueStyle = lipgloss.NewStyle()
)

type DetailSelection int

const (
	DetailNone DetailSelection = iota
	DetailSession
	DetailWindow
	DetailProc
)

type DetailModel struct {
	selType DetailSelection

	// session
	session    string
	sessionCWD string
	gitInfo    git.Info
	prInfo     string
	winCount   int
	procCount  int
	alertCount int

	// window
	windowIndex int
	windowPanes []tmux.Pane
	windowGit   git.Info
	windowAlert *db.Alert

	// proc
	proc    proc.Process
	procGit git.Info
}

func (d DetailModel) Render(width, height int) string {
	var lines []string
	switch d.selType {
	case DetailSession:
		lines = d.renderSession()
	case DetailWindow:
		lines = d.renderWindow()
	case DetailProc:
		lines = d.renderProc()
	default:
		lines = []string{""}
	}
	inner := strings.Join(lines, "\n")
	return detailBorder.Width(width - 2).Height(height - 2).Render(inner)
}

func row(label, value string) string {
	return detailLabelStyle.Render(label) + detailValueStyle.Render(value)
}

func (d DetailModel) renderSession() []string {
	lines := []string{
		row("repo", d.sessionCWD),
		row("branch", d.gitInfo.Branch),
	}
	if ind := gitIndicatorsLong(d.gitInfo); ind != "" {
		lines = append(lines, row("", ind))
	}
	if d.prInfo != "" {
		lines = append(lines, row("pr", d.prInfo))
	}
	lines = append(lines,
		"",
		row("windows", fmt.Sprint(d.winCount)),
		row("procs", fmt.Sprint(d.procCount)),
		row("alerts", fmt.Sprint(d.alertCount)),
	)
	return lines
}

func (d DetailModel) renderWindow() []string {
	var lines []string
	for _, p := range d.windowPanes {
		lines = append(lines, fmt.Sprintf("  pane %d  %s", p.PaneIndex, p.CWD))
	}
	if d.windowGit.Branch != "" {
		lines = append(lines, "")
		lines = append(lines, row("branch", d.windowGit.Branch))
		if ind := gitIndicatorsLong(d.windowGit); ind != "" {
			lines = append(lines, row("", ind))
		}
	}
	if d.windowAlert != nil {
		lines = append(lines, "")
		lines = append(lines, row("alert", d.windowAlert.Level+": "+d.windowAlert.Reason))
	}
	return lines
}

func (d DetailModel) renderProc() []string {
	lines := []string{
		row("name", d.proc.Name),
		row("pid", fmt.Sprint(d.proc.PID)),
		row("cmd", d.proc.Cmdline),
		row("uptime", formatProcDuration(d.proc.Uptime)),
		row("memory", fmt.Sprintf("%.1fMB", float64(d.proc.MemRSS)/1024/1024)),
	}
	if d.procGit.Branch != "" {
		lines = append(lines, "")
		lines = append(lines, row("branch", d.procGit.Branch))
		if ind := gitIndicatorsLong(d.procGit); ind != "" {
			lines = append(lines, row("", ind))
		}
	}
	return lines
}

func gitIndicatorsLong(info git.Info) string {
	var parts []string
	if info.Ahead > 0 {
		parts = append(parts, fmt.Sprintf("↑%d", info.Ahead))
	}
	if info.Behind > 0 {
		parts = append(parts, fmt.Sprintf("↓%d", info.Behind))
	}
	if info.Dirty {
		parts = append(parts, "*")
	}
	return strings.Join(parts, " ")
}
