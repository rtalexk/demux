package tui

import (
    "fmt"
    "os"
    "strings"
    "time"

    "github.com/charmbracelet/bubbles/key"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    runewidth "github.com/mattn/go-runewidth"
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
type panesMsg struct {
    panes          []tmux.Pane
    currentSession string // populated by CurrentTarget(); used for startup focus in Task 4
    currentWindow  int
}
type alertsMsg struct{ alerts []db.Alert }
type procDataMsg struct {
    procs  []proc.Process
    cwdMap map[int32]string
    gen    int // generation counter — stale results are discarded
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

    pulse        bool
    spinnerFrame int
    statusMsg    string
    statusExp    time.Time
    ready        bool // true after first panesMsg — gates deferred fetches
    procGen      int  // incremented on window change; discards in-flight proc fetches for old window
    popupMode    bool // true when launched with DEMUX_POPUP=1; quits after attach
}

func New(cfg config.Config, database *db.DB) Model {
    initStyles(ThemeFromConfig(cfg.Theme), cfg.Theme.Processes, cfg.IgnoredProcesses)
    return Model{
        cfg:       cfg,
        db:        database,
        focus:     panelSidebar,
        gitInfo:   make(map[string]git.Info),
        filter:    NewFilterModel(),
        popupMode: os.Getenv("DEMUX_POPUP") == "1",
    }
}

func (m Model) Init() tea.Cmd {
    // Only fetch panes on startup — sidebar renders immediately.
    // fetchAlerts, fetchProcs, and the tick are deferred until panesMsg arrives.
    return m.fetchPanes()
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
        session, window, _ := tmux.CurrentTarget()
        return panesMsg{panes: panes, currentSession: session, currentWindow: window}
    }
}

func (m Model) fetchAlerts() tea.Cmd {
    return func() tea.Msg {
        alerts, _ := m.db.AlertList()
        return alertsMsg{alerts: alerts}
    }
}

// scheduleProcFetch fires an immediate proc snapshot tagged with the current generation.
// Stale results (gen mismatch) are discarded in the procDataMsg handler.
func (m Model) scheduleProcFetch() tea.Cmd {
    gen := m.procGen
    return func() tea.Msg {
        procs, err := proc.Snapshot()
        if err != nil {
            return procDataMsg{gen: gen}
        }
        cwdMap, _ := proc.CWDAll()
        if cwdMap == nil {
            cwdMap = make(map[int32]string)
        }
        return procDataMsg{procs: procs, cwdMap: cwdMap, gen: gen}
    }
}

// scheduleDelayedProcFetch schedules a proc snapshot after 2s, tagged with the current generation.
func (m Model) scheduleDelayedProcFetch() tea.Cmd {
    gen := m.procGen
    return tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
        procs, err := proc.Snapshot()
        if err != nil {
            return procDataMsg{gen: gen}
        }
        cwdMap, _ := proc.CWDAll()
        if cwdMap == nil {
            cwdMap = make(map[int32]string)
        }
        return procDataMsg{procs: procs, cwdMap: cwdMap, gen: gen}
    })
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
        m.spinnerFrame++
        if time.Now().After(m.statusExp) {
            m.statusMsg = ""
        }
        return m, tea.Batch(tick(), m.fetchPanes(), m.fetchAlerts())
    case panesMsg:
        m.panes = msg.panes
        grouped := tmux.GroupBySessions(msg.panes)
        m.sidebar.SetData(msg.panes, m.alerts, m.gitInfo, tmux.SessionActivityMap(msg.panes), m.cfg)
        m.updateDetailFromSelection()
        var cmds []tea.Cmd
        if !m.ready {
            // First load: sidebar is visible — kick off tick and alerts; procs are fetched on-demand
            m.ready = true
            cmds = append(cmds, tick(), m.fetchAlerts())
        }
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
        if msg.gen != m.procGen {
            // Stale result from a previously selected window — discard.
            return m, nil
        }
        m.procs = msg.procs
        m.cwdMap = msg.cwdMap
        if node := m.sidebar.Selected(); node != nil && !node.IsSession {
            m.procList.SetWindowData(m.panes, node.Session, node.WindowIndex, m.procs, m.cwdMap, m.gitInfo, m.alertMap(), m.cfg)
        }
        m.updateDetailFromSelection()
        // Self-schedule next poll in 2s for the selected window.
        return m, m.scheduleDelayedProcFetch()
    case alertsMsg:
        m.alerts = msg.alerts
        m.sidebar.SetData(m.panes, msg.alerts, m.gitInfo, tmux.SessionActivityMap(m.panes), m.cfg)
        m.updateDetailFromSelection()
        return m, nil
    case gitResultMsg:
        m.gitInfo[msg.key] = msg.info
        m.sidebar.SetData(m.panes, m.alerts, m.gitInfo, tmux.SessionActivityMap(m.panes), m.cfg)
        m.updateDetailFromSelection()
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
        m.updateDetailFromSelection()
    case key.Matches(msg, keys.FocusProcList):
        m.focus = panelProcList
        m.updateDetailFromSelection()
    case key.Matches(msg, keys.Help):
        m.showHelp = !m.showHelp
    case key.Matches(msg, keys.Yank):
        m.populateYankFields()
        m.showYank = true
    case key.Matches(msg, keys.Filter):
        m.showFilter = true
        m.filter = NewFilterModel()
    case key.Matches(msg, keys.Refresh):
        m.procGen++
        return m, tea.Batch(m.fetchPanes(), m.fetchAlerts(), m.scheduleProcFetch())
    case key.Matches(msg, keys.AlertFilter):
        sidebarVisibleRows := m.height - 1 - 2
        if sidebarVisibleRows < 1 {
            sidebarVisibleRows = 1
        }
        m.sidebar.ToggleAlertFilter(sidebarVisibleRows)
        if node := m.sidebar.Selected(); node != nil && !node.IsSession {
            prevSess, prevWin := m.procList.CurrentWindow()
            m.procList.SetWindowData(m.panes, node.Session, node.WindowIndex, m.procs, m.cwdMap, m.gitInfo, m.alertMap(), m.cfg)
            m.updateDetailFromSelection()
            if node.Session != prevSess || node.WindowIndex != prevWin {
                m.procGen++
                return m, m.scheduleProcFetch()
            }
        } else {
            m.procList.Reset()
            m.updateDetailFromSelection()
        }
    default:
        if m.focus == panelSidebar {
            return m.handleSidebarKey(msg)
        }
        return m.handleProcListKey(msg)
    }
    return m, nil
}

func (m Model) handleSidebarKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    sidebarVisibleRows := m.height - 1 - 2 // contentH (height-1) minus border (2)
    if sidebarVisibleRows < 1 {
        sidebarVisibleRows = 1
    }

    switch {
    case key.Matches(msg, keys.Up):
        m.sidebar.MoveUp(sidebarVisibleRows)
    case key.Matches(msg, keys.Down):
        m.sidebar.MoveDown(sidebarVisibleRows)
    case key.Matches(msg, keys.Tab):
        m.sidebar.TabNextSession(sidebarVisibleRows)
    case key.Matches(msg, keys.ShiftTab):
        m.sidebar.TabPrevSession(sidebarVisibleRows)
    case key.Matches(msg, keys.GotoTop):
        m.sidebar.GotoTop(sidebarVisibleRows)
    case key.Matches(msg, keys.GotoBottom):
        m.sidebar.GotoBottom(sidebarVisibleRows)
    case key.Matches(msg, keys.Enter):
        if node := m.sidebar.Selected(); node != nil {
            if node.IsSession {
                m.sidebar.ToggleExpand()
            } else {
                // move focus to proclist
                m.focus = panelProcList
                m.procList.SetWindowData(m.panes, node.Session, node.WindowIndex, m.procs, m.cwdMap, m.gitInfo, m.alertMap(), m.cfg)
                m.updateDetailFromSelection()
            }
        }
    case key.Matches(msg, keys.Open):
        if node := m.sidebar.Selected(); node != nil {
            if node.IsSession {
                tmux.SwitchClient(node.Session)
            } else {
                m.resolveAlertForWindow(node.Session, node.WindowIndex)
                tmux.SwitchClient(fmt.Sprintf("%s:%d", node.Session, node.WindowIndex))
            }
            if m.popupMode {
                return m, tea.Quit
            }
        }
    case key.Matches(msg, keys.Esc):
        m.sidebar.MoveToSessionLevel()
    }
    // auto-populate proc list whenever a window node is highlighted (no Enter needed)
    // Trigger a fresh proc fetch when the highlighted window changes.
    // Clear the proc list when a session node is highlighted.
    var cmd tea.Cmd
    if node := m.sidebar.Selected(); node != nil {
        if !node.IsSession {
            prevSess, prevWin := m.procList.CurrentWindow()
            m.procList.SetWindowData(m.panes, node.Session, node.WindowIndex, m.procs, m.cwdMap, m.gitInfo, m.alertMap(), m.cfg)
            if node.Session != prevSess || node.WindowIndex != prevWin {
                m.procGen++
                cmd = m.scheduleProcFetch()
            }
        } else {
            m.procList.Reset()
        }
    }
    m.updateDetailFromSelection()
    return m, cmd
}

func (m Model) handleProcListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    contentH := m.height - 1
    innerW := m.width - m.cfg.SidebarWidth - 2
    if innerW < 1 {
        innerW = 1
    }
    detailContent := m.detail.ContentLines(innerW)
    detailH := detailContent + 2
    if detailH < 4 {
        detailH = 4
    }
    maxDetailH := contentH - 4
    if detailH > maxDetailH {
        detailH = maxDetailH
    }
    procH := contentH - detailH

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
    case key.Matches(msg, keys.GotoTop):
        m.procList.GotoTop()
    case key.Matches(msg, keys.GotoBottom):
        m.procList.GotoBottom()
    case key.Matches(msg, keys.Enter):
        if m.procList.ToggleCollapse() {
            // Rebuild immediately with current data. The nil guard is defensive: if the
            // sidebar selection is lost between refreshes, the next poll cycle rebuilds instead.
            if node := m.sidebar.Selected(); node != nil && !node.IsSession {
                m.procList.SetWindowData(m.panes, node.Session, node.WindowIndex, m.procs, m.cwdMap, m.gitInfo, m.alertMap(), m.cfg)
            }
            m.procGen++
            m.procList.clampOffset(procH - 2)
            m.updateDetailFromSelection()
            return m, m.scheduleDelayedProcFetch()
        }
        if node := m.sidebar.Selected(); node != nil && !node.IsSession {
            m.resolveAlertForWindow(node.Session, node.WindowIndex)
            tmux.SwitchClient(fmt.Sprintf("%s:%d", node.Session, node.WindowIndex))
            if m.popupMode {
                return m, tea.Quit
            }
        }
    case key.Matches(msg, keys.Open):
        if pane := m.procList.SelectedPane(); pane != nil {
            m.resolveAlertForWindow(pane.Session, pane.WindowIndex)
            tmux.SwitchClient(fmt.Sprintf("%s:%d.%d", pane.Session, pane.WindowIndex, pane.PaneIndex))
            if m.popupMode {
                return m, tea.Quit
            }
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
    m.procList.clampOffset(procH - 2) // procH includes border; pass inner content height
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
    prefix := fmt.Sprintf("%s:%d.", session, windowIndex)
    exact := fmt.Sprintf("%s:%d", session, windowIndex)
    for _, a := range m.alerts {
        if (a.Target != exact && !strings.HasPrefix(a.Target, prefix)) || a.Sticky {
            continue
        }
        if err := m.db.AlertRemove(a.Target); err != nil {
            m.statusMsg = "error removing alert: " + err.Error()
            m.statusExp = time.Now().Add(2 * time.Second)
        }
    }
}

func (m *Model) alertMap() map[string]db.Alert {
    am := make(map[string]db.Alert, len(m.alerts))
    for _, a := range m.alerts {
        am[a.Target] = a
    }
    return am
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
            paneCount := 0
            for _, wp := range windows {
                paneCount += len(wp)
            }
            m.detail = DetailModel{
                cfg:        m.cfg,
                selType:    DetailSession,
                session:    node.Session,
                sessionCWD: sessionCWD,
                gitInfo:    m.gitInfo[node.Session],
                winCount:   len(windows),
                paneCount:  paneCount,
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
            sessionCWD := primaryCWDForPanes(windows)
            alertCount := 0
            for _, a := range m.alerts {
                if strings.HasPrefix(a.Target, node.Session+":") {
                    alertCount++
                }
            }
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
                cfg:         m.cfg,
                selType:     DetailWindow,
                session:     node.Session,
                sessionCWD:  sessionCWD,
                gitInfo:     m.gitInfo[node.Session],
                winCount:    len(windows),
                procCount:   procCount,
                alertCount:  alertCount,
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
            cfg:      m.cfg,
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

    // sidebar spans full content height; proclist + detail stack on the right
    contentH := m.height - 1 // 1 (status bar)
    innerW := procW - 2
    detailContent := m.detail.ContentLines(innerW)
    detailH := detailContent + 2 // +2 for border
    minDetailH := 4
    maxDetailH := contentH - 4 // leave at least 4 rows for proc list
    if detailH < minDetailH {
        detailH = minDetailH
    }
    if detailH > maxDetailH {
        detailH = maxDetailH
    }
    procH := contentH - detailH

    // build sidebar border title: [1] Sessions (N) with optional spinner
    spinnerFrames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
    sessionCount := m.sidebar.SessionCount()
    sessionCountStr := statValueStyle.Render(fmt.Sprintf("(%d)", sessionCount))
    alertFilterMark := ""
    if m.sidebar.AlertFilterActive() {
        alertFilterMark = " [!]"
    }
    sidebarTitle := fmt.Sprintf(" [1] Sessions %s%s ", sessionCountStr, alertFilterMark)
    sidebarRightTitle := ""
    for _, info := range m.gitInfo {
        if info.Loading {
            frame := spinnerFrames[m.spinnerFrame%len(spinnerFrames)]
            sidebarRightTitle = fmt.Sprintf(" %s ", spinnerStyle.Render(frame))
            break
        }
    }

    // build proc list border title: [2] <session> / <window>
    bc := m.plainBreadcrumb()
    procTitleSuffix := " "
    if runes := []rune(bc); len(runes) > 0 && isIconRune(runes[len(runes)-1]) {
        procTitleSuffix = "  "
    }
    procTitle := " [2] " + bc + procTitleSuffix

    sidebar := m.sidebar.Render(sidebarW, contentH, m.focus == panelSidebar, sidebarTitle, sidebarRightTitle)
    procList := m.procList.Render(procW, procH, m.focus == panelProcList, procTitle)
    detail := m.detail.Render(procW, detailH)

    right := lipgloss.JoinVertical(lipgloss.Left, procList, detail)
    content := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, right)
    body := content

    statusBar := ""
    if m.statusMsg != "" && time.Now().Before(m.statusExp) {
        statusBar = m.statusMsg
    }
    if statusBar == "" {
        if m.focus == panelSidebar {
            statusBar = "  Tab:cycle  j/k:nav  Enter:select  !:alerts  ?:help  q:quit"
        } else {
            statusBar = "  Tab:cycle  j/k:nav  J/K:jump  x:kill  r:restart  l:log  q:quit"
        }
    }

    if m.showYank {
        overlay := lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.yank.Render())
        return overlay
    }
    if m.showFilter {
        return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.filter.Render())
    }

    statusBar = lipgloss.NewStyle().
        Width(m.width).
        MaxHeight(1).
        Render(statusBar)

    return lipgloss.JoinVertical(lipgloss.Left, body, statusBar)
}

// isIconRune reports whether r is likely a terminal icon that renders as 2
// columns: emoji (runewidth > 1), Nerd Font / Private Use Area glyphs
// (U+E000–U+F8FF, U+F0000+), and common symbol blocks that many terminals
// render wide.
func isIconRune(r rune) bool {
    if runewidth.RuneWidth(r) > 1 {
        return true
    }
    // Private Use Area — Nerd Font icons live here and render as 2-wide
    // in terminals even though Unicode assigns them width 1.
    if r >= 0xE000 && r <= 0xF8FF {
        return true
    }
    if r >= 0xF0000 {
        return true
    }
    return false
}

func (m Model) plainBreadcrumb() string {
    node := m.sidebar.Selected()
    if node == nil {
        return ""
    }
    if node.IsSession {
        return node.Session
    }
    windows := m.sidebar.WindowsForSession(node.Session)
    winLabel := fmt.Sprintf("%d", node.WindowIndex)
    if panes, ok := windows[node.WindowIndex]; ok && len(panes) > 0 {
        winLabel = fmt.Sprintf("%d: %s", node.WindowIndex, panes[0].WindowName)
    }
    return node.Session + " / " + winLabel
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
