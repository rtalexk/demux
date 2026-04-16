package tui

import (
	"fmt"
	"os"
	"os/exec"
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
	case marqueeTickMsg:
		return m.handleMarqueeTickMsg()
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
	case openViewerMsg:
		return m.handleOpenViewerMsg(msg)
	case procActionMsg:
		m.statusMsg = msg.status
		m.statusExp = time.Now().Add(3 * time.Second)
		return m, nil
	}
	return m, nil
}

func (m Model) handleOpenViewerMsg(msg openViewerMsg) (Model, tea.Cmd) {
	args := findViewer()
	if msg.ftCmd != "" {
		args = append(args, "-c", msg.ftCmd)
	}
	args = append(args, msg.path)
	return m, tea.ExecProcess(exec.Command(args[0], args[1:]...), func(err error) tea.Msg {
		_ = os.Remove(msg.path)
		return nil
	})
}

// findViewer returns the command+args for the best available read-only viewer:
// nvim, vim, or vi (all accept -R for read-only mode).
func findViewer() []string {
	for _, ed := range []string{"nvim", "vim", "vi"} {
		if _, err := exec.LookPath(ed); err == nil {
			return []string{ed, "-R"}
		}
	}
	return []string{"vi", "-R"}
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
	if m.showActionMenu {
		mo, cmd := m.handleActionMenuKey(msg.(tea.KeyMsg))
		return mo.(Model), cmd, true
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

func (m Model) handleMarqueeTickMsg() (Model, tea.Cmd) {
	if m.focus == panelSidebar {
		m.sidebar.TickMarquee()
	}
	if m.showYank {
		m.yank.TickMarquee()
	}
	return m, marqueeCmd()
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

func (m Model) handlePanesMsg(msg panesMsg) (Model, tea.Cmd) {
	m.panes = msg.panes
	// Build display maps from fresh pane data.
	m.paneIDMap = tmux.PaneIDToTargetMap(msg.panes)
	m.windowIDMap = tmux.WindowIDToTargetMap(msg.panes)
	m.sessionIDMap = tmux.SessionIDToNameMap(msg.panes)
	m.nameToIDMap = tmux.SessionNameToIDMap(msg.panes)
	m.sidebar.SetNameToIDMap(m.nameToIDMap)
	healStateSessionIDs(m.states, m.paneIDMap, m.nameToIDMap)
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
	m.states = msg.states
	healStateSessionIDs(m.states, m.paneIDMap, m.nameToIDMap)
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
			}, fmt.Sprintf("Yank: %s (%d)", pr.Name, pr.PID))
			return
		}
	}
	// session node from sidebar
	if node := m.sidebar.Selected(); node != nil {
		m.yank.SetFields([]YankField{
			{Key: "n", Label: "session", Value: node.Session},
			{Key: "t", Label: "target", Value: node.Session},
		}, "Yank: "+node.Session)
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
		sessionStates:  statesForSession(m.states, m.nameToIDMap[node.Session]),
		paneIDMap:      m.paneIDMap,
		windowIDMap:    m.windowIDMap,
		sessionIDMap:   m.sessionIDMap,
	}
}

// statesForSession returns all ToolState records belonging to the given session ID.
func statesForSession(states []db.ToolState, sessionID string) []db.ToolState {
	var out []db.ToolState
	for _, st := range states {
		if st.Target.SessionID == sessionID {
			out = append(out, st)
		}
	}
	return out
}

// healStateSessionIDs reconciles pane state records whose session_id is empty or
// stale in the DB against the live paneIDMap + nameToIDMap. This corrects records
// where resolveParentIDs failed at write time (e.g. SQLITE_BUSY) or where the
// pane was joined into a new session after the state was written.
//
// Mutations are in-memory only; the DB is not touched here. StateRefreshParentIDs
// (called from pane_focus events) persists corrections back to SQLite lazily.
func healStateSessionIDs(states []db.ToolState, paneIDMap, nameToIDMap map[string]string) {
	for i := range states {
		if states[i].Target.Type != db.TargetTypePane {
			continue
		}
		label, ok := paneIDMap[states[i].Target.ID]
		if !ok {
			continue // pane not in live map; leave as-is
		}
		colonIdx := strings.Index(label, ":")
		if colonIdx <= 0 {
			continue
		}
		sessName := label[:colonIdx]
		sessID := nameToIDMap[sessName]
		if sessID == "" || states[i].Target.SessionID == sessID {
			continue // already correct
		}
		states[i].Target.SessionID = sessID
	}
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
	// Sort todos before notes, preserving relative insertion order within each kind.
	// The renderer groups by kind visually; keeping the flat list in the same order
	// ensures cursor navigation matches what the user sees.
	sorted := make([]db.Item, 0, len(msg.items))
	for _, it := range msg.items {
		if it.Kind == db.KindTodo {
			sorted = append(sorted, it)
		}
	}
	for _, it := range msg.items {
		if it.Kind != db.KindTodo {
			sorted = append(sorted, it)
		}
	}
	m.itemList = sorted
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
			if err := clipboardFunc(val); err != nil {
				m.statusMsg = "copy failed: " + err.Error()
			} else {
				m.statusMsg = "yanked: " + val
			}
			m.showYank = false
			m.statusExp = time.Now().Add(2 * time.Second)
			return m, nil
		}
		if f, ok := m.yank.FieldByKey(msg.String()); ok {
			if err := clipboardFunc(f.Value); err != nil {
				m.statusMsg = "copy failed: " + err.Error()
			} else {
				m.statusMsg = "yanked: " + f.Value
			}
			m.showYank = false
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
