# Design: `focus_on_open` setting

## Overview

Add a `focus_on_open` config setting that controls which sidebar node is selected when Demux opens. The four modes cover two axes: level (window vs session) and target (current tmux active vs first with an alert).

## Config

**File:** `internal/config/config.go`

Add `FocusOnOpen string` to `Config`, default `"current_window"`.

```toml
focus_on_open = "current_window"  # current_window | current_session | alert_window | alert_session
```

| Value | Behaviour |
|---|---|
| `current_window` | Cursor on the currently active tmux window node (default) |
| `current_session` | Cursor on the currently active tmux session node |
| `alert_window` | Cursor on the first window node with an alert (highest severity, then newest) |
| `alert_session` | Cursor on the session node of the first alerted session |

## tmux package

**File:** `internal/tmux/query.go`

Add `CurrentTarget() (session string, windowIndex int, err error)` using:

```
tmux display-message -p "#{session_name}\t#{window_index}"
```

Include result in `panesMsg`:

```go
type panesMsg struct {
    panes          []tmux.Pane
    currentSession string
    currentWindow  int
}
```

`fetchPanes()` calls `tmux.CurrentTarget()` alongside `tmux.ListPanes()` and populates both fields.

## SidebarModel

**File:** `internal/tui/sidebar.go`

Add two methods:

- `FocusNode(session string, windowIndex int, isSessionLevel bool, visibleRows int)` — finds the matching node by session+window (or session only for session-level), sets `s.cursor`, clamps viewport.
- `FocusFirstAlertSession(visibleRows int)` — walks nodes to find the first session node that has any alert, sets cursor there.

The `alert_window` path reuses the existing first-alert-window logic currently inlined in `ToggleAlertFilter`, extracted into a private `focusFirstAlertWindow(visibleRows int)` helper called by both.

## Model

**File:** `internal/tui/model.go`

New fields on `Model`:

```go
currentSession  string
currentWindow   int
startupFocusDone bool
```

**`panesMsg` handler (first-load block, `!m.ready`):**
- Store `msg.currentSession` and `msg.currentWindow`.
- If `cfg.FocusOnOpen` is `current_window` or `current_session`: call `m.sidebar.FocusNode(...)` with `isSessionLevel = (mode == "current_session")`.

**`alertsMsg` handler:**
- If `!m.startupFocusDone`: set `m.startupFocusDone = true`, then:
  - `alert_window` → `m.sidebar.FocusFirstAlertWindow(visibleRows)`
  - `alert_session` → `m.sidebar.FocusFirstAlertSession(visibleRows)`
  - No-op if no alerts exist.
- `visibleRows` is computed as `m.height - 1 - 2` (same formula used elsewhere in the model).

## Error handling

- If `CurrentTarget()` fails (e.g. demux launched outside tmux), `currentSession` is empty. `FocusNode` with an empty session finds no match and leaves the cursor at 0 — same as current behaviour.
- If no alerts exist when `alert_*` mode is active, cursor stays at 0.
