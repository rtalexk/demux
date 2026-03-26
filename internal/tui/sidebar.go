package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/rtalex/demux/internal/config"
	"github.com/rtalex/demux/internal/db"
	"github.com/rtalex/demux/internal/git"
	"github.com/rtalex/demux/internal/tmux"
)

var (
	borderActive   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62"))
	borderInactive = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("244"))
	sessionStyle   = lipgloss.NewStyle().Bold(true)
	windowIndent   = "  "
	selectedBG     = lipgloss.NewStyle().Background(lipgloss.Color("62")).Foreground(lipgloss.Color("230"))
)

type SidebarNode struct {
	Session     string
	WindowIndex int
	IsSession   bool
	Expanded    bool
}

type SidebarModel struct {
	nodes    []SidebarNode
	cursor   int
	offset   int // viewport scroll offset
	sessions map[string]map[int][]tmux.Pane
	alerts   map[string]db.Alert
	gitInfo  map[string]git.Info
	cfg      config.Config
}

func (s *SidebarModel) SetData(panes []tmux.Pane, alerts []db.Alert, gitInfo map[string]git.Info, cfg config.Config) {
	s.sessions = tmux.GroupBySessions(panes)
	s.alerts = make(map[string]db.Alert, len(alerts))
	for _, a := range alerts {
		s.alerts[a.Target] = a
	}
	s.gitInfo = gitInfo
	s.cfg = cfg

	// preserve cursor position, rebuild nodes
	selectedSession := ""
	selectedWindow := -1
	if s.cursor < len(s.nodes) {
		n := s.nodes[s.cursor]
		selectedSession = n.Session
		selectedWindow = n.WindowIndex
	}

	s.rebuildNodes()

	// restore cursor
	for i, n := range s.nodes {
		if n.Session == selectedSession && n.WindowIndex == selectedWindow {
			s.cursor = i
			break
		}
	}
	if s.cursor >= len(s.nodes) {
		s.cursor = max(0, len(s.nodes)-1)
	}
}

func (s *SidebarModel) rebuildNodes() {
	// preserve expanded state
	expanded := map[string]bool{}
	for _, n := range s.nodes {
		if n.IsSession {
			expanded[n.Session] = n.Expanded
		}
	}

	s.nodes = nil
	// sort sessions for stable ordering
	sessions := make([]string, 0, len(s.sessions))
	for name := range s.sessions {
		sessions = append(sessions, name)
	}
	sort.Strings(sessions)

	for _, name := range sessions {
		if s.cfg.IgnoredSessions != nil {
			ignored := false
			for _, ig := range s.cfg.IgnoredSessions {
				if ig == name {
					ignored = true
					break
				}
			}
			if ignored {
				continue
			}
		}

		exp, ok := expanded[name]
		if !ok {
			exp = true // default expanded
		}
		s.nodes = append(s.nodes, SidebarNode{Session: name, IsSession: true, Expanded: exp})
		if exp {
			windows := s.sessions[name]
			winIdxs := make([]int, 0, len(windows))
			for wi := range windows {
				winIdxs = append(winIdxs, wi)
			}
			sort.Ints(winIdxs)
			for _, wi := range winIdxs {
				s.nodes = append(s.nodes, SidebarNode{Session: name, WindowIndex: wi})
			}
		}
	}
}

func (s SidebarModel) Render(width, height int, focused bool) string {
	visibleRows := height - 2
	if visibleRows < 1 {
		visibleRows = 1
	}

	// compute display offset without mutating (Bubbletea passes model by value in View)
	offset := s.offset
	if s.cursor < offset {
		offset = s.cursor
	}
	if s.cursor >= offset+visibleRows {
		offset = s.cursor - visibleRows + 1
	}
	if offset < 0 {
		offset = 0
	}

	// reserve 1 row for the "▼ more" hint when nodes extend below the viewport
	hasMore := offset+visibleRows < len(s.nodes)
	contentRows := visibleRows
	if hasMore {
		contentRows = visibleRows - 1
	}

	var lines []string
	end := offset + contentRows
	if end > len(s.nodes) {
		end = len(s.nodes)
	}
	for i := offset; i < end; i++ {
		line := s.renderNode(s.nodes[i], i == s.cursor, width)
		lines = append(lines, line)
	}
	if hasMore {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("▼ more"))
	}

	inner := strings.Join(lines, "\n")
	style := borderInactive
	if focused {
		style = borderActive
	}
	return style.Width(width - 2).Height(height - 2).Render(inner)
}

func (s SidebarModel) renderNode(node SidebarNode, selected bool, width int) string {
	var text string
	if node.IsSession {
		text = s.renderSession(node, width)
	} else {
		text = s.renderWindow(node, width)
	}
	if selected {
		return selectedBG.Render(text)
	}
	return text
}

func (s SidebarModel) renderSession(node SidebarNode, width int) string {
	prefix := "▼ "
	if !node.Expanded {
		prefix = "▶ "
	}

	// Build the indicator suffix first
	indicatorSuffix := ""
	if info, ok := s.gitInfo[node.Session]; ok {
		if info.Loading {
			indicatorSuffix += "  …"
		} else {
			if ind := compactGitIndicators(info); ind != "" {
				indicatorSuffix += "  " + ind
			}
		}
	} else if s.cfg.Git.Enabled {
		indicatorSuffix += "  …"
	}
	for target, a := range s.alerts {
		if strings.HasPrefix(target, node.Session+":") {
			indicatorSuffix += "  " + alertIcon(a.Level)
			break
		}
	}

	// Truncate session name to fit available width, leaving room for indicators
	maxW := width - 4
	nameRunes := []rune(prefix + node.Session)
	indRunes := len([]rune(stripANSI(indicatorSuffix)))
	maxName := maxW - indRunes
	if maxName < 4 {
		maxName = 4
	}
	if len(nameRunes) > maxName {
		nameRunes = append(nameRunes[:maxName-1], '…')
	}
	text := string(nameRunes) + indicatorSuffix

	return sessionStyle.Render(text)
}

func (s SidebarModel) renderWindow(node SidebarNode, width int) string {
	windows := s.sessions[node.Session]
	primaryCWD := primaryCWDForPanes(windows)
	wPanes := windows[node.WindowIndex]

	name := fmt.Sprintf("%d", node.WindowIndex)
	if len(wPanes) > 0 && wPanes[0].WindowName != "" {
		name = fmt.Sprintf("%d: %s", node.WindowIndex, wPanes[0].WindowName)
	}

	// Build the indicator suffix first
	indicatorSuffix := ""
	winCWD := windowCWDFromPanes(wPanes)
	if winCWD != "" && !git.IsDescendant(winCWD, primaryCWD) && winCWD != primaryCWD {
		gitKey := fmt.Sprintf("%s:%d", node.Session, node.WindowIndex)
		if info, ok := s.gitInfo[gitKey]; ok {
			if info.Loading {
				indicatorSuffix += "  ↪ …"
			} else {
				indicatorSuffix += "  ↪"
				if ind := compactGitIndicators(info); ind != "" {
					indicatorSuffix += " " + ind
				}
			}
		} else {
			indicatorSuffix += "  ↪ …"
		}
	}
	target := fmt.Sprintf("%s:%d", node.Session, node.WindowIndex)
	if a, ok := s.alerts[target]; ok {
		indicatorSuffix += "  " + alertIcon(a.Level)
	}

	// Truncate name to fit available width, leaving room for indicators
	maxW := width - 4
	nameRunes := []rune(windowIndent + name)
	indRunes := len([]rune(stripANSI(indicatorSuffix)))
	maxName := maxW - indRunes
	if maxName < 4 {
		maxName = 4
	}
	if len(nameRunes) > maxName {
		nameRunes = append(nameRunes[:maxName-1], '…')
	}
	text := string(nameRunes) + indicatorSuffix

	return text
}

func compactGitIndicators(info git.Info) string {
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

func alertIcon(level string) string {
	switch level {
	case "info":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Render("●")
	case "warn":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Render("")
	case "error":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true).Render("")
	}
	return ""
}

func windowCWDFromPanes(panes []tmux.Pane) string {
	for _, p := range panes {
		if p.PaneIndex == 0 {
			return p.CWD
		}
	}
	if len(panes) > 0 {
		return panes[0].CWD
	}
	return ""
}

func (s *SidebarModel) clampViewport(visibleRows int) {
	if s.cursor < s.offset {
		s.offset = s.cursor
	}
	if s.cursor >= s.offset+visibleRows {
		s.offset = s.cursor - visibleRows + 1
	}
	if s.offset < 0 {
		s.offset = 0
	}
}

func (s *SidebarModel) MoveUp(visibleRows int) {
	if s.cursor > 0 {
		s.cursor--
	}
	s.clampViewport(visibleRows)
}

func (s *SidebarModel) MoveDown(visibleRows int) {
	if s.cursor < len(s.nodes)-1 {
		s.cursor++
	}
	s.clampViewport(visibleRows)
}

func (s SidebarModel) WindowsForSession(session string) map[int][]tmux.Pane {
	return s.sessions[session]
}

func (s *SidebarModel) ToggleExpand() {
	if s.cursor < len(s.nodes) && s.nodes[s.cursor].IsSession {
		s.nodes[s.cursor].Expanded = !s.nodes[s.cursor].Expanded
		s.rebuildNodes()
	}
}

func (s *SidebarModel) MoveToSessionLevel() {
	for s.cursor > 0 && !s.nodes[s.cursor].IsSession {
		s.cursor--
	}
}

func (s SidebarModel) Selected() *SidebarNode {
	if s.cursor < 0 || s.cursor >= len(s.nodes) {
		return nil
	}
	n := s.nodes[s.cursor]
	return &n
}


