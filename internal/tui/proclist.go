package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/rtalex/demux/internal/config"
	"github.com/rtalex/demux/internal/git"
	"github.com/rtalex/demux/internal/proc"
	"github.com/rtalex/demux/internal/tmux"
)

var (
	procBorderActive   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62"))
	procBorderInactive = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240"))
	paneHeaderStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("33"))
	procLine1Style     = lipgloss.NewStyle().PaddingLeft(2)
	procLine2Style     = lipgloss.NewStyle().PaddingLeft(4).Foreground(lipgloss.Color("240"))
)

type ProcListNode struct {
	IsPaneHeader bool
	Pane         tmux.Pane
	GitDeviant   bool
	GitInfo      git.Info
	Proc         proc.Process
	Port         int
}

type ProcListModel struct {
	nodes      []ProcListNode
	cursor     int
	filterText string
	primaryCWD string
}

func (p *ProcListModel) SetWindowData(panes []tmux.Pane, session string, windowIndex int, gitInfo map[string]git.Info, cfg config.Config) {
	grouped := tmux.GroupBySessions(panes)
	windows := grouped[session]
	p.primaryCWD = primaryCWDForPanes(windows)
	wPanes := windows[windowIndex]

	p.nodes = nil
	for _, pane := range sortPanes(wPanes) {
		paneCWD := pane.CWD
		gitKey := fmt.Sprintf("%s:%d:%d", pane.Session, pane.WindowIndex, pane.PaneIndex)
		info := gitInfo[gitKey]
		deviant := p.primaryCWD != "" && !git.IsDescendant(paneCWD, p.primaryCWD) && paneCWD != p.primaryCWD

		p.nodes = append(p.nodes, ProcListNode{
			IsPaneHeader: true,
			Pane:         pane,
			GitDeviant:   deviant,
			GitInfo:      info,
		})

		// find processes for this pane by CWD match
		allProcs, _ := proc.Snapshot()
		for _, pr := range allProcs {
			cwd, err := proc.CWD(pr.PID)
			if err != nil || cwd != paneCWD {
				continue
			}
			p.nodes = append(p.nodes, ProcListNode{Proc: pr})
		}
	}
}

func sortPanes(panes []tmux.Pane) []tmux.Pane {
	sorted := make([]tmux.Pane, len(panes))
	copy(sorted, panes)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].PaneIndex < sorted[i].PaneIndex {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	return sorted
}

func (p *ProcListModel) SetFilter(text string) {
	p.filterText = text
}

func (p ProcListModel) Render(width, height int, focused bool) string {
	border := procBorderInactive
	if focused {
		border = procBorderActive
	}

	filter := strings.ToLower(p.filterText)
	var lines []string

	for i, node := range p.nodes {
		selected := i == p.cursor
		var line string
		if node.IsPaneHeader {
			line = p.renderPaneHeader(node, selected)
		} else {
			line = p.renderProc(node, selected)
		}
		if filter == "" || strings.Contains(strings.ToLower(stripANSI(line)), filter) {
			lines = append(lines, line)
		}
	}

	inner := strings.Join(lines, "\n")
	return border.Width(width - 2).Height(height - 2).Render(inner)
}

func (p ProcListModel) renderPaneHeader(node ProcListNode, selected bool) string {
	text := fmt.Sprintf("pane %d", node.Pane.PaneIndex)
	if node.Pane.CWD != "" {
		text += "  " + node.Pane.CWD
	}
	if node.GitDeviant {
		if node.GitInfo.Loading {
			text += "  ↪ …"
		} else {
			text += "  ↪ " + compactGitIndicators(node.GitInfo)
		}
	}
	line := paneHeaderStyle.Render(text)
	if selected {
		return selectedBG.Render(text)
	}
	return line
}

func (p ProcListModel) renderProc(node ProcListNode, selected bool) string {
	pr := node.Proc
	line1 := pr.Name
	if pr.PID > 0 {
		line1 += fmt.Sprintf("  pid:%d", pr.PID)
	}
	if node.Port > 0 {
		line1 += fmt.Sprintf("  :%d", node.Port)
	}
	line2 := fmt.Sprintf("cpu:%.1f%%  mem:%.1fMB  up:%s",
		pr.CPU,
		float64(pr.MemRSS)/1024/1024,
		formatProcDuration(pr.Uptime),
	)

	l1 := procLine1Style.Render(line1)
	l2 := procLine2Style.Render(line2)
	if selected {
		l1 = selectedBG.Render("  " + line1)
	}
	return l1 + "\n" + l2
}

func formatProcDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	switch {
	case h >= 24:
		return fmt.Sprintf("%dd%dh", h/24, h%24)
	case h > 0:
		return fmt.Sprintf("%dh%dm", h, m)
	case m > 0:
		return fmt.Sprintf("%dm", m)
	default:
		return fmt.Sprintf("%ds", s)
	}
}

func stripANSI(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && s[i] != 'm' {
				i++
			}
			i++ // skip 'm'
			continue
		}
		result.WriteByte(s[i])
		i++
	}
	return result.String()
}

func (p *ProcListModel) MoveUp() {
	if p.cursor > 0 {
		p.cursor--
	}
}

func (p *ProcListModel) MoveDown() {
	if p.cursor < len(p.nodes)-1 {
		p.cursor++
	}
}

func (p *ProcListModel) JumpToNextPane() {
	for i := p.cursor + 1; i < len(p.nodes); i++ {
		if p.nodes[i].IsPaneHeader {
			p.cursor = i
			return
		}
	}
}

func (p *ProcListModel) JumpToPrevPane() {
	for i := p.cursor - 1; i >= 0; i-- {
		if p.nodes[i].IsPaneHeader {
			p.cursor = i
			return
		}
	}
}

func (p ProcListModel) SelectedNode() *ProcListNode {
	if p.cursor < 0 || p.cursor >= len(p.nodes) {
		return nil
	}
	n := p.nodes[p.cursor]
	return &n
}
