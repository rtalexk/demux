package tui

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "time"

    "github.com/charmbracelet/bubbles/key"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    xansi "github.com/charmbracelet/x/ansi"
    runewidth "github.com/mattn/go-runewidth"
    "github.com/rtalex/demux/internal/config"
    "github.com/rtalex/demux/internal/db"
    "github.com/rtalex/demux/internal/git"
    "github.com/rtalex/demux/internal/proc"
    "github.com/rtalex/demux/internal/query"
    "github.com/rtalex/demux/internal/session"
    "github.com/rtalex/demux/internal/tmux"
)

type panel int

const (
    panelSidebar panel = iota
    panelProcList
)

const searchBoxH = 3

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Message types
type tickMsg time.Time
type panesMsg struct {
    panes          []tmux.Pane
    currentSession string // populated by CurrentTarget(); used for startup focus in Task 4
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

type searchDebounceMsg struct{ gen int }
type queryResultMsg struct {
    result query.Result
    gen    int
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
    help     HelpModel

    showYank bool
    showHelp bool

    pulse        bool
    spinnerFrame int
    statusMsg    string
    statusExp    time.Time
    ready        bool // true after first panesMsg — gates deferred fetches
    procGen      int  // incremented on window change; discards in-flight proc fetches for old window
    popupMode    bool // true when launched with DEMUX_POPUP=1; quits after attach

    currentSession   string
    startupFocusDone bool

    searchInput SearchInputModel
    queryResult query.Result
    searchGen   int

    sessionsConfig session.SessionsConfig
    configDir      string
}

func New(cfg config.Config, database *db.DB) Model {
    initStyles(ThemeFromConfig(cfg.Theme), cfg.Theme.Processes, cfg.IgnoredProcesses)
    m := Model{
        cfg:       cfg,
        db:        database,
        focus:     panelSidebar,
        gitInfo:   make(map[string]git.Info),
        popupMode: os.Getenv("DEMUX_POPUP") == "1",
    }
    m.searchInput = NewSearchInputModel()
    cfgPath, _ := config.DefaultPath()
    m.configDir = filepath.Dir(cfgPath)
    var loadErr error
    m.sessionsConfig, loadErr = session.LoadConfigSessions(m.configDir)
    if loadErr != nil {
        fmt.Fprintf(os.Stderr, "demux: failed to load config sessions from %s: %v\n", m.configDir, loadErr)
    }
    if cfg.Sidebar.DefaultFilter != "" {
        m.sidebar.filter = SidebarFilter(cfg.Sidebar.DefaultFilter)
    } else {
        m.sidebar.filter = FilterTmux
    }
    return m
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

// resolveWindowSpecs maps a session's window name list to tmux.WindowSpec
// values by looking them up in the resolved template map. Unknown names are
// logged to stderr and skipped.
func resolveWindowSpecs(names []string, templates map[string]session.WindowTemplate) []tmux.WindowSpec {
    if len(names) == 0 {
        return nil
    }
    specs := make([]tmux.WindowSpec, 0, len(names))
    for _, name := range names {
        t, ok := templates[name]
        if !ok {
            fmt.Fprintf(os.Stderr, "demux: session references unknown window_template id %q\n", name)
            continue
        }
        specs = append(specs, tmux.WindowSpec{
            Name:           t.Name,
            AfterCreateCmd: t.AfterCreateCmd,
        })
    }
    return specs
}

func (m Model) fetchPanes() tea.Cmd {
    return func() tea.Msg {
        panes, err := tmux.ListPanes()
        if err != nil {
            return panesMsg{}
        }
        session, _, _ := tmux.CurrentTarget()
        return panesMsg{panes: panes, currentSession: session}
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
            // Preserve Dir and IsWorktreeRoot even when git is unavailable.
            return gitResultMsg{key: k, info: git.Info{Dir: info.Dir, IsWorktreeRoot: info.IsWorktreeRoot}}
        }
        return gitResultMsg{key: k, info: info}
    }
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // Delegate to overlay handlers first
    if m.showHelp {
        if msg, ok := msg.(tea.KeyMsg); ok {
            if key.Matches(msg, keys.Esc) || key.Matches(msg, keys.Help) || msg.String() == "q" {
                m.showHelp = false
                return m, nil
            }
        }
        return m, nil
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
        merged := session.Merge(msg.panes, m.sessionsConfig.Entries)
        m.sidebar.SetData(merged, m.alerts, m.gitInfo, m.cfg)
        m.updateDetailFromSelection()
        var cmds []tea.Cmd
        if !m.ready {
            // First load: sidebar is visible — kick off tick and alerts; procs are fetched on-demand
            m.currentSession = msg.currentSession
            switch m.cfg.Sidebar.FocusOnOpen {
            case "current_session", "first_session":
                visibleRows := max(1, m.height-1-2-searchBoxH)
                m.applyNonAlertFocusMode(m.cfg.Sidebar.FocusOnOpen, visibleRows)
            }
            m.ready = true
            cmds = append(cmds, tick(), m.fetchAlerts())
            // If startup focus landed on a window node, kick off an initial proc fetch.
            if node := m.sidebar.Selected(); node != nil {
                m.procGen++
                cmds = append(cmds, m.scheduleProcFetch())
            }
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
        if node := m.sidebar.Selected(); node != nil {
            m.procList.SetSessionData(m.panes, node.Session, m.procs, m.cwdMap, m.gitInfo, m.alertMap(), m.cfg)
        }
        m.updateDetailFromSelection()
        // Self-schedule next poll in 2s for the selected window.
        return m, m.scheduleDelayedProcFetch()
    case alertsMsg:
        m.alerts = msg.alerts
        merged := session.Merge(m.panes, m.sessionsConfig.Entries)
        m.sidebar.SetData(merged, msg.alerts, m.gitInfo, m.cfg)
        if !m.startupFocusDone {
            m.startupFocusDone = true
            visibleRows := max(1, m.height-1-2-searchBoxH)
            if m.cfg.Sidebar.FocusOnOpen == "alert_session" {
                m.sidebar.FocusFirstAlertSession(visibleRows)
            }
            if m.cfg.Sidebar.FocusSearchOnOpen {
                m.searchInput.EnterInsertMode()
            }
        }
        m.updateDetailFromSelection()
        // If startup focus landed on a window node, kick off an initial proc fetch.
        var cmds []tea.Cmd
        if node := m.sidebar.Selected(); node != nil {
            m.procGen++
            cmds = append(cmds, m.scheduleProcFetch())
        }
        if pruneCmd := m.pruneStaleAlerts(); pruneCmd != nil {
            cmds = append(cmds, pruneCmd)
        }
        return m, tea.Batch(cmds...)
    case gitResultMsg:
        m.gitInfo[msg.key] = msg.info
        merged := session.Merge(m.panes, m.sessionsConfig.Entries)
        m.sidebar.SetData(merged, m.alerts, m.gitInfo, m.cfg)
        m.updateDetailFromSelection()
        return m, nil
    case queryResultMsg:
        if msg.gen == m.searchGen {
            m.queryResult = msg.result
            m.sidebar.SetSearchResult(msg.result)
            m.procList.SetSearchQuery(query.Parse(m.searchInput.Value()), msg.result)
            if node := m.sidebar.Selected(); node != nil {
                m.procList.SetSessionData(m.panes, node.Session, m.procs, m.cwdMap, m.gitInfo, m.alertMap(), m.cfg)
                m.procGen++
                m.updateDetailFromSelection()
                return m, m.scheduleProcFetch()
            } else {
                m.procList.Reset()
            }
        }
        return m, nil
    case searchDebounceMsg:
        if msg.gen != m.searchGen {
            return m, nil
        }
        pq := query.Parse(m.searchInput.Value())
        for _, sess := range m.sidebar.sessions {
            if !sess.IsLive {
                pq.ExtraSessions = append(pq.ExtraSessions, sess.DisplayName)
            }
        }
        gen := m.searchGen
        return m, func() tea.Msg {
            result, err := query.Run(pq)
            if err != nil {
                return queryResultMsg{gen: gen}
            }
            return queryResultMsg{result: result, gen: gen}
        }
    }
    return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    if m.searchInput.IsInsert() {
        sidebarVisibleRows := m.height - 1 - 2 - searchBoxH
        if sidebarVisibleRows < 1 {
            sidebarVisibleRows = 1
        }
        m.sidebar.visibleRows = sidebarVisibleRows
        switch msg.String() {
        case "esc", "enter":
            if msg.String() == "enter" {
                if node := m.sidebar.Selected(); node != nil {
                    sess := m.sidebar.FindSession(node.Session)
                    if sess != nil && !sess.IsLive && sess.IsConfig && sess.Config != nil {
                        if err := tmux.NewSession(sess.DisplayName, sess.Config.Path); err != nil {
                            m.statusMsg = "launch failed: " + err.Error()
                            m.statusExp = time.Now().Add(5 * time.Second)
                            m.sidebar.SetLaunchErr(err.Error())
                            m.searchInput.ExitInsertMode()
                            return m, nil
                        }
                        if specs := resolveWindowSpecs(sess.Config.Windows, m.sessionsConfig.WindowTemplates); len(specs) > 0 {
                            if err := tmux.CreateSessionWindows(sess.DisplayName, specs); err != nil {
                                m.statusMsg = "window setup failed: " + err.Error()
                                m.statusExp = time.Now().Add(5 * time.Second)
                            }
                        }
                        m.sidebar.ClearLaunchErr()
                        m.searchInput.ExitInsertMode()
                        return m, m.fetchPanes()
                    }
                    if err := tmux.SwitchClient(node.Session); err == nil {
                        if m.popupMode {
                            return m, tea.Quit
                        }
                    }
                }
            }
            m.searchInput.ExitInsertMode()
            return m, nil
        case "ctrl+j", "ctrl+n":
            m.sidebar.CursorDown()
            if node := m.sidebar.Selected(); node != nil {
                m.procList.SetSessionData(m.panes, node.Session, m.procs, m.cwdMap, m.gitInfo, m.alertMap(), m.cfg)
                m.procGen++
                m.updateDetailFromSelection()
                return m, m.scheduleProcFetch()
            }
            return m, nil
        case "ctrl+k", "ctrl+p":
            m.sidebar.CursorUp()
            if node := m.sidebar.Selected(); node != nil {
                m.procList.SetSessionData(m.panes, node.Session, m.procs, m.cwdMap, m.gitInfo, m.alertMap(), m.cfg)
                m.procGen++
                m.updateDetailFromSelection()
                return m, m.scheduleProcFetch()
            }
            return m, nil
        default:
            var cmd tea.Cmd
            prevVal := m.searchInput.Value()
            m.searchInput, cmd = m.searchInput.Update(msg)
            if m.searchInput.Value() != prevVal {
                if m.searchInput.Value() == "" {
                    // Input cleared: reset immediately without waiting for debounce.
                    m.queryResult = query.Result{}
                    m.sidebar.SetSearchResult(query.Result{})
                    m.procList.SetSearchQuery(query.ParsedQuery{}, query.Result{})
                    m.searchGen++ // cancel any in-flight query
                    return m, cmd
                }
                m.searchGen++
                return m, tea.Batch(cmd, debounceSearch(m.searchGen))
            }
            return m, cmd
        }
    }

    switch {
    case key.Matches(msg, keys.Quit):
        return m, tea.Quit
    case key.Matches(msg, keys.FocusSidebar):
        m.focus = panelSidebar
        m.updateDetailFromSelection()
    case key.Matches(msg, keys.FocusProcList):
        // compact mode: FocusProcList is a no-op; panelSidebar is the only panel
        if m.cfg.Mode != "compact" {
            m.focus = panelProcList
        }
        // updateDetailFromSelection gates on m.focus, so safe to call unconditionally
        m.updateDetailFromSelection()
    case key.Matches(msg, keys.Help):
        m.showHelp = !m.showHelp
    case key.Matches(msg, keys.Yank):
        m.populateYankFields()
        m.showYank = true
    case key.Matches(msg, keys.Refresh):
        m.procGen++
        return m, tea.Batch(m.fetchPanes(), m.fetchAlerts(), m.scheduleProcFetch())
    case key.Matches(msg, keys.AlertFilter),
        key.Matches(msg, keys.FilterTmux),
        key.Matches(msg, keys.FilterAll),
        key.Matches(msg, keys.FilterConfig),
        key.Matches(msg, keys.FilterWorktree):

        sidebarVisibleRows := m.height - 1 - 2 - searchBoxH
        if sidebarVisibleRows < 1 {
            sidebarVisibleRows = 1
        }
        var newFilter SidebarFilter
        switch {
        case key.Matches(msg, keys.AlertFilter):
            newFilter = FilterPriority
        case key.Matches(msg, keys.FilterTmux):
            newFilter = FilterTmux
        case key.Matches(msg, keys.FilterAll):
            newFilter = FilterAll
        case key.Matches(msg, keys.FilterConfig):
            newFilter = FilterConfig
        case key.Matches(msg, keys.FilterWorktree):
            newFilter = FilterWorktree
        }
        m.sidebar.SetFilter(newFilter, sidebarVisibleRows)
        if node := m.sidebar.Selected(); node != nil {
            m.procList.SetSessionData(m.panes, node.Session, m.procs, m.cwdMap, m.gitInfo, m.alertMap(), m.cfg)
            m.updateDetailFromSelection()
        }
    default:
        if msg.String() == "f" {
            m.searchInput.EnterInsertMode()
            return m, nil
        }
        if msg.String() == "ctrl+u" && m.searchInput.IsActive() {
            m.searchInput.Clear()
            m.queryResult = query.Result{}
            m.sidebar.SetSearchResult(query.Result{})
            m.procList.SetSearchQuery(query.ParsedQuery{}, query.Result{})
            m.searchGen++
            return m, nil
        }
        if m.focus == panelSidebar {
            return m.handleSidebarKey(msg)
        }
        return m.handleProcListKey(msg)
    }
    return m, nil
}

func (m Model) handleSidebarKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    sidebarVisibleRows := m.height - 1 - 2 - searchBoxH // contentH (height-1) minus border (2) minus searchBoxH
    if sidebarVisibleRows < 1 {
        sidebarVisibleRows = 1
    }
    m.sidebar.visibleRows = sidebarVisibleRows

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
    case key.Matches(msg, keys.Enter), key.Matches(msg, keys.Open):
        if node := m.sidebar.Selected(); node != nil {
            sess := m.sidebar.FindSession(node.Session)
            if sess != nil && !sess.IsLive && sess.IsConfig && sess.Config != nil {
                if err := tmux.NewSession(sess.DisplayName, sess.Config.Path); err != nil {
                    m.statusMsg = "launch failed: " + err.Error()
                    m.statusExp = time.Now().Add(5 * time.Second)
                    m.sidebar.SetLaunchErr(err.Error())
                    return m, nil
                }
                m.sidebar.ClearLaunchErr()
                return m, m.fetchPanes()
            }
            target := m.sidebar.BestAlertTargetInSession(node.Session, m.cfg.Sidebar.SwitchFocus)
            if target == "" {
                target = node.Session
            }
            err := tmux.SwitchClient(target)
            if err != nil && target != node.Session {
                err = tmux.SwitchClient(node.Session)
            }
            if err == nil && m.popupMode {
                return m, tea.Quit
            }
        }
    case key.Matches(msg, keys.Esc):
        m.sidebar.MoveToSessionLevel()
    }
    // Populate proc list: session overview for all nodes.
    var cmd tea.Cmd
    if node := m.sidebar.Selected(); node != nil {
        m.procList.SetSessionData(m.panes, node.Session, m.procs, m.cwdMap, m.gitInfo, m.alertMap(), m.cfg)
        m.procGen++
        cmd = m.scheduleProcFetch()
    }
    m.updateDetailFromSelection()
    return m, cmd
}

func (m Model) handleProcListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    contentH := m.height - 1
    innerW := m.width - m.cfg.Sidebar.Width - 2
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
            if node := m.sidebar.Selected(); node != nil {
                m.procList.SetSessionData(m.panes, node.Session, m.procs, m.cwdMap, m.gitInfo, m.alertMap(), m.cfg)
            }
            m.procGen++
            m.procList.clampOffset(procH - 2)
            m.updateDetailFromSelection()
            return m, m.scheduleDelayedProcFetch()
        }
    case key.Matches(msg, keys.Expand):
        if m.procList.Expand() {
            if node := m.sidebar.Selected(); node != nil {
                m.procList.SetSessionData(m.panes, node.Session, m.procs, m.cwdMap, m.gitInfo, m.alertMap(), m.cfg)
            }
            m.procGen++
            m.procList.clampOffset(procH - 2)
            m.updateDetailFromSelection()
            return m, m.scheduleDelayedProcFetch()
        }
    case key.Matches(msg, keys.Collapse):
        if m.procList.Collapse() {
            if node := m.sidebar.Selected(); node != nil {
                m.procList.SetSessionData(m.panes, node.Session, m.procs, m.cwdMap, m.gitInfo, m.alertMap(), m.cfg)
            }
            m.procGen++
            m.procList.clampOffset(procH - 2)
            m.updateDetailFromSelection()
            return m, m.scheduleDelayedProcFetch()
        }
    case key.Matches(msg, keys.ExpandAll):
        if m.procList.ExpandAll() {
            if node := m.sidebar.Selected(); node != nil {
                m.procList.SetSessionData(m.panes, node.Session, m.procs, m.cwdMap, m.gitInfo, m.alertMap(), m.cfg)
            }
            m.procGen++
            m.procList.clampOffset(procH - 2)
            m.updateDetailFromSelection()
            return m, m.scheduleDelayedProcFetch()
        }
    case key.Matches(msg, keys.CollapseAll):
        if m.procList.CollapseAll() {
            if node := m.sidebar.Selected(); node != nil {
                m.procList.SetSessionData(m.panes, node.Session, m.procs, m.cwdMap, m.gitInfo, m.alertMap(), m.cfg)
            }
            m.procGen++
            m.procList.clampOffset(procH - 2)
            m.updateDetailFromSelection()
            return m, m.scheduleDelayedProcFetch()
        }
    case key.Matches(msg, keys.Open):
        var target string
        if pane := m.procList.SelectedPane(); pane != nil {
            target = fmt.Sprintf("%s:%d.%d", pane.Session, pane.WindowIndex, pane.PaneIndex)
        } else if node := m.sidebar.Selected(); node != nil {
            target = node.Session
        }
        if target != "" {
            tmux.SwitchClient(target)
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
        if selNode != nil && !selNode.IsPaneHeader && !selNode.IsWindowHeader {
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
    // session node from sidebar
    if node := m.sidebar.Selected(); node != nil {
        m.yank.SetFields([]YankField{
            {Key: "n", Label: "session", Value: node.Session},
            {Key: "t", Label: "target", Value: node.Session},
        })
    }
}

// applyNonAlertFocusMode applies a non-alert focus mode to the sidebar.
// Valid modes: current_session, first_session.
// No-ops on empty or unrecognised mode.
func (m *Model) applyNonAlertFocusMode(mode string, visibleRows int) {
    switch mode {
    case "current_session":
        m.sidebar.FocusNode(m.currentSession, visibleRows)
    case "first_session":
        // cursor is already 0, which is always the first session — no-op
    }
}

// pruneStaleAlerts removes non-sticky alerts whose pane/window/session target no
// longer appears in the current pane list. Returns a cmd that removes the stale
// entries from the DB and re-fetches the alert list, or nil if nothing to prune.
func (m *Model) pruneStaleAlerts() tea.Cmd {
    if len(m.panes) == 0 {
        return nil
    }
    // Build lookup sets for every live target granularity.
    paneTargets := make(map[string]bool, len(m.panes))
    winTargets := make(map[string]bool)
    sesTargets := make(map[string]bool)
    for _, p := range m.panes {
        paneTargets[fmt.Sprintf("%s:%d.%d", p.Session, p.WindowIndex, p.PaneIndex)] = true
        winTargets[fmt.Sprintf("%s:%d", p.Session, p.WindowIndex)] = true
        sesTargets[p.Session] = true
    }

    var stale []string
    for _, a := range m.alerts {
        switch {
        case strings.Contains(a.Target, "."):
            if !paneTargets[a.Target] {
                stale = append(stale, a.Target)
            }
        case strings.Contains(a.Target, ":"):
            if !winTargets[a.Target] {
                stale = append(stale, a.Target)
            }
        default:
            if !sesTargets[a.Target] {
                stale = append(stale, a.Target)
            }
        }
    }
    if len(stale) == 0 {
        return nil
    }
    d := m.db
    return func() tea.Msg {
        for _, t := range stale {
            d.AlertRemove(t)
        }
        alerts, _ := d.AlertList()
        return alertsMsg{alerts: alerts}
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
        sess := m.sidebar.FindSession(node.Session)
        isConfigOnly := sess != nil && !sess.IsLive && sess.IsConfig
        configPath := ""
        configWorktree := ""
        if isConfigOnly && sess.Config != nil {
            configPath = sess.Config.Path
            if sess.Config.Worktree && configPath != "" {
                configWorktree = filepath.Base(configPath) + " (" + filepath.Base(filepath.Dir(configPath)) + ")"
            }
        }
        m.detail = DetailModel{
            cfg:            m.cfg,
            selType:        DetailSession,
            session:        node.Session,
            sessionCWD:     sessionCWD,
            isConfigOnly:   isConfigOnly,
            configPath:     configPath,
            configWorktree: configWorktree,
            gitInfo:        m.gitInfo[node.Session],
            winCount:     len(windows),
            paneCount:    paneCount,
            procCount:    procCount,
            alertCount:   alertCount,
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
        if selNode.IsWindowHeader {
            sess := selNode.Pane.Session
            winIdx := selNode.Pane.WindowIndex
            grouped := tmux.GroupBySessions(m.panes)
            windows := grouped[sess]
            wPanes := windows[winIdx]
            gitKey := fmt.Sprintf("%s:%d", sess, winIdx)
            var windowAlert *db.Alert
            target := fmt.Sprintf("%s:%d", sess, winIdx)
            if a, err := m.db.AlertByTarget(target); err == nil && a != nil {
                windowAlert = a
            }
            sessionCWD := primaryCWDForPanes(windows)
            alertCount := 0
            for _, a := range m.alerts {
                if strings.HasPrefix(a.Target, sess+":") {
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
                session:     sess,
                sessionCWD:  sessionCWD,
                gitInfo:     m.gitInfo[sess],
                winCount:    len(windows),
                procCount:   procCount,
                alertCount:  alertCount,
                windowIndex: winIdx,
                windowPanes: wPanes,
                windowGit:   m.gitInfo[gitKey],
                windowAlert: windowAlert,
            }
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

    if m.cfg.Mode == "compact" {
        return m.compactView()
    }

    sidebarW := m.cfg.Sidebar.Width
    if sidebarW <= 0 {
        sidebarW = 30
    }
    procW := m.width - sidebarW
    if procW < 10 {
        procW = 10
    }

    // sidebar spans full content height; proclist + detail stack on the right
    contentH := m.height
    if m.cfg.StatusBar.Show {
        contentH-- // reserve 1 row for status bar
    }
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

    // build sidebar border title: [h] Sessions (N)
    sessionCount := m.sidebar.SessionCount()
    sessionCountStr := statValueStyle.Render(fmt.Sprintf("(%d)", sessionCount))
    filterMark := ""
    if f := m.sidebar.ActiveFilter(); f != FilterTmux {
        filterMark = " [" + string(f) + "]"
    }
    sidebarTitle := fmt.Sprintf(" [h] Sessions %s%s ", sessionCountStr, filterMark)

    // build proc list border title: [l] <session> / <window>
    bc := m.plainBreadcrumb()
    procTitleSuffix := " "
    if runes := []rune(bc); len(runes) > 0 && isIconRune(runes[len(runes)-1]) {
        procTitleSuffix = "  "
    }
    procTitle := " [l] " + bc + procTitleSuffix

    sidebarContent := m.sidebar.Render(sidebarW, contentH-searchBoxH, m.focus == panelSidebar, sidebarTitle, "")
    searchBox := m.searchInput.View(sidebarW)
    leftCol := lipgloss.JoinVertical(lipgloss.Left, searchBox, sidebarContent)

    procList := m.procList.Render(procW, procH, m.focus == panelProcList, procTitle)
    detail := m.detail.Render(procW, detailH)

    right := lipgloss.JoinVertical(lipgloss.Left, procList, detail)
    content := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, right)
    body := content

    var full string
    if m.cfg.StatusBar.Show {
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
        spinnerStr := ""
        if m.cfg.Git.ShowSpinner {
            for _, info := range m.gitInfo {
                if info.Loading {
                    frame := spinnerFrames[m.spinnerFrame%len(spinnerFrames)]
                    spinnerStr = " " + spinnerStyle.Render(frame) + " "
                    break
                }
            }
        }
        if spinnerStr != "" {
            leftWidth := m.width - lipgloss.Width(spinnerStr)
            statusBar = lipgloss.JoinHorizontal(lipgloss.Top,
                lipgloss.NewStyle().Width(leftWidth).MaxHeight(1).Render(statusBar),
                spinnerStr,
            )
        } else {
            statusBar = lipgloss.NewStyle().
                Width(m.width).
                MaxHeight(1).
                Render(statusBar)
        }
        full = lipgloss.JoinVertical(lipgloss.Left, body, statusBar)
    } else {
        full = body
    }

    if m.showHelp {
        return overlayCenter(m.help.Render(), full, m.width, m.height)
    }
    if m.showYank {
        return overlayCenter(m.yank.Render(), full, m.width, m.height)
    }

    return full
}

func (m Model) compactView() string {
    contentH := m.height
    if m.cfg.StatusBar.Show {
        contentH-- // reserve 1 row for status bar
    }

    sessionCount := m.sidebar.SessionCount()
    sessionCountStr := statValueStyle.Render(fmt.Sprintf("(%d)", sessionCount))
    filterMark := ""
    if f := m.sidebar.ActiveFilter(); f != FilterTmux {
        filterMark = " [" + string(f) + "]"
    }
    sidebarTitle := fmt.Sprintf(" [h] Sessions %s%s ", sessionCountStr, filterMark)

    // compact mode: sidebar is the only panel and always has focus
    sidebarContent := m.sidebar.Render(m.width, contentH-searchBoxH, true, sidebarTitle, "")
    searchBox := m.searchInput.View(m.width)
    leftCol := lipgloss.JoinVertical(lipgloss.Left, searchBox, sidebarContent)

    var full string
    if m.cfg.StatusBar.Show {
        statusBar := "  j/k:nav  Enter:open  !:alerts  ?:help  q:quit"
        if m.statusMsg != "" && time.Now().Before(m.statusExp) {
            statusBar = m.statusMsg
        }
        spinnerStr := ""
        if m.cfg.Git.ShowSpinner {
            for _, info := range m.gitInfo {
                if info.Loading {
                    frame := spinnerFrames[m.spinnerFrame%len(spinnerFrames)]
                    spinnerStr = " " + spinnerStyle.Render(frame) + " "
                    break
                }
            }
        }
        if spinnerStr != "" {
            leftWidth := m.width - lipgloss.Width(spinnerStr)
            statusBar = lipgloss.JoinHorizontal(lipgloss.Top,
                lipgloss.NewStyle().Width(leftWidth).MaxHeight(1).Render(statusBar),
                spinnerStr,
            )
        } else {
            statusBar = lipgloss.NewStyle().
                Width(m.width).
                MaxHeight(1).
                Render(statusBar)
        }
        full = lipgloss.JoinVertical(lipgloss.Left, leftCol, statusBar)
    } else {
        full = leftCol
    }

    if m.showHelp {
        return overlayCenter(m.help.Render(), full, m.width, m.height)
    }
    if m.showYank {
        return overlayCenter(m.yank.Render(), full, m.width, m.height)
    }
    return full
}

// overlayCenter places fg centered on top of bg (bgW×bgH visible columns).
// Uses ANSI-aware slicing so the background content remains visible.
func overlayCenter(fg, bg string, bgW, bgH int) string {
    fgLines := strings.Split(fg, "\n")
    bgLines := strings.Split(bg, "\n")

    fgH := len(fgLines)
    fgW := 0
    for _, l := range fgLines {
        if w := lipgloss.Width(l); w > fgW {
            fgW = w
        }
    }

    startY := (bgH - fgH) / 2
    startX := (bgW - fgW) / 2
    if startX < 0 {
        startX = 0
    }
    if startY < 0 {
        startY = 0
    }

    result := make([]string, len(bgLines))
    for i, bgLine := range bgLines {
        fgIdx := i - startY
        if fgIdx < 0 || fgIdx >= fgH {
            result[i] = bgLine
            continue
        }
        left := xansi.Truncate(bgLine, startX, "")
        right := xansi.TruncateLeft(bgLine, startX+fgW, "")
        result[i] = left + fgLines[fgIdx] + right
    }
    return strings.Join(result, "\n")
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
    return node.Session
}


func debounceSearch(gen int) tea.Cmd {
    return func() tea.Msg {
        time.Sleep(150 * time.Millisecond)
        return searchDebounceMsg{gen: gen}
    }
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
