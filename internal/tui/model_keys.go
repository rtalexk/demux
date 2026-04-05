package tui

import (
    "fmt"
    "time"

    "github.com/charmbracelet/bubbles/key"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/rtalexk/demux/internal/db"
    demuxlog "github.com/rtalexk/demux/internal/log"
    "github.com/rtalexk/demux/internal/query"
    "github.com/rtalexk/demux/internal/session"
    "github.com/rtalexk/demux/internal/tmux"
)

const (
    dirDown = 1
    dirUp   = -1
)

// resolveFilterKey maps a key message to a SidebarFilter.
// Returns (filter, true) if the key matches a filter binding, ("", false) otherwise.
func resolveFilterKey(msg tea.KeyMsg) (SidebarFilter, bool) {
    switch {
    case key.Matches(msg, keys.AlertFilter.Binding):
        return FilterPriority, true
    case key.Matches(msg, keys.FilterTmux.Binding):
        return FilterTmux, true
    case key.Matches(msg, keys.FilterAll.Binding):
        return FilterAll, true
    case key.Matches(msg, keys.FilterConfig.Binding):
        return FilterConfig, true
    case key.Matches(msg, keys.FilterWorktree.Binding):
        return FilterWorktree, true
    }
    return "", false
}

// handleNormalModeDefault handles the default case in handleKey (non-insert, non-special keys).
func (m Model) handleNormalModeDefault(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
    if newFilter, ok := resolveFilterKey(msg); ok {
        sidebarVisibleRows := m.height - 1 - 2 - searchBoxH
        if sidebarVisibleRows < 1 {
            sidebarVisibleRows = 1
        }
        m.sidebar.SetFilter(newFilter, sidebarVisibleRows)
        if node := m.sidebar.Selected(); node != nil {
            m.procList.SetSessionData(m.panes, node.Session, m.procs, m.cwdMap, m.gitInfo, m.alertMap(), m.cfg)
            m.updateDetailFromSelection()
        }
        return m, nil
    }
    if m.focus == panelSidebar {
        return m.handleSidebarKey(msg)
    }
    return m.handleProcListKey(msg)
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    if m.searchInput.IsInsert() {
        return m.handleSearchInsertKey(msg)
    }

    switch {
    case key.Matches(msg, keys.Quit.Binding):
        return m, tea.Quit
    case key.Matches(msg, keys.FocusSidebar.Binding):
        m.focus = panelSidebar
        m.updateDetailFromSelection()
    case key.Matches(msg, keys.FocusProcList.Binding):
        // compact mode: FocusProcList is a no-op; panelSidebar is the only panel
        if m.cfg.Mode != "compact" {
            m.focus = panelProcList
        }
        // updateDetailFromSelection gates on m.focus, so safe to call unconditionally
        m.updateDetailFromSelection()
    case key.Matches(msg, keys.Help.Binding):
        m.showHelp = !m.showHelp
    case key.Matches(msg, keys.Yank.Binding):
        m.populateYankFields()
        m.showYank = true
    case key.Matches(msg, keys.Refresh.Binding):
        m.procGen++
        return m, tea.Batch(m.fetchPanes(), m.fetchAlerts(), m.scheduleProcFetch())
    default:
        return m.handleNormalModeDefault(msg)
    }
    return m, nil
}

// navigateSidebarInSearch moves the sidebar cursor up (direction=-1) or down (direction=1)
// while the search input is in insert mode, then updates proc list and detail.
func (m Model) navigateSidebarInSearch(direction int) (Model, tea.Cmd) {
    if direction > 0 {
        m.sidebar.CursorDown()
    } else {
        m.sidebar.CursorUp()
    }
    if node := m.sidebar.Selected(); node != nil {
        m.procList.SetSessionData(m.panes, node.Session, m.procs, m.cwdMap, m.gitInfo, m.alertMap(), m.cfg)
        m.procGen++
        m.updateDetailFromSelection()
        return m, m.scheduleProcFetch()
    }
    return m, nil
}

// handleSearchInsertKey handles key events when the search input is in insert mode.
// Returns the updated model and command.
func (m Model) handleSearchInsertKey(msg tea.KeyMsg) (Model, tea.Cmd) {
    sidebarVisibleRows := m.height - 1 - 2 - searchBoxH
    if sidebarVisibleRows < 1 {
        sidebarVisibleRows = 1
    }
    m.sidebar.visibleRows = sidebarVisibleRows
    switch msg.String() {
    case "esc", "enter":
        if msg.String() == "enter" {
            m, cmd := m.openSidebarSelected()
            m.searchInput.ExitInsertMode()
            return m, cmd
        }
        m.searchInput.ExitInsertMode()
        return m, nil
    case "ctrl+j", "ctrl+n":
        return m.navigateSidebarInSearch(dirDown)
    case "ctrl+k", "ctrl+p":
        return m.navigateSidebarInSearch(dirUp)
    case "ctrl+o":
        return m.openSidebarSelected()
    default:
        return m.handleSearchInputUpdate(msg)
    }
}

// handleSearchInputUpdate processes a raw key event against the search input field.
func (m Model) handleSearchInputUpdate(msg tea.KeyMsg) (Model, tea.Cmd) {
    var cmd tea.Cmd
    prevVal := m.searchInput.Value()
    m.searchInput, cmd = m.searchInput.Update(msg)
    if m.searchInput.Value() == prevVal {
        return m, cmd
    }
    m.searchGen++
    if m.searchInput.Value() == "" {
        m.queryResult = query.Result{}
        m.sidebar.SetSearchResult(query.Result{})
        m.procList.SetSearchQuery(query.ParsedQuery{}, query.Result{})
        return m, cmd
    }
    return m, tea.Batch(cmd, debounceSearch(m.searchGen))
}

// launchConfigSession creates a new tmux session from a config-only session entry.
// Returns the updated model and a command (nil on launch failure, fetchPanes or tea.Quit on success).
func (m Model) launchConfigSession(sess *session.Session) (Model, tea.Cmd) {
    if err := tmux.NewSession(sess.DisplayName, sess.Config.Path); err != nil {
        m.statusMsg = "launch failed: " + err.Error()
        m.statusExp = time.Now().Add(5 * time.Second)
        m.sidebar.SetLaunchErr(err.Error())
        return m, nil
    }
    if specs := resolveWindowSpecs(sess.Config.Windows, m.sessionsConfig.WindowTemplates); len(specs) > 0 {
        if err := tmux.CreateSessionWindows(sess.DisplayName, sess.Config.Path, specs); err != nil {
            m.statusMsg = "window setup failed: " + err.Error()
            m.statusExp = time.Now().Add(5 * time.Second)
        }
    }
    m.sidebar.ClearLaunchErr()
    if m.popupMode {
        return m, tea.Quit
    }
    return m, m.fetchPanes()
}

// switchLiveSession switches the tmux client to the best target within sessionName.
func (m Model) switchLiveSession(sessionName string) (Model, tea.Cmd) {
    target := m.sidebar.BestAlertTargetInSession(sessionName, m.cfg.Sidebar.SwitchFocus)
    if target == "" {
        target = sessionName
    }
    err := tmux.SwitchClient(target)
    if err != nil && target != sessionName {
        err = tmux.SwitchClient(sessionName)
    }
    if err == nil && m.popupMode {
        return m, tea.Quit
    }
    return m, nil
}

// launchOrSwitchSession launches a config-only session or switches to a live session.
// Used by both handleSidebarKey and openSidebarSelected.
func (m Model) launchOrSwitchSession(sess *session.Session, sessionName string) (Model, tea.Cmd) {
    if sess != nil && !sess.IsLive && sess.IsConfig && sess.Config != nil {
        return m.launchConfigSession(sess)
    }
    return m.switchLiveSession(sessionName)
}

// sidebarNavUpdate runs after most sidebar key actions to refresh proc list and detail.
func (m Model) sidebarNavUpdate() (Model, tea.Cmd) {
    var cmd tea.Cmd
    if node := m.sidebar.Selected(); node != nil {
        m.procList.SetSessionData(m.panes, node.Session, m.procs, m.cwdMap, m.gitInfo, m.alertMap(), m.cfg)
        m.procGen++
        cmd = m.scheduleProcFetch()
    }
    m.updateDetailFromSelection()
    return m, cmd
}

// sidebarCursorMove handles cursor-movement keys in the sidebar.
// Returns the updated model and true if the key was consumed.
func (m Model) sidebarCursorMove(msg tea.KeyMsg, rows int) (Model, bool) {
    switch {
    case key.Matches(msg, keys.Up.Binding):
        m.sidebar.MoveUp(rows)
    case key.Matches(msg, keys.Down.Binding):
        m.sidebar.MoveDown(rows)
    case key.Matches(msg, keys.Tab.Binding):
        m.sidebar.TabNextSession(rows)
    case key.Matches(msg, keys.ShiftTab.Binding):
        m.sidebar.TabPrevSession(rows)
    case key.Matches(msg, keys.GotoTop.Binding):
        m.sidebar.GotoTop(rows)
    case key.Matches(msg, keys.GotoBottom.Binding):
        m.sidebar.GotoBottom(rows)
    default:
        return m, false
    }
    return m, true
}

func (m Model) handleSidebarKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    sidebarVisibleRows := m.height - 1 - 2 - searchBoxH // contentH (height-1) minus border (2) minus searchBoxH
    if sidebarVisibleRows < 1 {
        sidebarVisibleRows = 1
    }
    m.sidebar.visibleRows = sidebarVisibleRows

    if nm, ok := m.sidebarCursorMove(msg, sidebarVisibleRows); ok {
        return nm.sidebarNavUpdate()
    }
    switch {
    case key.Matches(msg, keys.Enter.Binding), key.Matches(msg, keys.Open.Binding):
        if node := m.sidebar.Selected(); node != nil {
            sess := m.sidebar.FindSession(node.Session)
            return m.launchOrSwitchSession(sess, node.Session)
        }
    case key.Matches(msg, keys.Esc.Binding):
        m.sidebar.MoveToSessionLevel()
    case key.Matches(msg, keys.Defer.Binding):
        if node := m.sidebar.Selected(); node != nil {
            return m, m.toggleDeferAlert(node.Session)
        }
    case key.Matches(msg, keys.DeferSticky.Binding):
        if node := m.sidebar.Selected(); node != nil {
            return m, m.toggleStickyDeferAlert(node.Session)
        }
    }
    return m.sidebarNavUpdate()
}

// procListDimensions computes procH and detailH for the current model state.
func (m Model) procListDimensions() (procH, detailH int) {
    contentH := m.height - 1
    innerW := m.width - m.cfg.Sidebar.Width - 2
    if innerW < 1 {
        innerW = 1
    }
    detailContent := m.detail.ContentLines(innerW)
    detailH = detailContent + 2
    if detailH < 4 {
        detailH = 4
    }
    maxDetailH := contentH - 4
    if detailH > maxDetailH {
        detailH = maxDetailH
    }
    procH = contentH - detailH
    return procH, detailH
}

// afterCollapse performs the shared post-toggle update after an expand/collapse operation:
// rebuilds proc data for the current selection, clamps viewport, updates detail,
// and schedules a delayed proc fetch.
func (m Model) afterCollapse(procH int) (Model, tea.Cmd) {
    if node := m.sidebar.Selected(); node != nil {
        m.procList.SetSessionData(m.panes, node.Session, m.procs, m.cwdMap, m.gitInfo, m.alertMap(), m.cfg)
    }
    m.procGen++
    m.procList.clampOffset(procH - 2)
    m.updateDetailFromSelection()
    return m, m.scheduleDelayedProcFetch()
}

// handleProcListNav handles navigation key cases in the proc list.
// Returns (model, cmd, true) if the key was handled, or (m, nil, false) if not.
func (m Model) handleProcListNav(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
    switch {
    case key.Matches(msg, keys.Up.Binding):
        m.procList.MoveUp()
    case key.Matches(msg, keys.Down.Binding):
        m.procList.MoveDown()
    case key.Matches(msg, keys.Tab.Binding):
        m.procList.TabNext()
    case key.Matches(msg, keys.JumpUp.Binding):
        m.procList.JumpToPrevPane()
    case key.Matches(msg, keys.JumpDown.Binding):
        m.procList.JumpToNextPane()
    case key.Matches(msg, keys.GotoTop.Binding):
        m.procList.GotoTop()
    case key.Matches(msg, keys.GotoBottom.Binding):
        m.procList.GotoBottom()
    default:
        return m, nil, false
    }
    return m, nil, true
}

// collapseToggleFn returns the procList toggle function matching the key, or nil if not matched.
func (m Model) collapseToggleFn(msg tea.KeyMsg) func() bool {
    switch {
    case key.Matches(msg, keys.Enter.Binding):
        return m.procList.ToggleCollapse
    case key.Matches(msg, keys.Expand.Binding):
        return m.procList.Expand
    case key.Matches(msg, keys.Collapse.Binding):
        return m.procList.Collapse
    case key.Matches(msg, keys.ExpandAll.Binding):
        return m.procList.ExpandAll
    case key.Matches(msg, keys.CollapseAll.Binding):
        return m.procList.CollapseAll
    }
    return nil
}

// handleProcListCollapse handles collapse/expand key cases in the proc list.
// Returns (model, cmd, true) if the key caused a layout change, or (m, nil, false) if not.
func (m Model) handleProcListCollapse(msg tea.KeyMsg, procH int) (Model, tea.Cmd, bool) {
    fn := m.collapseToggleFn(msg)
    if fn != nil && fn() {
        nm, cmd := m.afterCollapse(procH)
        return nm, cmd, true
    }
    return m, nil, false
}

// handleProcListOpen handles the Open key in the proc list.
func (m Model) handleProcListOpen() (Model, tea.Cmd) {
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
    return m, nil
}

func (m Model) handleProcListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    procH, _ := m.procListDimensions()

    if nm, cmd, ok := m.handleProcListNav(msg); ok {
        nm.procList.clampOffset(procH - 2)
        nm.updateDetailFromSelection()
        return nm, cmd
    }
    if nm, cmd, ok := m.handleProcListCollapse(msg, procH); ok {
        return nm, cmd
    }

    switch {
    case key.Matches(msg, keys.Open.Binding):
        nm, cmd := m.handleProcListOpen()
        nm.procList.clampOffset(procH - 2)
        nm.updateDetailFromSelection()
        return nm, cmd
    case key.Matches(msg, keys.Esc.Binding):
        m.focus = panelSidebar
    case key.Matches(msg, keys.Kill.Binding):
        // TODO: confirmation prompt
    case key.Matches(msg, keys.Restart.Binding):
        // TODO: restart via tmux send-keys Up Enter
    case key.Matches(msg, keys.Log.Binding):
        // TODO: tmux popup with scrollback
    }
    m.procList.clampOffset(procH - 2) // procH includes border; pass inner content height
    m.updateDetailFromSelection()
    return m, nil
}

// deferAlertState summarizes a target's current alert for defer-toggle decisions.
type deferAlertState struct {
    hasDefer bool // target has a defer-level alert
    isSticky bool // that defer alert is sticky
    blocked  bool // target has a higher-severity (non-defer) alert
}

// deferStateFor scans m.alerts once and returns the defer state for target.
func (m Model) deferStateFor(target string) deferAlertState {
    var s deferAlertState
    for _, a := range m.alerts {
        if a.Target != target {
            continue
        }
        if a.Level == db.LevelDefer {
            s.hasDefer = true
            s.isSticky = a.Sticky
        } else {
            s.blocked = true
        }
    }
    return s
}

func (m Model) toggleDeferAlert(target string) tea.Cmd {
    s := m.deferStateFor(target)
    if s.isSticky || s.blocked {
        return nil
    }
    reason := m.cfg.Alerts.DeferDefaultReason
    d := m.db
    return func() tea.Msg {
        if s.hasDefer {
            if err := d.AlertRemove(target); err != nil {
                demuxlog.Warn("defer remove failed", "err", err)
            }
        } else {
            if err := d.AlertSet(target, reason, db.LevelDefer, false); err != nil {
                demuxlog.Warn("defer set failed", "err", err)
            }
        }
        alerts, err := d.AlertList()
        if err != nil {
            demuxlog.Warn("fetch alerts after defer toggle failed", "err", err)
        }
        return alertsMsg{alerts: alerts}
    }
}

func (m Model) toggleStickyDeferAlert(target string) tea.Cmd {
    s := m.deferStateFor(target)
    if s.blocked {
        return nil
    }
    reason := m.cfg.Alerts.DeferDefaultReason
    d := m.db
    return func() tea.Msg {
        var opErr error
        switch {
        case s.isSticky:
            // Toggle off: remove sticky defer
            opErr = d.AlertRemove(target)
        case s.hasDefer:
            // Upgrade existing non-sticky defer to sticky
            opErr = d.AlertUpgradeToSticky(target)
        default:
            // Create new sticky defer
            opErr = d.AlertSet(target, reason, db.LevelDefer, true)
        }
        if opErr != nil {
            demuxlog.Warn("sticky defer toggle failed", "err", opErr)
        }
        alerts, err := d.AlertList()
        if err != nil {
            demuxlog.Warn("fetch alerts after sticky defer toggle failed", "err", err)
        }
        return alertsMsg{alerts: alerts}
    }
}

func (m Model) openSidebarSelected() (Model, tea.Cmd) {
    node := m.sidebar.Selected()
    if node == nil {
        return m, nil
    }
    sess := m.sidebar.FindSession(node.Session)
    return m.launchOrSwitchSession(sess, node.Session)
}
