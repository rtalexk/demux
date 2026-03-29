# focus_on_open Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a `focus_on_open` config setting that positions the sidebar cursor on a specific session or window when Demux opens.

**Architecture:** Four tasks in dependency order — config field, tmux helper, sidebar navigation methods, model wiring. Each task is independently testable before the next begins.

**Tech Stack:** Go, BubbleTea (TUI framework), TOML config via BurntSushi/toml, tmux CLI

---

### Task 1: Config field

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Step 1: Write the failing tests**

Add to `internal/config/config_test.go`:

```go
func TestDefaults_FocusOnOpen(t *testing.T) {
    cfg := config.Default()
    if cfg.FocusOnOpen != "current_window" {
        t.Errorf("expected default FocusOnOpen=\"current_window\", got %q", cfg.FocusOnOpen)
    }
}

func TestLoadFromFile_FocusOnOpen(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "demux.toml")
    os.WriteFile(path, []byte(`focus_on_open = "alert_window"`), 0644)
    cfg, err := config.Load(path)
    if err != nil {
        t.Fatal(err)
    }
    if cfg.FocusOnOpen != "alert_window" {
        t.Errorf("expected \"alert_window\", got %q", cfg.FocusOnOpen)
    }
}
```

**Step 2: Run tests to verify they fail**

```bash
cd /Users/rtalex/com.github/demux/main
go test ./internal/config/... -run "TestDefaults_FocusOnOpen|TestLoadFromFile_FocusOnOpen" -v
```

Expected: FAIL — `cfg.FocusOnOpen` is empty string.

**Step 3: Add the field**

In `internal/config/config.go`, add to the `Config` struct after `AlertFilterWindows`:

```go
FocusOnOpen        string      `toml:"focus_on_open"`
```

In `Default()`, add after `AlertFilterWindows: "all",`:

```go
FocusOnOpen: "current_window",
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/config/... -run "TestDefaults_FocusOnOpen|TestLoadFromFile_FocusOnOpen" -v
```

Expected: PASS

**Step 5: Run full suite**

```bash
go test ./...
```

Expected: all pass

**Step 6: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add focus_on_open setting"
```

---

### Task 2: tmux.CurrentTarget + panesMsg

**Files:**
- Modify: `internal/tmux/query.go`
- Modify: `internal/tmux/query_test.go`
- Modify: `internal/tui/model.go`

**Step 1: Write the failing test for ParseCurrentTarget**

`CurrentTarget()` shells out to tmux and can't be unit-tested directly. Instead, add a pure parser function `ParseCurrentTarget(raw string) (string, int, error)` and test that.

Add to `internal/tmux/query_test.go`:

```go
func TestParseCurrentTarget(t *testing.T) {
    session, window, err := ParseCurrentTarget("myproject\t3\n")
    if err != nil {
        t.Fatal(err)
    }
    if session != "myproject" {
        t.Errorf("expected session \"myproject\", got %q", session)
    }
    if window != 3 {
        t.Errorf("expected window 3, got %d", window)
    }
}

func TestParseCurrentTarget_Empty(t *testing.T) {
    _, _, err := ParseCurrentTarget("")
    if err == nil {
        t.Error("expected error for empty input")
    }
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/tmux/... -run "TestParseCurrentTarget" -v
```

Expected: FAIL — `ParseCurrentTarget` undefined.

**Step 3: Implement ParseCurrentTarget and CurrentTarget**

Add to `internal/tmux/query.go`:

```go
// ParseCurrentTarget parses the output of `tmux display-message -p "#{session_name}\t#{window_index}"`.
func ParseCurrentTarget(raw string) (string, int, error) {
    raw = strings.TrimSpace(raw)
    if raw == "" {
        return "", 0, fmt.Errorf("empty output")
    }
    parts := strings.SplitN(raw, "\t", 2)
    if len(parts) < 2 {
        return "", 0, fmt.Errorf("unexpected format: %q", raw)
    }
    wi, err := strconv.Atoi(strings.TrimSpace(parts[1]))
    if err != nil {
        return "", 0, fmt.Errorf("invalid window index: %w", err)
    }
    return parts[0], wi, nil
}

// CurrentTarget returns the session name and window index of the tmux client
// that launched this process. Returns an error if tmux is unavailable.
func CurrentTarget() (string, int, error) {
    out, err := exec.Command("tmux", "display-message", "-p", "#{session_name}\t#{window_index}").Output()
    if err != nil {
        return "", 0, fmt.Errorf("tmux display-message: %w", err)
    }
    return ParseCurrentTarget(string(out))
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/tmux/... -run "TestParseCurrentTarget" -v
```

Expected: PASS

**Step 5: Thread currentSession/currentWindow through panesMsg**

In `internal/tui/model.go`, update `panesMsg`:

```go
type panesMsg struct {
    panes          []tmux.Pane
    currentSession string
    currentWindow  int
}
```

Update `fetchPanes()`:

```go
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
```

**Step 6: Run full suite**

```bash
go test ./...
```

Expected: all pass (panesMsg struct change is backward-compatible — handler fields are just ignored until Task 4).

**Step 7: Commit**

```bash
git add internal/tmux/query.go internal/tmux/query_test.go internal/tui/model.go
git commit -m "feat(tmux): add CurrentTarget; thread current session/window through panesMsg"
```

---

### Task 3: SidebarModel navigation methods

**Files:**
- Modify: `internal/tui/sidebar.go`
- Modify: `internal/tui/sidebar_test.go`

**Background:** The sidebar stores a flat `[]SidebarNode`. Session nodes have `IsSession: true`. Window nodes follow their session node immediately when the session is expanded. All sessions start expanded by default. The cursor is an index into this slice.

**Step 1: Write failing tests**

Add to `internal/tui/sidebar_test.go`:

```go
// --- FocusNode ---

func TestFocusNode_SessionLevel(t *testing.T) {
    s := SidebarModel{
        sessions: map[string]map[int][]tmux.Pane{
            "alpha": {0: nil},
            "beta":  {0: nil, 1: nil},
        },
        alerts: map[string]db.Alert{},
        cfg:    config.Config{SessionSort: []string{"alphabetical"}},
    }
    s.rebuildNodes()
    s.FocusNode("beta", 0, true, 20)
    node := s.Selected()
    if node == nil || !node.IsSession || node.Session != "beta" {
        t.Errorf("expected session node beta, got %+v", node)
    }
}

func TestFocusNode_WindowLevel(t *testing.T) {
    s := SidebarModel{
        sessions: map[string]map[int][]tmux.Pane{
            "sess": {0: nil, 2: nil},
        },
        alerts: map[string]db.Alert{},
        cfg:    config.Config{SessionSort: []string{"alphabetical"}},
    }
    s.rebuildNodes()
    s.FocusNode("sess", 2, false, 20)
    node := s.Selected()
    if node == nil || node.IsSession || node.Session != "sess" || node.WindowIndex != 2 {
        t.Errorf("expected window node sess:2, got %+v", node)
    }
}

func TestFocusNode_NoMatch_LeavesCursorAt0(t *testing.T) {
    s := SidebarModel{
        sessions: makeSessions("alpha"),
        alerts:   map[string]db.Alert{},
    }
    s.rebuildNodes()
    s.FocusNode("nonexistent", 0, true, 20)
    if s.cursor != 0 {
        t.Errorf("expected cursor=0, got %d", s.cursor)
    }
}

// --- FocusFirstAlertSession ---

func TestFocusFirstAlertSession_MovesToAlertedSession(t *testing.T) {
    t1 := time.Now()
    s := SidebarModel{
        sessions: makeSessions("alpha", "beta"),
        alerts: map[string]db.Alert{
            "beta:0.0": {Target: "beta:0.0", Level: "warn", CreatedAt: t1},
        },
        cfg: config.Config{SessionSort: []string{"alphabetical"}},
    }
    s.rebuildNodes()
    // alphabetical: alpha first, beta second; but beta has alert so it sorts first
    // force alphabetical order so beta is NOT first by sort, to test the method itself
    s.cfg.SessionSort = []string{"alphabetical"}
    s.rebuildNodes()
    s.FocusFirstAlertSession(20)
    node := s.Selected()
    if node == nil || !node.IsSession || node.Session != "beta" {
        t.Errorf("expected session node beta, got %+v", node)
    }
}

func TestFocusFirstAlertSession_NoAlerts_LeavesCursorAt0(t *testing.T) {
    s := SidebarModel{
        sessions: makeSessions("alpha", "beta"),
        alerts:   map[string]db.Alert{},
    }
    s.rebuildNodes()
    s.FocusFirstAlertSession(20)
    if s.cursor != 0 {
        t.Errorf("expected cursor=0, got %d", s.cursor)
    }
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/tui/... -run "TestFocusNode|TestFocusFirstAlertSession" -v
```

Expected: FAIL — methods undefined.

**Step 3: Extract focusFirstAlertWindow helper**

In `internal/tui/sidebar.go`, extract the alert-window logic currently inside `ToggleAlertFilter` into a private helper, then call it from both `ToggleAlertFilter` and the new public method.

Find this block inside `ToggleAlertFilter`:

```go
if s.filterAlerts {
    // Move cursor to first window node with an alert.
    for i, n := range s.nodes {
        if n.IsSession {
            continue
        }
        if s.windowAlert(n.Session, n.WindowIndex) != nil {
            s.cursor = i
            break
        }
    }
}
```

Replace with a call to the new helper:

```go
if s.filterAlerts {
    s.focusFirstAlertWindow()
}
```

Add the helper (private, no viewport clamp — callers do that):

```go
func (s *SidebarModel) focusFirstAlertWindow() {
    for i, n := range s.nodes {
        if n.IsSession {
            continue
        }
        if s.windowAlert(n.Session, n.WindowIndex) != nil {
            s.cursor = i
            return
        }
    }
}
```

**Step 4: Add FocusNode and FocusFirstAlertSession**

Add to `internal/tui/sidebar.go`:

```go
// FocusNode positions the cursor on the node matching session+windowIndex.
// If isSessionLevel is true, targets the session node; otherwise targets the window node.
// No-ops if no matching node is found, leaving cursor at its current position.
func (s *SidebarModel) FocusNode(session string, windowIndex int, isSessionLevel bool, visibleRows int) {
    for i, n := range s.nodes {
        if n.Session != session {
            continue
        }
        if isSessionLevel && n.IsSession {
            s.cursor = i
            s.clampViewport(visibleRows)
            return
        }
        if !isSessionLevel && !n.IsSession && n.WindowIndex == windowIndex {
            s.cursor = i
            s.clampViewport(visibleRows)
            return
        }
    }
}

// FocusFirstAlertSession positions the cursor on the first session node that has any alert.
// No-ops if no alerted session exists.
func (s *SidebarModel) FocusFirstAlertSession(visibleRows int) {
    for i, n := range s.nodes {
        if !n.IsSession {
            continue
        }
        if !s.newestSessionAlert(n.Session).IsZero() {
            s.cursor = i
            s.clampViewport(visibleRows)
            return
        }
    }
}

// FocusFirstAlertWindow positions the cursor on the first window node that has any alert.
// No-ops if no alerted window exists.
func (s *SidebarModel) FocusFirstAlertWindow(visibleRows int) {
    s.focusFirstAlertWindow()
    s.clampViewport(visibleRows)
}
```

**Step 5: Run the new tests**

```bash
go test ./internal/tui/... -run "TestFocusNode|TestFocusFirstAlertSession" -v
```

Expected: PASS

**Step 6: Run full suite**

```bash
go test ./...
```

Expected: all pass

**Step 7: Commit**

```bash
git add internal/tui/sidebar.go internal/tui/sidebar_test.go
git commit -m "feat(tui): add FocusNode, FocusFirstAlertSession, FocusFirstAlertWindow to SidebarModel"
```

---

### Task 4: Wire focus_on_open in Model

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/proclist_open_test.go` (or add a new `model_focus_test.go` — see below)

**Background:** The model processes two async messages on startup:
1. `panesMsg` — arrives first, triggers `m.ready = true` on first receipt
2. `alertsMsg` — arrives shortly after, triggered by the `fetchAlerts()` call inside the first `panesMsg` handler

`current_*` modes are applied in the first `panesMsg` handler because we don't need alerts.
`alert_*` modes are applied in the first `alertsMsg` handler, guarded by `startupFocusDone`.

The `visibleRows` formula used throughout the model is `m.height - 1 - 2`. Since height may be 0 at startup, pass at least 1.

**Step 1: Write failing tests**

Create `internal/tui/model_focus_test.go`:

```go
package tui

import (
    "testing"
    "time"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/rtalex/demux/internal/config"
    "github.com/rtalex/demux/internal/db"
    "github.com/rtalex/demux/internal/tmux"
)

func focusTestModel(focusOnOpen string) Model {
    cfg := config.Default()
    cfg.FocusOnOpen = focusOnOpen
    database, _ := db.Open(":memory:")
    return New(cfg, database)
}

func applyPanesMsg(m Model, currentSession string, currentWindow int) (Model, tea.Cmd) {
    panes := []tmux.Pane{
        {Session: "alpha", WindowIndex: 0},
        {Session: "alpha", WindowIndex: 1},
        {Session: "beta", WindowIndex: 0},
    }
    msg := panesMsg{panes: panes, currentSession: currentSession, currentWindow: currentWindow}
    updated, cmd := m.Update(msg)
    return updated.(Model), cmd
}

func applyAlertsMsg(m Model, alerts []db.Alert) (Model, tea.Cmd) {
    updated, cmd := m.Update(alertsMsg{alerts: alerts})
    return updated.(Model), cmd
}

func TestFocusOnOpen_CurrentWindow(t *testing.T) {
    m := focusTestModel("current_window")
    m.height = 40
    m, _ = applyPanesMsg(m, "beta", 0)
    node := m.sidebar.Selected()
    if node == nil || node.IsSession || node.Session != "beta" || node.WindowIndex != 0 {
        t.Errorf("expected window node beta:0, got %+v", node)
    }
}

func TestFocusOnOpen_CurrentSession(t *testing.T) {
    m := focusTestModel("current_session")
    m.height = 40
    m, _ = applyPanesMsg(m, "beta", 0)
    node := m.sidebar.Selected()
    if node == nil || !node.IsSession || node.Session != "beta" {
        t.Errorf("expected session node beta, got %+v", node)
    }
}

func TestFocusOnOpen_AlertWindow(t *testing.T) {
    m := focusTestModel("alert_window")
    m.height = 40
    m, _ = applyPanesMsg(m, "alpha", 0)
    alerts := []db.Alert{
        {Target: "beta:0", Level: "warn", CreatedAt: time.Now()},
    }
    m, _ = applyAlertsMsg(m, alerts)
    node := m.sidebar.Selected()
    if node == nil || node.IsSession || node.Session != "beta" || node.WindowIndex != 0 {
        t.Errorf("expected window node beta:0, got %+v", node)
    }
}

func TestFocusOnOpen_AlertSession(t *testing.T) {
    m := focusTestModel("alert_session")
    m.height = 40
    m, _ = applyPanesMsg(m, "alpha", 0)
    alerts := []db.Alert{
        {Target: "beta:0", Level: "warn", CreatedAt: time.Now()},
    }
    m, _ = applyAlertsMsg(m, alerts)
    node := m.sidebar.Selected()
    if node == nil || !node.IsSession || node.Session != "beta" {
        t.Errorf("expected session node beta, got %+v", node)
    }
}

func TestFocusOnOpen_AlertWindow_NoAlerts_StaysAt0(t *testing.T) {
    m := focusTestModel("alert_window")
    m.height = 40
    m, _ = applyPanesMsg(m, "alpha", 0)
    m, _ = applyAlertsMsg(m, nil)
    node := m.sidebar.Selected()
    if node == nil {
        t.Fatal("expected a node, got nil")
    }
    // cursor should be at 0 — whatever that node is
    if m.sidebar.cursor != 0 {
        t.Errorf("expected cursor=0, got %d", m.sidebar.cursor)
    }
}

func TestFocusOnOpen_SubsequentAlerts_DoNotRefocus(t *testing.T) {
    m := focusTestModel("alert_window")
    m.height = 40
    m, _ = applyPanesMsg(m, "alpha", 0)
    alerts := []db.Alert{
        {Target: "beta:0", Level: "warn", CreatedAt: time.Now()},
    }
    m, _ = applyAlertsMsg(m, alerts)
    firstCursor := m.sidebar.cursor
    // Simulate a second alerts tick with a different alert
    alerts2 := []db.Alert{
        {Target: "alpha:1", Level: "error", CreatedAt: time.Now()},
    }
    m, _ = applyAlertsMsg(m, alerts2)
    if m.sidebar.cursor != firstCursor {
        t.Errorf("expected cursor to stay at %d after second alertsMsg, got %d", firstCursor, m.sidebar.cursor)
    }
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/tui/... -run "TestFocusOnOpen" -v
```

Expected: FAIL — `startupFocusDone` field doesn't exist yet; `currentSession`/`currentWindow` not stored; focus logic not wired.

**Step 3: Add fields to Model**

In `internal/tui/model.go`, add to the `Model` struct after `popupMode`:

```go
currentSession  string
currentWindow   int
startupFocusDone bool
```

**Step 4: Wire panesMsg handler**

In the `panesMsg` case inside `Update`, inside the `!m.ready` block (just before `m.ready = true`), add:

```go
m.currentSession = msg.currentSession
m.currentWindow = msg.currentWindow
switch m.cfg.FocusOnOpen {
case "current_window":
    visibleRows := max(1, m.height-1-2)
    m.sidebar.FocusNode(m.currentSession, m.currentWindow, false, visibleRows)
case "current_session":
    visibleRows := max(1, m.height-1-2)
    m.sidebar.FocusNode(m.currentSession, m.currentWindow, true, visibleRows)
}
```

Place this block before `m.ready = true` so the sidebar data (set by `m.sidebar.SetData(...)` just above) is already populated.

**Step 5: Wire alertsMsg handler**

In the `alertsMsg` case, after `m.alerts = msg.alerts` and `m.sidebar.SetData(...)`, add:

```go
if !m.startupFocusDone {
    m.startupFocusDone = true
    visibleRows := max(1, m.height-1-2)
    switch m.cfg.FocusOnOpen {
    case "alert_window":
        m.sidebar.FocusFirstAlertWindow(visibleRows)
    case "alert_session":
        m.sidebar.FocusFirstAlertSession(visibleRows)
    }
}
```

**Step 6: Run the new tests**

```bash
go test ./internal/tui/... -run "TestFocusOnOpen" -v
```

Expected: PASS

**Step 7: Run full suite**

```bash
go test ./...
```

Expected: all pass

**Step 8: Commit**

```bash
git add internal/tui/model.go internal/tui/model_focus_test.go
git commit -m "feat(tui): wire focus_on_open startup behavior in model"
```
