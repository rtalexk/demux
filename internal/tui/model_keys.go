package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rtalexk/demux/internal/db"
	"github.com/rtalexk/demux/internal/proc"
	"github.com/rtalexk/demux/internal/query"
	"github.com/rtalexk/demux/internal/session"
	"github.com/rtalexk/demux/internal/tmux"
)

const (
	dirDown = 1
	dirUp   = -1

	// tmuxScrollbackLines is the number of lines passed to tmux capture-pane -S.
	// Negative value = capture from that many lines above the visible area.
	tmuxScrollbackLines = "-32768"

	// clearConfirmTargetReserve accounts for the "  target: " label (10 chars), border (1), and padding (1).
	clearConfirmTargetReserve = 12
)

// resolveFilterKey maps a key message to a SidebarFilter.
// Returns (filter, true) if the key matches a filter binding, ("", false) otherwise.
func resolveFilterKey(msg tea.KeyMsg) (SidebarFilter, bool) {
	switch {
	case key.Matches(msg, keys.StateFilter.Binding):
		return FilterPriority, true
	case key.Matches(msg, keys.WatchFilter.Binding):
		return FilterWatch, true
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
	// 'a' is context-sensitive: action menu in proclist, FilterAll in sidebar.
	if msg.String() == "a" && m.focus == panelProcList {
		return m.openActionMenu(), nil
	}
	if newFilter, ok := resolveFilterKey(msg); ok {
		sidebarVisibleRows := m.height - statusBarH - borderOverhead - searchBoxH
		if sidebarVisibleRows < 1 {
			sidebarVisibleRows = 1
		}
		m.sidebar.SetFilter(newFilter, sidebarVisibleRows)
		if node := m.sidebar.Selected(); node != nil {
			m.procList.SetSessionData(m.panes, node.Session, m.procs, m.cwdMap, m.gitInfo, m.cfg)
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

	// When the item input has focus, route all keys directly to it so
	// that alphanumeric global bindings (y, q, R, C, …) are not triggered.
	if m.itemInput.IsActive() {
		return m.handleItemInputKey(msg)
	}

	// Global bindings are always active, even in items mode.
	switch {
	case key.Matches(msg, keys.Quit.Binding):
		return m, tea.Quit
	case key.Matches(msg, keys.FocusSidebar.Binding):
		m.focus = panelSidebar
		m.updateDetailFromSelection()
		return m, nil
	case key.Matches(msg, keys.FocusProcList.Binding):
		// compact mode: FocusProcList is a no-op; panelSidebar is the only panel
		if m.cfg.Mode != "compact" {
			m.focus = panelProcList
		}
		// updateDetailFromSelection gates on m.focus, so safe to call unconditionally
		m.updateDetailFromSelection()
		return m, nil
	case key.Matches(msg, keys.Help.Binding):
		m.showHelp = !m.showHelp
		return m, nil
	case key.Matches(msg, keys.Yank.Binding):
		m.populateYankFields()
		m.showYank = true
		return m, nil
	case key.Matches(msg, keys.Refresh.Binding):
		m.procGen++
		return m, tea.Batch(m.fetchPanes(), m.fetchStates(), m.fetchWatches(), m.fetchItemSessions(), m.scheduleProcFetch())
	case msg.Type == tea.KeyF12:
		// Sticky-mode follow poke: force-refresh and refocus the sidebar
		// on the now-active session on the resulting panesMsg.
		if m.cfg.StickyMode {
			m.pendingStickyRefocus = true
			m.procGen++
			return m, tea.Batch(m.fetchPanes(), m.fetchStates(), m.fetchWatches(), m.fetchItemSessions(), m.scheduleProcFetch())
		}
		return m, nil
	case msg.String() == "C":
		// C toggles items mode globally regardless of focus.
		// Not available in compact mode (no proclist panel to render into).
		if m.cfg.Mode == "compact" {
			return m, nil
		}
		if m.itemsMode {
			m.itemsMode = false
			m.itemInput.Exit()
			return m, nil
		}
		if node := m.sidebar.Selected(); node != nil {
			m.focus = panelProcList
			return m.openItemsMode(node.Session)
		}
		return m, nil
	}

	// Items mode intercepts remaining keys only when the proclist panel is focused.
	// When the sidebar is focused, normal sidebar shortcuts take precedence.
	// (itemInput.IsActive() is already handled above.)
	if m.itemsMode && m.focus == panelProcList {
		return m.handleItemsKey(msg)
	}

	return m.handleNormalModeDefault(msg)
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
		m.procList.SetSessionData(m.panes, node.Session, m.procs, m.cwdMap, m.gitInfo, m.cfg)
		m.procGen++
		m.updateDetailFromSelection()
		return m, m.scheduleProcFetch()
	}
	return m, nil
}

// handleSearchInsertKey handles key events when the search input is in insert mode.
// Returns the updated model and command.
func (m Model) handleSearchInsertKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	sidebarVisibleRows := m.height - statusBarH - borderOverhead - searchBoxH
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
	if err := tmux.NewSessionDetached(sess.DisplayName, sess.Config.Path); err != nil {
		m.statusMsg = "launch failed: " + err.Error()
		m.statusExp = time.Now().Add(statusExpError)
		m.sidebar.SetLaunchErr(err.Error())
		return m, nil
	}
	if specs := resolveWindowSpecs(sess.Config.Windows, m.sessionsConfig.WindowTemplates); len(specs) > 0 {
		if err := tmux.CreateSessionWindows(sess.DisplayName, sess.Config.Path, specs); err != nil {
			m.statusMsg = "window setup failed: " + err.Error()
			m.statusExp = time.Now().Add(statusExpError)
		}
	}
	if err := tmux.Connect(sess.DisplayName); err != nil {
		m.statusMsg = "connect failed: " + err.Error()
		m.statusExp = time.Now().Add(statusExpError)
		m.sidebar.SetLaunchErr(err.Error())
		return m, nil
	}
	m.sidebar.ClearLaunchErr()
	if m.popupMode {
		return m, tea.Quit
	}
	return m, m.fetchPanes()
}

// highestPriorityPaneTarget returns the tmux target ID (%N or @N) of the
// highest-priority active state for the given session ID, or "" when none
// exists. Session-level states are skipped; idle states are excluded.
func highestPriorityPaneTarget(sessionID string, states []db.ToolState) string {
	best := ""
	bestPri := -1
	for _, st := range states {
		if st.Target.SessionID != sessionID || st.Target.Type == db.TargetTypeSession {
			continue
		}
		if !st.Value.IsDisplayable() {
			continue
		}
		if pri := st.Value.Priority(); pri > bestPri {
			bestPri = pri
			best = st.Target.ID
		}
	}
	return best
}

// switchLiveSession switches the tmux client to the given session, landing on
// the pane with the highest-priority active state when one exists.
func (m Model) switchLiveSession(sessionName string) (Model, tea.Cmd) {
	sessionID := m.nameToIDMap[sessionName]
	target := highestPriorityPaneTarget(sessionID, m.states)
	if target == "" {
		target = sessionName
	}
	err := tmux.SwitchClient(target)
	if err == nil && m.popupMode {
		return m, tea.Quit
	}
	if err == nil && m.cfg.StickyMode {
		// In sticky mode the user wants 'o'/Enter to drop them out of the
		// sidebar pane into the session content (otherwise focus stays in
		// the sidebar). Equivalent to tmux's default `prefix + o`.
		_ = tmux.SelectNextPane()
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
		m.procList.SetSessionData(m.panes, node.Session, m.procs, m.cwdMap, m.gitInfo, m.cfg)
		m.procGen++
		cmd = m.scheduleProcFetch()
		// When items mode is active, follow the focused session.
		if m.itemsMode && node.Session != m.itemSession {
			m.itemSession = node.Session
			m.itemCursor = 0
			m.itemInput.Exit()
			cmd = tea.Batch(cmd, m.fetchItems(node.Session))
		}
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
	sidebarVisibleRows := m.height - statusBarH - borderOverhead - searchBoxH
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
	case key.Matches(msg, keys.FlagState.Binding):
		if node := m.sidebar.Selected(); node != nil {
			if st := m.sidebar.stateForSession(node.Session); st != nil && st.Value == db.StateFlagged {
				return m, m.clearCurrentState(st.Target)
			}
			sessionID := m.nameToIDMap[node.Session]
			if sessionID == "" {
				break // session not live in tmux; no stable ID to flag
			}
			t := db.Target{Type: db.TargetTypeSession, ID: sessionID, SessionID: sessionID}
			return m, m.flagCurrentState(t)
		}
	case key.Matches(msg, keys.WatchSession.Binding):
		if node := m.sidebar.Selected(); node != nil {
			return m, m.watchCurrentSession(node.Session)
		}
	case key.Matches(msg, keys.ClearState.Binding):
		if node := m.sidebar.Selected(); node != nil {
			if st := m.sidebar.stateForSession(node.Session); st != nil {
				return m.showClearConfirm(st.Target), nil
			}
			sessionID := m.nameToIDMap[node.Session]
			if sessionID == "" {
				break // session not live in tmux; nothing to clear
			}
			t := db.Target{Type: db.TargetTypeSession, ID: sessionID, SessionID: sessionID}
			return m.showClearConfirm(t), nil
		}
	}
	return m.sidebarNavUpdate()
}

// procListDimensions computes procH and detailH for the current model state.
func (m Model) procListDimensions() (procH, detailH int) {
	dims := m.buildLayoutDims()
	return dims.procH, dims.detailH
}

// afterCollapse performs the shared post-toggle update after an expand/collapse operation:
// rebuilds proc data for the current selection, clamps viewport, updates detail,
// and schedules a delayed proc fetch.
func (m Model) afterCollapse(procH int) (Model, tea.Cmd) {
	if node := m.sidebar.Selected(); node != nil {
		m.procList.SetSessionData(m.panes, node.Session, m.procs, m.cwdMap, m.gitInfo, m.cfg)
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
		target = pane.DisplayLabel()
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
	case msg.String() == "x":
		if node := m.procList.SelectedNode(); node != nil && node.Proc.PID != 0 {
			pid := node.Proc.PID
			name := node.Proc.FriendlyName()
			m.confirm = ConfirmModel{
				prompt: fmt.Sprintf("Kill %s (PID %d)?", name, pid),
			}
			m.confirmCmd = killProcCmd(pid)
			m.showConfirm = true
			return m, nil
		}
	case key.Matches(msg, keys.Open.Binding):
		nm, cmd := m.handleProcListOpen()
		nm.procList.clampOffset(procH - 2)
		nm.updateDetailFromSelection()
		return nm, cmd
	case key.Matches(msg, keys.Esc.Binding):
		m.focus = panelSidebar
	case key.Matches(msg, keys.FlagState.Binding):
		if t := m.procListStateIdentity(); t != nil {
			if st := activeStateFor(m.states, t.Type, t.ID); st != nil && st.Value == db.StateFlagged {
				return m, m.clearCurrentState(*t)
			}
			return m, m.flagCurrentState(*t)
		}
	case key.Matches(msg, keys.WatchSession.Binding):
		if sess := m.procListSession(); sess != "" {
			return m, m.watchCurrentSession(sess)
		}
	case key.Matches(msg, keys.ClearState.Binding):
		if t := m.procListStateIdentity(); t != nil {
			return m.showClearConfirm(*t), nil
		}
	}
	m.procList.clampOffset(procH - 2) // procH includes border; pass inner content height
	m.updateDetailFromSelection()
	return m, nil
}

// watchCurrentSession toggles the watch state for the given session.
func (m Model) watchCurrentSession(session string) tea.Cmd {
	d := m.db
	_, watched := m.sidebar.watches[session]
	return func() tea.Msg {
		if watched {
			_ = d.WatchClear(session)
		} else {
			_ = d.WatchSet(session)
		}
		list, _ := d.WatchList()
		return watchesMsg{watches: list}
	}
}

// flagCurrentState sets the flagged state for the given target.
func (m Model) flagCurrentState(t db.Target) tea.Cmd {
	d := m.db
	msg := m.cfg.Tui.FlagDefaultMessage
	return func() tea.Msg {
		_ = d.StateSet(t, "", db.StateFlagged, msg, db.SourceUser, false, nil)
		states, _ := d.StateList(0, "")
		return statesMsg{states: states}
	}
}

// procListSession returns the session name for the currently selected proc list node.
func (m Model) procListSession() string {
	node := m.procList.SelectedNode()
	if node == nil {
		return ""
	}
	return node.Pane.Session
}

// procListStateIdentity returns the typed Target for the currently selected
// proc list node: window Target for window headers, pane Target for pane/proc nodes.
func (m Model) procListStateIdentity() *db.Target {
	node := m.procList.SelectedNode()
	if node == nil {
		return nil
	}
	pane := node.Pane
	if node.IsWindowHeader {
		if pane.WindowID == "" {
			return nil
		}
		t := db.Target{
			Type:      db.TargetTypeWindow,
			ID:        pane.WindowID,
			SessionID: pane.SessionID,
			WindowID:  pane.WindowID,
		}
		return &t
	}
	if pane.PaneID == "" {
		return nil
	}
	t := db.Target{
		Type:      db.TargetTypePane,
		ID:        pane.PaneID,
		SessionID: pane.SessionID,
		WindowID:  pane.WindowID,
		PaneID:    pane.PaneID,
	}
	return &t
}

// clearCurrentState clears the state for the given typed target.
func (m Model) clearCurrentState(t db.Target) tea.Cmd {
	d := m.db
	return func() tea.Msg {
		_ = d.StateClear(t)
		states, _ := d.StateList(0, "")
		return statesMsg{states: states}
	}
}

// showClearConfirm opens the confirmation overlay for clearing the state of target.
func (m Model) showClearConfirm(t db.Target) Model {
	st := activeStateFor(m.states, t.Type, t.ID)
	maxTargetLen := m.width/2 - clearConfirmTargetReserve
	if maxTargetLen < 20 {
		maxTargetLen = 20
	}
	targetDisplay := t.Format(m.paneIDMap, m.windowIDMap, m.sessionIDMap)
	body := "  target: " + truncateTarget(targetDisplay, maxTargetLen)
	if st != nil {
		body += "\n  state:  " + st.Value.String()
		if st.Message != "" {
			msg := st.Message
			if runes := []rune(msg); len(runes) > maxTargetLen {
				msg = string(runes[:maxTargetLen-1]) + ellipsis
			}
			body += "\n          " + msg
		}
	}
	m.confirm = ConfirmModel{
		prompt: "Clear state?",
		body:   body,
	}
	m.confirmCmd = m.clearCurrentState(t)
	m.showConfirm = true
	return m
}

func (m Model) openSidebarSelected() (Model, tea.Cmd) {
	node := m.sidebar.Selected()
	if node == nil {
		return m, nil
	}
	sess := m.sidebar.FindSession(node.Session)
	return m.launchOrSwitchSession(sess, node.Session)
}

// killProcCmd returns a tea.Cmd that sends SIGKILL to pid and reports the result
// via procActionMsg. Shared by the direct-x handler and the action-menu kill path.
func killProcCmd(pid int32) tea.Cmd {
	return func() tea.Msg {
		if err := proc.Kill(pid); err != nil {
			return procActionMsg{fmt.Sprintf("kill %d: %v", pid, err)}
		}
		return procActionMsg{fmt.Sprintf("killed PID %d", pid)}
	}
}

// handleActionMenuKey routes keys when the action menu (or a sub-popup) is open.
func (m Model) handleActionMenuKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.actionMenu.subPopup != SubPopupNone {
		return m.handleSubPopupKey(msg)
	}
	// Dispatch single-key shortcuts defined on each actionItem. This avoids
	// duplicating the shortcut mapping here and in buildActionItems.
	pressed := msg.String()
	for i, item := range m.actionMenu.items {
		if item.Shortcut != "" && item.Shortcut == pressed {
			m.actionMenu.cursor = i
			return m.executeActionMenuItem()
		}
	}
	switch {
	case key.Matches(msg, keys.Down.Binding):
		m.actionMenu.MoveDown()
	case key.Matches(msg, keys.Up.Binding):
		m.actionMenu.MoveUp()
	case key.Matches(msg, keys.Enter.Binding):
		return m.executeActionMenuItem()
	case msg.String() == "q", key.Matches(msg, keys.Esc.Binding):
		m.showActionMenu = false
	}
	return m, nil
}

// handleSubPopupKey routes keys when a sub-popup is open inside the action menu.
func (m Model) handleSubPopupKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch m.actionMenu.subPopup {
	case SubPopupKillConfirm:
		switch msg.String() {
		case "y", "enter":
			pid := m.actionMenu.target.Proc.PID
			m.showActionMenu = false
			m.actionMenu.subPopup = SubPopupNone
			return m, killProcCmd(pid)
		case "n", "esc", "q":
			m.actionMenu.subPopup = SubPopupNone
		}
	}
	return m, nil
}

// executeActionMenuItem executes the currently selected action menu item.
func (m Model) executeActionMenuItem() (Model, tea.Cmd) {
	item := m.actionMenu.Selected()
	if item == nil {
		return m, nil
	}
	switch item.Kind {
	case ActionKill:
		m.actionMenu.subPopup = SubPopupKillConfirm
		return m, nil

	case ActionRestart:
		paneID := m.actionMenu.target.Pane.PaneID
		m.showActionMenu = false
		return m, func() tea.Msg {
			if err := exec.Command("tmux", "send-keys", "-t", paneID, "Up", "Enter").Run(); err != nil {
				return procActionMsg{"restart failed: " + err.Error()}
			}
			return procActionMsg{"restart sent to " + paneID}
		}

	case ActionViewLogs:
		paneID := m.actionMenu.target.Pane.PaneID
		m.showActionMenu = false
		return m, func() tea.Msg {
			out, err := exec.Command("tmux", "capture-pane", "-t", paneID, "-p", "-S", tmuxScrollbackLines).Output()
			if err != nil {
				return procActionMsg{"logs: " + err.Error()}
			}
			path, err := writeTempFile("demux-logs-*.log", out)
			if err != nil {
				return procActionMsg{"logs: " + err.Error()}
			}
			return openViewerMsg{path: path}
		}

	case ActionOpenBrowser:
		port := m.actionMenu.target.Port
		m.showActionMenu = false
		return m, func() tea.Msg {
			url := fmt.Sprintf("http://localhost:%d", port)
			var cmd *exec.Cmd
			switch runtime.GOOS {
			case "darwin":
				cmd = exec.Command("open", url)
			case "linux":
				cmd = exec.Command("xdg-open", url)
			default:
				return procActionMsg{"open browser: unsupported on " + runtime.GOOS}
			}
			if err := cmd.Run(); err != nil {
				return procActionMsg{"open browser: " + err.Error()}
			}
			return procActionMsg{"opened browser"}
		}

	case ActionShowEnv:
		pid := m.actionMenu.target.Proc.PID
		m.showActionMenu = false
		return m, func() tea.Msg {
			lines, err := proc.Environ(pid)
			if err != nil {
				return procActionMsg{"env: " + err.Error()}
			}
			path, err := writeTempFile("demux-env-*.txt", []byte(strings.Join(lines, "\n")+"\n"))
			if err != nil {
				return procActionMsg{"env: " + err.Error()}
			}
			return openViewerMsg{path: path, ftCmd: "set ft=sh"}
		}

	case ActionShowFullCmd:
		cmdline := m.actionMenu.target.Proc.Cmdline
		m.showActionMenu = false
		return m, func() tea.Msg {
			path, err := writeTempFile("demux-cmd-*.txt", []byte(cmdline+"\n"))
			if err != nil {
				return procActionMsg{"cmd: " + err.Error()}
			}
			return openViewerMsg{path: path}
		}
	}
	return m, nil
}

// openActionMenu opens the action menu for the currently selected process node.
// No-op when the cursor is not on a selectable process node.
func (m Model) openActionMenu() Model {
	node := m.procList.SelectedNode()
	if node == nil {
		return m
	}
	if node.IsPaneHeader || node.IsWindowHeader || node.Proc.PID == 0 {
		return m
	}
	// displayPane suppresses CWD when it matches the window CWD. Recover the real
	// value from the live pane list so Copy Working Directory is always available.
	if node.Pane.CWD == "" && node.Pane.PaneID != "" {
		for _, p := range m.panes {
			if p.PaneID == node.Pane.PaneID {
				node.Pane.CWD = p.CWD
				break
			}
		}
	}
	// For process nodes prefer the live async CWD (may differ from pane CWD).
	if node.Proc.PID != 0 {
		if cwd := m.cwdMap[node.Proc.PID]; cwd != "" {
			node.Pane.CWD = cwd
		}
	}
	m.actionMenu = actionMenuModel{
		items:  buildActionItems(*node),
		cursor: 0,
		target: *node,
	}
	m.showActionMenu = true
	return m
}
