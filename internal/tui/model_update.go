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
    "github.com/rtalexk/demux/internal/db"
    "github.com/rtalexk/demux/internal/git"
    demuxlog "github.com/rtalexk/demux/internal/log"
    "github.com/rtalexk/demux/internal/proc"
    "github.com/rtalexk/demux/internal/query"
    "github.com/rtalexk/demux/internal/session"
    "github.com/rtalexk/demux/internal/tmux"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    if overlay, cmd, handled := m.handleOverlays(msg); handled {
        return overlay, cmd
    }
    return m.handleMsg(msg)
}

func (m Model) handleMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        return m.handleKey(msg)
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        return m, nil
    case tickMsg:
        return m.handleTickMsg(msg)
    case panesMsg:
        return m.handlePanesMsg(msg)
    case procDataMsg:
        return m.handleProcDataMsg(msg)
    case alertsMsg:
        return m.handleAlertsMsg(msg)
    case gitResultMsg:
        return m.handleGitResultMsg(msg)
    case queryResultMsg:
        return m.handleQueryResultMsg(msg)
    case searchDebounceMsg:
        return m.handleSearchDebounceMsg(msg)
    }
    return m, nil
}

// handleOverlays routes messages to full-screen overlay handlers.
// Returns (model, cmd, true) if the message was consumed by an overlay.
func (m Model) handleOverlays(msg tea.Msg) (Model, tea.Cmd, bool) {
    if m.showHelp {
        mo, cmd := m.handleHelpOverlay(msg)
        return mo, cmd, true
    }
    if m.showYank {
        mo, cmd := m.updateYank(msg)
        return mo.(Model), cmd, true
    }
    return m, nil, false
}

func (m Model) handleGitResultMsg(msg gitResultMsg) (Model, tea.Cmd) {
    m.gitInfo[msg.key] = msg.info
    merged := session.Merge(m.panes, m.sessionsConfig.Entries)
    m.sidebar.SetData(merged, m.alerts, m.gitInfo, m.cfg)
    m.updateDetailFromSelection()
    return m, nil
}

func (m Model) handleHelpOverlay(msg tea.Msg) (Model, tea.Cmd) {
    if keyMsg, ok := msg.(tea.KeyMsg); ok {
        switch {
        case key.Matches(keyMsg, keys.Esc.Binding), key.Matches(keyMsg, keys.Help.Binding), keyMsg.String() == "q":
            m.showHelp = false
        case key.Matches(keyMsg, keys.Up.Binding):
            m.help.ScrollUp()
        case key.Matches(keyMsg, keys.Down.Binding):
            m.help.ScrollDown(m.height)
        }
    }
    return m, nil
}

func (m Model) handleTickMsg(_ tickMsg) (Model, tea.Cmd) {
    m.pulse = !m.pulse
    m.spinnerFrame++
    if time.Now().After(m.statusExp) {
        m.statusMsg = ""
    }
    return m, tea.Batch(tick(time.Duration(m.cfg.RefreshIntervalMs)*time.Millisecond), m.fetchPanes(), m.fetchAlerts())
}

func (m Model) handleQueryResultMsg(msg queryResultMsg) (Model, tea.Cmd) {
    if msg.gen != m.searchGen {
        return m, nil
    }
    m.queryResult = msg.result
    m.sidebar.SetSearchResult(msg.result)
    m.procList.SetSearchQuery(query.Parse(m.searchInput.Value()), msg.result)
    if node := m.sidebar.Selected(); node != nil {
        m.procList.SetSessionData(m.panes, node.Session, m.procs, m.cwdMap, m.gitInfo, m.alertMap(), m.cfg)
        m.procGen++
        m.updateDetailFromSelection()
        return m, m.scheduleProcFetch()
    }
    m.procList.Reset()
    return m, nil
}

func (m Model) handlePanesMsg(msg panesMsg) (Model, tea.Cmd) {
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
        cmds = append(cmds, tick(time.Duration(m.cfg.RefreshIntervalMs)*time.Millisecond), m.fetchAlerts())
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
            primaryCWD := tmux.PrimaryPaneCWD(windows[0])
            if primaryCWD != "" {
                cmds = append(cmds, fetchGit(sessionName, primaryCWD, m.cfg.Git.TimeoutMs))
            }
        }
    }
    return m, tea.Batch(cmds...)
}

func (m Model) handleProcDataMsg(msg procDataMsg) (Model, tea.Cmd) {
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
}

func (m Model) handleAlertsMsg(msg alertsMsg) (Model, tea.Cmd) {
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
}

func (m Model) handleSearchDebounceMsg(msg searchDebounceMsg) (Model, tea.Cmd) {
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
    paneTargets, winTargets, sesTargets := buildTargetSets(m.panes)
    var stale []string
    for _, a := range m.alerts {
        if a.Sticky {
            continue
        }
        if isStaleAlert(a.Target, paneTargets, winTargets, sesTargets) {
            stale = append(stale, a.Target)
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
        alerts, err := d.AlertList()
        if err != nil {
            demuxlog.Warn("fetch alerts failed", "err", err)
        }
        return alertsMsg{alerts: alerts}
    }
}

// buildTargetSets constructs lookup maps for every live pane, window and session target.
func buildTargetSets(panes []tmux.Pane) (paneTargets, winTargets, sesTargets map[string]bool) {
    paneTargets = make(map[string]bool, len(panes))
    winTargets = make(map[string]bool)
    sesTargets = make(map[string]bool)
    for _, p := range panes {
        paneTargets[fmt.Sprintf("%s:%d.%d", p.Session, p.WindowIndex, p.PaneIndex)] = true
        winTargets[fmt.Sprintf("%s:%d", p.Session, p.WindowIndex)] = true
        sesTargets[p.Session] = true
    }
    return paneTargets, winTargets, sesTargets
}

// isStaleAlert reports whether the given alert target is absent from the live target sets.
func isStaleAlert(target string, paneTargets, winTargets, sesTargets map[string]bool) bool {
    switch {
    case strings.Contains(target, "."):
        return !paneTargets[target]
    case strings.Contains(target, ":"):
        return !winTargets[target]
    default:
        return !sesTargets[target]
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
        m.detail = m.detailForSidebarNode(*node)
        return
    }
    if m.focus == panelProcList {
        selNode := m.procList.SelectedNode()
        if selNode == nil || selNode.IsPaneHeader {
            m.detail = DetailModel{}
            return
        }
        if selNode.IsWindowHeader {
            m.detail = m.detailForWindowNode(*selNode)
            return
        }
        m.detail = m.detailForProcNode(*selNode)
    }
}

func (m *Model) detailForSidebarNode(node SidebarNode) DetailModel {
    grouped := tmux.GroupBySessions(m.panes)
    windows := grouped[node.Session]
    sessionCWD := tmux.PrimaryPaneCWD(windows[0])
    paneCount := 0
    for _, wp := range windows {
        paneCount += len(wp)
    }
    sess := m.sidebar.FindSession(node.Session)
    isConfigOnly := sess != nil && !sess.IsLive && sess.IsConfig
    configPath, configWorktree := configOnlyFields(sess, isConfigOnly)
    return DetailModel{
        cfg:            m.cfg,
        selType:        DetailSession,
        session:        node.Session,
        sessionCWD:     sessionCWD,
        isConfigOnly:   isConfigOnly,
        configPath:     configPath,
        configWorktree: configWorktree,
        gitInfo:        m.gitInfo[node.Session],
        winCount:       len(windows),
        paneCount:      paneCount,
        procCount:      countProcsUnderCWD(m.procs, m.cwdMap, sessionCWD),
        alertCount:     countSessionAlerts(m.alerts, node.Session),
    }
}

// countSessionAlerts returns the number of alerts whose target is within sessionName.
func countSessionAlerts(alerts []db.Alert, sessionName string) int {
    count := 0
    prefix := sessionName + ":"
    for _, a := range alerts {
        if strings.HasPrefix(a.Target, prefix) {
            count++
        }
    }
    return count
}

// countProcsUnderCWD counts processes whose working directory is sessionCWD or a descendant.
func countProcsUnderCWD(procs []proc.Process, cwdMap map[int32]string, sessionCWD string) int {
    if sessionCWD == "" {
        return 0
    }
    count := 0
    for _, pr := range procs {
        cwd := cwdMap[pr.PID]
        if cwd == "" {
            continue
        }
        if cwd == sessionCWD || git.IsDescendant(cwd, sessionCWD) {
            count++
        }
    }
    return count
}

// configOnlyFields extracts the configPath and configWorktree display string for a
// config-only session. Returns empty strings when isConfigOnly is false or the
// session has no Config.
func configOnlyFields(sess *session.Session, isConfigOnly bool) (configPath, configWorktree string) {
    if !isConfigOnly || sess == nil || sess.Config == nil {
        return "", ""
    }
    configPath = sess.Config.Path
    if !sess.Config.Worktree || configPath == "" {
        return configPath, ""
    }
    // If configPath itself is the worktree root container (.bare/ lives here),
    // show just the repo name. Otherwise show "worktree (repo)".
    if fi, err := os.Stat(filepath.Join(configPath, ".bare")); err == nil && fi.IsDir() {
        bareStr := lipgloss.NewStyle().Italic(true).Render("_bare_")
        configWorktree = bareStr + " (" + filepath.Base(configPath) + ")"
    } else {
        configWorktree = filepath.Base(configPath) + " (" + filepath.Base(filepath.Dir(configPath)) + ")"
    }
    return configPath, configWorktree
}

func (m *Model) detailForWindowNode(node ProcListNode) DetailModel {
    sess := node.Pane.Session
    winIdx := node.Pane.WindowIndex
    grouped := tmux.GroupBySessions(m.panes)
    windows := grouped[sess]
    wPanes := windows[winIdx]
    gitKey := fmt.Sprintf("%s:%d", sess, winIdx)
    var windowAlert *db.Alert
    target := fmt.Sprintf("%s:%d", sess, winIdx)
    if a, err := m.db.AlertByTarget(target); err == nil && a != nil {
        windowAlert = a
    }
    sessionCWD := tmux.PrimaryPaneCWD(windows[0])
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
    return DetailModel{
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
}

func (m *Model) detailForProcNode(node ProcListNode) DetailModel {
    pr := node.Proc
    cwd := m.cwdMap[pr.PID]
    portStr := ""
    if node.Port > 0 {
        portStr = fmt.Sprintf("%d", node.Port)
    }
    return DetailModel{
        cfg:      m.cfg,
        selType:  DetailProc,
        proc:     pr,
        procGit:  m.gitInfo[cwd],
        procPort: portStr,
        procCWD:  cwd,
    }
}

func (m Model) updateYank(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if key.Matches(msg, keys.Esc.Binding) || msg.String() == "q" {
            m.showYank = false
            return m, nil
        }
        if key.Matches(msg, keys.Enter.Binding) {
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
