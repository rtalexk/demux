package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rtalexk/demux/internal/db"
	demuxlog "github.com/rtalexk/demux/internal/log"
	"github.com/rtalexk/demux/internal/query"
	"github.com/rtalexk/demux/internal/tmux"
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
				m, cmd := m.openSidebarSelected()
				m.searchInput.ExitInsertMode()
				return m, cmd
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
		case "ctrl+o":
			m, cmd := m.openSidebarSelected()
			return m, cmd
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
	return m, nil
}

func (m Model) handleSidebarKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	sidebarVisibleRows := m.height - 1 - 2 - searchBoxH // contentH (height-1) minus border (2) minus searchBoxH
	if sidebarVisibleRows < 1 {
		sidebarVisibleRows = 1
	}
	m.sidebar.visibleRows = sidebarVisibleRows

	switch {
	case key.Matches(msg, keys.Up.Binding):
		m.sidebar.MoveUp(sidebarVisibleRows)
	case key.Matches(msg, keys.Down.Binding):
		m.sidebar.MoveDown(sidebarVisibleRows)
	case key.Matches(msg, keys.Tab.Binding):
		m.sidebar.TabNextSession(sidebarVisibleRows)
	case key.Matches(msg, keys.ShiftTab.Binding):
		m.sidebar.TabPrevSession(sidebarVisibleRows)
	case key.Matches(msg, keys.GotoTop.Binding):
		m.sidebar.GotoTop(sidebarVisibleRows)
	case key.Matches(msg, keys.GotoBottom.Binding):
		m.sidebar.GotoBottom(sidebarVisibleRows)
	case key.Matches(msg, keys.Enter.Binding), key.Matches(msg, keys.Open.Binding):
		if node := m.sidebar.Selected(); node != nil {
			sess := m.sidebar.FindSession(node.Session)
			if sess != nil && !sess.IsLive && sess.IsConfig && sess.Config != nil {
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
	case key.Matches(msg, keys.Esc.Binding):
		m.sidebar.MoveToSessionLevel()
	case key.Matches(msg, keys.Defer.Binding):
		if node := m.sidebar.Selected(); node != nil {
			return m, m.toggleDeferAlert(node.Session)
		}
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
	case key.Matches(msg, keys.Enter.Binding):
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
	case key.Matches(msg, keys.Expand.Binding):
		if m.procList.Expand() {
			if node := m.sidebar.Selected(); node != nil {
				m.procList.SetSessionData(m.panes, node.Session, m.procs, m.cwdMap, m.gitInfo, m.alertMap(), m.cfg)
			}
			m.procGen++
			m.procList.clampOffset(procH - 2)
			m.updateDetailFromSelection()
			return m, m.scheduleDelayedProcFetch()
		}
	case key.Matches(msg, keys.Collapse.Binding):
		if m.procList.Collapse() {
			if node := m.sidebar.Selected(); node != nil {
				m.procList.SetSessionData(m.panes, node.Session, m.procs, m.cwdMap, m.gitInfo, m.alertMap(), m.cfg)
			}
			m.procGen++
			m.procList.clampOffset(procH - 2)
			m.updateDetailFromSelection()
			return m, m.scheduleDelayedProcFetch()
		}
	case key.Matches(msg, keys.ExpandAll.Binding):
		if m.procList.ExpandAll() {
			if node := m.sidebar.Selected(); node != nil {
				m.procList.SetSessionData(m.panes, node.Session, m.procs, m.cwdMap, m.gitInfo, m.alertMap(), m.cfg)
			}
			m.procGen++
			m.procList.clampOffset(procH - 2)
			m.updateDetailFromSelection()
			return m, m.scheduleDelayedProcFetch()
		}
	case key.Matches(msg, keys.CollapseAll.Binding):
		if m.procList.CollapseAll() {
			if node := m.sidebar.Selected(); node != nil {
				m.procList.SetSessionData(m.panes, node.Session, m.procs, m.cwdMap, m.gitInfo, m.alertMap(), m.cfg)
			}
			m.procGen++
			m.procList.clampOffset(procH - 2)
			m.updateDetailFromSelection()
			return m, m.scheduleDelayedProcFetch()
		}
	case key.Matches(msg, keys.Open.Binding):
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

func (m Model) toggleDeferAlert(target string) tea.Cmd {
	var hasDefer bool
	for _, a := range m.alerts {
		if a.Target == target && a.Level == db.LevelDefer {
			hasDefer = true
			break
		}
	}
	reason := m.cfg.Alerts.DeferDefaultReason
	d := m.db
	return func() tea.Msg {
		if hasDefer {
			if err := d.AlertRemove(target); err != nil {
				demuxlog.Warn("defer remove failed", "err", err)
			}
		} else {
			if err := d.AlertSet(target, reason, db.LevelDefer); err != nil {
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

func (m Model) openSidebarSelected() (Model, tea.Cmd) {
	node := m.sidebar.Selected()
	if node == nil {
		return m, nil
	}
	sess := m.sidebar.FindSession(node.Session)
	if sess != nil && !sess.IsLive && sess.IsConfig && sess.Config != nil {
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
	if err := tmux.SwitchClient(node.Session); err == nil {
		if m.popupMode {
			return m, tea.Quit
		}
	}
	return m, nil
}
