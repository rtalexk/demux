package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rtalex/demux/internal/config"
	"github.com/rtalex/demux/internal/db"
	"github.com/rtalex/demux/internal/git"
	"github.com/rtalex/demux/internal/proc"
	"github.com/rtalex/demux/internal/tmux"
)

type panel int

const (
	panelSidebar panel = iota
	panelProcList
)

// Message types
type tickMsg time.Time
type panesMsg struct{ panes []tmux.Pane }
type alertsMsg struct{ alerts []db.Alert }
type procDataMsg struct {
	procs  []proc.Process
	cwdMap map[int32]string
}
type gitResultMsg struct {
	key  string // session name, or "session:window" for deviants
	info git.Info
}

type Model struct {
	cfg    config.Config
	db     *db.DB
	focus  panel
	width  int
	height int

	panes   []tmux.Pane
	alerts  []db.Alert
	gitInfo map[string]git.Info // keyed by session name
	procs   []proc.Process
	cwdMap  map[int32]string // PID -> CWD, pre-fetched async

	sidebar  SidebarModel
	procList ProcListModel
	detail   DetailModel
	yank     YankModel
	filter   FilterModel
	help     HelpModel

	showYank   bool
	showFilter bool
	showHelp   bool

	pulse     bool
	statusMsg string
	statusExp time.Time
}

func New(cfg config.Config, database *db.DB) Model {
	return Model{
		cfg:     cfg,
		db:      database,
		focus:   panelSidebar,
		gitInfo: make(map[string]git.Info),
		filter:  NewFilterModel(),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(tick(), m.fetchPanes(), m.fetchAlerts(), fetchProcs())
}

func tick() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) fetchPanes() tea.Cmd {
	return func() tea.Msg {
		panes, err := tmux.ListPanes()
		if err != nil {
			return panesMsg{}
		}
		return panesMsg{panes: panes}
	}
}

func (m Model) fetchAlerts() tea.Cmd {
	return func() tea.Msg {
		alerts, _ := m.db.AlertList()
		return alertsMsg{alerts: alerts}
	}
}

func fetchProcs() tea.Cmd {
	return func() tea.Msg {
		procs, err := proc.Snapshot()
		if err != nil {
			return procDataMsg{}
		}
		cwdMap := make(map[int32]string, len(procs))
		for _, p := range procs {
			if cwd, err := proc.CWD(p.PID); err == nil {
				cwdMap[p.PID] = cwd
			}
		}
		return procDataMsg{procs: procs, cwdMap: cwdMap}
	}
}

func fetchGit(k, dir string, timeoutMs int) tea.Cmd {
	return func() tea.Msg {
		info, err := git.Fetch(dir, timeoutMs)
		if err != nil {
			return gitResultMsg{key: k, info: git.Info{}}
		}
		return gitResultMsg{key: k, info: info}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Delegate to overlay handlers first
	if m.showFilter {
		return m.updateFilter(msg)
	}
	if m.showYank {
		return m.updateYank(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tickMsg:
		m.pulse = !m.pulse
		if time.Now().After(m.statusExp) {
			m.statusMsg = ""
		}
		return m, tea.Batch(tick(), m.fetchPanes(), m.fetchAlerts(), fetchProcs())
	case panesMsg:
		m.panes = msg.panes
		grouped := tmux.GroupBySessions(msg.panes)
		m.sidebar.SetData(msg.panes, m.alerts, m.gitInfo, m.cfg)
		m.updateDetailFromSelection()
		var cmds []tea.Cmd
		if m.cfg.Git.Enabled {
			for sessionName, windows := range grouped {
				info := m.gitInfo[sessionName]
				info.Loading = true
				m.gitInfo[sessionName] = info
				primaryCWD := primaryCWDForPanes(windows)
				if primaryCWD != "" {
					cmds = append(cmds, fetchGit(sessionName, primaryCWD, m.cfg.Git.TimeoutMs))
				}
			}
		}
		return m, tea.Batch(cmds...)
	case procDataMsg:
		m.procs = msg.procs
		m.cwdMap = msg.cwdMap
		m.updateDetailFromSelection()
	case alertsMsg:
		m.alerts = msg.alerts
		m.sidebar.SetData(m.panes, msg.alerts, m.gitInfo, m.cfg)
		return m, nil
	case gitResultMsg:
		m.gitInfo[msg.key] = msg.info
		m.sidebar.SetData(m.panes, m.alerts, m.gitInfo, m.cfg)
		return m, nil
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, keys.FocusSidebar):
		m.focus = panelSidebar
	case key.Matches(msg, keys.FocusProcList):
		m.focus = panelProcList
	case key.Matches(msg, keys.Help):
		m.showHelp = !m.showHelp
	case key.Matches(msg, keys.Yank):
		m.populateYankFields()
		m.showYank = true
	case key.Matches(msg, keys.Filter):
		m.showFilter = true
		m.filter = NewFilterModel()
	case key.Matches(msg, keys.Refresh):
		return m, tea.Batch(m.fetchPanes(), m.fetchAlerts(), fetchProcs())
	default:
		if m.focus == panelSidebar {
			return m.handleSidebarKey(msg)
		}
		return m.handleProcListKey(msg)
	}
	return m, nil
}

func (m Model) handleSidebarKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	detailH := 8
	topH := m.height - detailH - 4
	sidebarVisibleRows := topH - 2
	if sidebarVisibleRows < 1 {
		sidebarVisibleRows = 1
	}

	switch {
	case key.Matches(msg, keys.Up):
		m.sidebar.MoveUp(sidebarVisibleRows)
	case key.Matches(msg, keys.Down):
		m.sidebar.MoveDown(sidebarVisibleRows)
	case key.Matches(msg, keys.Tab):
		m.sidebar.MoveDown(sidebarVisibleRows)
	case key.Matches(msg, keys.Enter):
		if node := m.sidebar.Selected(); node != nil {
			if node.IsSession {
				m.sidebar.ToggleExpand()
			} else {
				// select window, resolve non-sticky alert, move focus to proclist
				m.resolveAlertForWindow(node.Session, node.WindowIndex)
				m.focus = panelProcList
				m.procList.SetWindowData(m.panes, node.Session, node.WindowIndex, m.procs, m.cwdMap, m.gitInfo, m.cfg)
				m.updateDetailFromSelection()
			}
		}
	case key.Matches(msg, keys.Esc):
		m.sidebar.MoveToSessionLevel()
	}
	m.updateDetailFromSelection()
	return m, nil
}

func (m Model) handleProcListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Up):
		m.procList.MoveUp()
	case key.Matches(msg, keys.Down):
		m.procList.MoveDown()
	case key.Matches(msg, keys.Tab):
		m.procList.TabNext()
	case key.Matches(msg, keys.JumpUp):
		m.procList.JumpToPrevPane()
	case key.Matches(msg, keys.JumpDown):
		m.procList.JumpToNextPane()
	case key.Matches(msg, keys.Enter):
		if node := m.sidebar.Selected(); node != nil && !node.IsSession {
			tmux.SwitchClient(fmt.Sprintf("%s:%d", node.Session, node.WindowIndex))
		}
	case key.Matches(msg, keys.Esc):
		m.focus = panelSidebar
	case key.Matches(msg, keys.Kill):
		// TODO: confirmation prompt
	case key.Matches(msg, keys.Restart):
		// TODO: restart via tmux send-keys Up Enter
	case key.Matches(msg, keys.Log):
		// TODO: tmux popup with scrollback
	}
	m.updateDetailFromSelection()
	return m, nil
}

func (m *Model) populateYankFields() {
	if m.focus == panelProcList {
		selNode := m.procList.SelectedNode()
		if selNode != nil && !selNode.IsPaneHeader {
			pr := selNode.Proc
			cwd := m.cwdMap[pr.PID]
			portStr := ""
			if selNode.Port > 0 {
				portStr = fmt.Sprintf("%d", selNode.Port)
			}
			m.yank.SetFields([]YankField{
				{Key: "p", Label: "PID", Value: fmt.Sprint(pr.PID)},
				{Key: "n", Label: "name", Value: pr.Name},
				{Key: "c", Label: "cmdline", Value: pr.Cmdline},
				{Key: "d", Label: "CWD", Value: cwd},
				{Key: "o", Label: "port", Value: portStr},
			})
			return
		}
	}
	// session or window node from sidebar
	node := m.sidebar.Selected()
	if node != nil {
		if node.IsSession {
			m.yank.SetFields([]YankField{
				{Key: "n", Label: "session", Value: node.Session},
				{Key: "t", Label: "target", Value: node.Session},
			})
		} else {
			m.yank.SetFields([]YankField{
				{Key: "n", Label: "session", Value: node.Session},
				{Key: "t", Label: "target", Value: fmt.Sprintf("%s:%d", node.Session, node.WindowIndex)},
			})
		}
	}
}

func (m *Model) resolveAlertForWindow(session string, windowIndex int) {
	target := fmt.Sprintf("%s:%d", session, windowIndex)
	alert, err := m.db.AlertByTarget(target)
	if err != nil || alert == nil || alert.Sticky {
		return
	}
	if err := m.db.AlertRemove(target); err != nil {
		m.statusMsg = "error removing alert: " + err.Error()
		m.statusExp = time.Now().Add(2 * time.Second)
	}
}

func (m *Model) updateDetailFromSelection() {
	if m.focus == panelSidebar {
		node := m.sidebar.Selected()
		if node == nil {
			m.detail = DetailModel{}
			return
		}
		if node.IsSession {
			grouped := tmux.GroupBySessions(m.panes)
			windows := grouped[node.Session]
			alertCount := 0
			for _, a := range m.alerts {
				if strings.HasPrefix(a.Target, node.Session+":") {
					alertCount++
				}
			}
			sessionCWD := primaryCWDForPanes(windows)
			// count processes whose CWD is under the session's primary CWD
			procCount := 0
			if sessionCWD != "" {
				for _, pr := range m.procs {
					cwd := m.cwdMap[pr.PID]
					if cwd == "" {
						continue
					}
					if cwd == sessionCWD || git.IsDescendant(cwd, sessionCWD) {
						procCount++
					}
				}
			}
			m.detail = DetailModel{
				selType:    DetailSession,
				session:    node.Session,
				sessionCWD: sessionCWD,
				gitInfo:    m.gitInfo[node.Session],
				winCount:   len(windows),
				procCount:  procCount,
				alertCount: alertCount,
			}
		} else {
			// window node selected
			grouped := tmux.GroupBySessions(m.panes)
			windows := grouped[node.Session]
			wPanes := windows[node.WindowIndex]
			gitKey := fmt.Sprintf("%s:%d", node.Session, node.WindowIndex)
			var windowAlert *db.Alert
			target := fmt.Sprintf("%s:%d", node.Session, node.WindowIndex)
			if a, err := m.db.AlertByTarget(target); err == nil && a != nil {
				windowAlert = a
			}
			m.detail = DetailModel{
				selType:     DetailWindow,
				windowIndex: node.WindowIndex,
				windowPanes: wPanes,
				windowGit:   m.gitInfo[gitKey],
				windowAlert: windowAlert,
			}
		}
		return
	}
	// panelProcList focus
	if m.focus == panelProcList {
		selNode := m.procList.SelectedNode()
		if selNode == nil || selNode.IsPaneHeader {
			m.detail = DetailModel{}
			return
		}
		pr := selNode.Proc
		cwd := m.cwdMap[pr.PID]
		portStr := ""
		if selNode.Port > 0 {
			portStr = fmt.Sprintf("%d", selNode.Port)
		}
		m.detail = DetailModel{
			selType:  DetailProc,
			proc:     pr,
			procGit:  m.gitInfo[cwd],
			procPort: portStr,
			procCWD:  cwd,
		}
	}
}

func (m Model) updateFilter(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, keys.Esc) {
			m.showFilter = false
			m.procList.SetFilter("")
			return m, nil
		}
		if key.Matches(msg, keys.Enter) {
			m.showFilter = false
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	m.procList.SetFilter(m.filter.Value())
	return m, cmd
}

func (m Model) updateYank(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, keys.Esc) || msg.String() == "q" {
			m.showYank = false
			return m, nil
		}
		if key.Matches(msg, keys.Enter) {
			val := m.yank.SelectedValue()
			CopyToClipboard(val)
			m.showYank = false
			m.statusMsg = "yanked: " + val
			m.statusExp = time.Now().Add(2 * time.Second)
			return m, nil
		}
		switch msg.String() {
		case "j", "down":
			m.yank.MoveDown()
		case "k", "up":
			m.yank.MoveUp()
		}
	}
	return m, nil
}

func (m Model) View() string {
	if m.width == 0 {
		return "loading..."
	}

	if m.showHelp {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.help.Render())
	}

	sidebarW := m.cfg.SidebarWidth
	if sidebarW <= 0 {
		sidebarW = 30
	}
	procW := m.width - sidebarW
	if procW < 10 {
		procW = 10
	}

	detailH := 8
	topH := m.height - detailH - 4 // titlebar (3 rows: border+content+border) + statusbar

	sidebar := m.sidebar.Render(sidebarW, topH, m.focus == panelSidebar)
	procList := m.procList.Render(procW, topH, m.focus == panelProcList)
	detail := m.detail.Render(m.width-2, detailH)

	// build breadcrumb from current sidebar selection
	breadcrumb := m.breadcrumb()

	headerBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("244"))

	leftHeader := headerBox.
		Bold(true).
		Width(sidebarW - 2).
		Padding(0, 1).
		Render("Sessions")

	rightHeader := headerBox.
		Width(procW - 2).
		Padding(0, 1).
		Render(breadcrumb)

	titlebar := lipgloss.JoinHorizontal(lipgloss.Top, leftHeader, rightHeader)

	top := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, procList)
	body := lipgloss.JoinVertical(lipgloss.Left, titlebar, top, detail)

	statusBar := ""
	if m.statusMsg != "" && time.Now().Before(m.statusExp) {
		statusBar = m.statusMsg
	}
	if statusBar == "" {
		if m.focus == panelSidebar {
			statusBar = "  1:sidebar  2:procs  j/k:nav  Enter:select  ?:help  q:quit"
		} else {
			statusBar = "  1:sidebar  2:procs  j/k:nav  J/K:jump  x:kill  r:restart  l:log  q:quit"
		}
	}

	if m.showYank {
		overlay := lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.yank.Render())
		return overlay
	}
	if m.showFilter {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.filter.Render())
	}

	return lipgloss.JoinVertical(lipgloss.Left, body, statusBar)
}

func (m Model) breadcrumb() string {
	node := m.sidebar.Selected()
	if node == nil {
		return ""
	}
	if node.IsSession {
		return node.Session
	}
	// window node: find window name from panes
	windows := m.sidebar.WindowsForSession(node.Session)
	winName := fmt.Sprintf("%d", node.WindowIndex)
	if panes, ok := windows[node.WindowIndex]; ok && len(panes) > 0 {
		winName = panes[0].WindowName
	}
	return node.Session + " / " + winName
}

func primaryCWDForPanes(windows map[int][]tmux.Pane) string {
	panes, ok := windows[0]
	if !ok || len(panes) == 0 {
		// try first available window
		for _, ps := range windows {
			if len(ps) > 0 {
				return ps[0].CWD
			}
		}
		return ""
	}
	for _, p := range panes {
		if p.PaneIndex == 0 {
			return p.CWD
		}
	}
	return panes[0].CWD
}

// Run launches the Bubbletea program.
func Run(cfg config.Config, database *db.DB) error {
	m := New(cfg, database)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
