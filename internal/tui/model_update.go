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
	case statesMsg:
		return m.handleStatesMsg(msg)
	case watchesMsg:
		return m.handleWatchesMsg(msg)
	case itemSessionsMsg:
		return m.handleItemSessionsMsg(msg)
	case itemsMsg:
		return m.handleItemsMsg(msg)
	case itemEditorDoneMsg:
		return m.handleItemEditorDoneMsg(msg)
	case itemDeleteConfirmedMsg:
		return m.handleItemDeleteConfirmed(msg)
	case gitResultMsg:
		return m.handleGitResultMsg(msg)
	case queryResultMsg:
		return m.handleQueryResultMsg(msg)
	case searchDebounceMsg:
		return m.handleSearchDebounceMsg(msg)
	}
	return m, nil
}

// handleOverlays routes key messages to full-screen overlay handlers.
// Returns (model, cmd, true) if the message was consumed by an overlay.
// Non-key messages fall through so background updates (ticks, state fetches)
// continue to run while an overlay is visible.
func (m Model) handleOverlays(msg tea.Msg) (Model, tea.Cmd, bool) {
	if _, isKey := msg.(tea.KeyMsg); !isKey {
		return m, nil, false
	}
	if m.showHelp {
		mo, cmd := m.handleHelpOverlay(msg)
		return mo, cmd, true
	}
	if m.showYank {
		mo, cmd := m.updateYank(msg)
		return mo.(Model), cmd, true
	}
	if m.showConfirm {
		mo, cmd := m.handleConfirmOverlay(msg)
		return mo, cmd, true
	}
	return m, nil, false
}

func (m Model) handleConfirmOverlay(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "y", "enter":
		cmd := m.confirmCmd
		m.showConfirm = false
		m.confirmCmd = nil
		return m, cmd
	case "n", "esc", "q":
		m.showConfirm = false
		m.confirmCmd = nil
	}
	return m, nil
}

func (m Model) handleGitResultMsg(msg gitResultMsg) (Model, tea.Cmd) {
	m.gitInfo[msg.key] = msg.info
	merged := session.Merge(m.panes, m.sessionsConfig.Entries)
	m.sidebar.SetData(merged, m.states, m.gitInfo, m.cfg)
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
	return m, tea.Batch(tick(time.Duration(m.cfg.RefreshIntervalMs)*time.Millisecond), m.fetchPanes(), m.fetchStates(), m.fetchWatches(), m.fetchItemSessions())
}

func (m Model) handleQueryResultMsg(msg queryResultMsg) (Model, tea.Cmd) {
	if msg.gen != m.searchGen {
		return m, nil
	}
	m.queryResult = msg.result
	m.sidebar.SetSearchResult(msg.result)
	m.procList.SetSearchQuery(query.Parse(m.searchInput.Value()), msg.result)
	if node := m.sidebar.Selected(); node != nil {
		m.procList.SetSessionData(m.panes, node.Session, m.procs, m.cwdMap, m.gitInfo, m.cfg)
		m.procGen++
		m.updateDetailFromSelection()
		return m, m.scheduleProcFetch()
	}
	m.procList.Reset()
	return m, nil
}

// resolveStateTargets returns a copy of states with each state's Target replaced
// by the current "session:window.pane" from the live pane list when a pane_id
// match is found. Unmatched or legacy states keep their stored Target.
func resolveStateTargets(states []db.ToolState, panes []tmux.Pane) []db.ToolState {
	paneIDMap := tmux.PaneIDToTargetMap(panes)
	out := make([]db.ToolState, len(states))
	for i, st := range states {
		if st.PaneID != "" {
			if resolved, ok := paneIDMap[st.PaneID]; ok {
				st.Target = resolved
			}
		}
		out[i] = st
	}
	return out
}

func (m Model) handlePanesMsg(msg panesMsg) (Model, tea.Cmd) {
	m.panes = msg.panes
	m.states = resolveStateTargets(m.states, msg.panes) // re-resolve with fresh pane data
	grouped := tmux.GroupBySessions(msg.panes)
	merged := session.Merge(msg.panes, m.sessionsConfig.Entries)
	m.sidebar.SetData(merged, m.states, m.gitInfo, m.cfg)
	m.sidebar.SetActiveSession(msg.currentSession)
	m.updateDetailFromSelection()
	var cmds []tea.Cmd
	if !m.ready {
		// First load: sidebar is visible — kick off tick and states; procs are fetched on-demand
		m.currentSession = msg.currentSession
		visibleRows := max(1, m.height-1-2-searchBoxH)
		switch m.cfg.Sidebar.FocusOnOpen {
		case "current_session", "first_session":
			m.applyNonAlertFocusMode(m.cfg.Sidebar.FocusOnOpen, visibleRows)
		}
		m.ready = true
		// If states already arrived (Init fetched them in parallel), apply
		// state-dependent startup focus now instead of waiting for statesMsg.
		if !m.startupFocusDone && len(m.states) > 0 {
			m.startupFocusDone = true
			if m.cfg.Sidebar.FocusOnOpen == "state_session" {
				m.applyNonAlertFocusMode("state_session", visibleRows)
			}
			if m.cfg.Sidebar.FocusSearchOnOpen {
				m.searchInput.EnterInsertMode()
			}
		}
		cmds = append(cmds, tick(time.Duration(m.cfg.RefreshIntervalMs)*time.Millisecond), m.fetchStates(), m.fetchWatches(), m.fetchItemSessions())
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
		m.procList.SetSessionData(m.panes, node.Session, m.procs, m.cwdMap, m.gitInfo, m.cfg)
	}
	m.updateDetailFromSelection()
	// Self-schedule next poll in 2s for the selected window.
	return m, m.scheduleDelayedProcFetch()
}

func (m Model) handleStatesMsg(msg statesMsg) (Model, tea.Cmd) {
	m.states = resolveStateTargets(msg.states, m.panes)
	m.procList.SetStates(m.states)
	merged := session.Merge(m.panes, m.sessionsConfig.Entries)
	m.sidebar.SetData(merged, m.states, m.gitInfo, m.cfg)
	// Only apply startup focus once panes have landed (m.ready). If states
	// arrived before panes, handlePanesMsg will apply focus when panes arrive.
	if !m.startupFocusDone && m.ready {
		m.startupFocusDone = true
		if m.cfg.Sidebar.FocusOnOpen == "state_session" {
			visibleRows := max(1, m.height-1-2-searchBoxH)
			m.applyNonAlertFocusMode("state_session", visibleRows)
		}
		if m.cfg.Sidebar.FocusSearchOnOpen {
			m.searchInput.EnterInsertMode()
		}
	}
	m.updateDetailFromSelection()
	var cmds []tea.Cmd
	if node := m.sidebar.Selected(); node != nil {
		m.procGen++
		cmds = append(cmds, m.scheduleProcFetch())
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

// applyNonAlertFocusMode applies the configured focus_on_open mode to the sidebar.
// Valid modes: current_session, first_session, state_session.
// No-ops on empty or unrecognised mode.
func (m *Model) applyNonAlertFocusMode(mode string, visibleRows int) {
	switch mode {
	case "current_session":
		m.sidebar.FocusNode(m.currentSession, visibleRows)
	case "first_session":
		// cursor is already 0, which is always the first session — no-op
	case "state_session":
		if sess := m.sidebar.FirstStateSession(); sess != "" {
			m.sidebar.FocusNode(sess, visibleRows)
		}
	}
}

func (m *Model) stateMap() map[string]db.ToolState {
	out := make(map[string]db.ToolState, len(m.states))
	for _, s := range m.states {
		out[s.Target] = s
	}
	return out
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
		sessionStates:  statesForSession(m.states, node.Session),
	}
}

// statesForSession returns all ToolState records whose Target matches or is prefixed by
// "session:" (i.e. targets belonging to that session).
func statesForSession(states []db.ToolState, session string) []db.ToolState {
	prefix := session + ":"
	var out []db.ToolState
	for _, st := range states {
		if st.Target == session || strings.HasPrefix(st.Target, prefix) {
			out = append(out, st)
		}
	}
	return out
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
	sessionCWD := tmux.PrimaryPaneCWD(windows[0])
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
		windowIndex: winIdx,
		windowPanes: wPanes,
		windowGit:   m.gitInfo[gitKey],
	}
}

func (m Model) handleWatchesMsg(msg watchesMsg) (Model, tea.Cmd) {
	m.sidebar.SetWatches(msg.watches)
	return m, nil
}

func (m Model) handleItemSessionsMsg(msg itemSessionsMsg) (Model, tea.Cmd) {
	m.sidebar.SetItemSessions(msg.sessions)
	m.sidebar.SetNoteSessions(msg.noteSessions)
	return m, nil
}

func (m Model) handleItemsMsg(msg itemsMsg) (Model, tea.Cmd) {
	if msg.session != m.itemSession {
		return m, nil // stale response
	}
	m.itemList = msg.items
	if m.itemCursor >= len(m.itemList) {
		if len(m.itemList) > 0 {
			m.itemCursor = len(m.itemList) - 1
		} else {
			m.itemCursor = 0
		}
	}
	return m, nil
}

func (m Model) handleItemDeleteConfirmed(msg itemDeleteConfirmedMsg) (Model, tea.Cmd) {
	if err := m.db.ItemDelete(msg.id); err != nil {
		demuxlog.Error("item delete failed", "id", msg.id, "err", err)
		m.statusMsg = "error deleting item: " + err.Error()
		m.statusExp = time.Now().Add(4 * time.Second)
	} else {
		demuxlog.Info("item deleted", "id", msg.id, "session", msg.session)
	}
	return m, tea.Batch(m.fetchItems(m.itemSession), m.fetchItemSessions())
}

func (m Model) handleItemEditorDoneMsg(msg itemEditorDoneMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		demuxlog.Error("editor exited with error", "err", msg.err)
		m.statusMsg = "editor error: " + msg.err.Error()
		m.statusExp = time.Now().Add(4 * time.Second)
		return m, nil
	}
	if msg.tempFile != "" {
		if err := syncItemsFromFile(m.db, msg.session, msg.tempFile); err != nil {
			demuxlog.Error("item editor sync failed", "err", err)
			m.statusMsg = "error syncing editor changes: " + err.Error()
			m.statusExp = time.Now().Add(4 * time.Second)
		} else {
			demuxlog.Info("editor changes synced", "session", msg.session)
		}
	}
	return m, tea.Batch(m.fetchItems(m.itemSession), m.fetchItemSessions())
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
