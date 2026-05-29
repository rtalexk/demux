// Package tmuxhooks defines demux's tmux hook set and keeps a running tmux
// server registered with it. The hook definitions live here as structured
// data so the runtime reconciler and `demux hooks install` share one source
// of truth.
package tmuxhooks

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Hook is one demux-owned tmux hook entry: a tmux hook event and the
// run-shell command registered on it.
type Hook struct {
	Event   string
	Command string
}

// DesiredHooks returns demux's full canonical hook set. Each entry calls a
// `demux event …` command that names the tmux event; the event handler decides
// the reaction (clear done-states, move the sidebar, reconcile slot panes). The
// client-attached bootstrap hook is intentionally excluded: it is owned by the
// demux managed block in ~/.tmux.conf and is what triggers reconciliation, so
// the reconciler must never manage it.
func DesiredHooks() []Hook {
	// pane_focus is backgrounded (-b). It fires on after-select-pane and
	// client-focus-in, which tmux raises at the very start of a mouse scroll or
	// drag (the first interaction selects/focuses the pane). A non-backgrounded
	// run-shell blocks the tmux server's command queue until `demux event`
	// returns, so the DB work (which can stall on the SQLite lock held by the
	// frequent state-writer hook) freezes tmux mid-redraw and tears the
	// copy-mode/selection paint. pane_focus does pure DB work with no tmux
	// mutation or cross-hook ordering, so backgrounding it is safe.
	const paneFocus = `run-shell -b 'demux event pane_focus --pane-id=#{pane_id} --window-id=#{window_id} --session-id=#{session_id} 2>/dev/null; true'`
	// window_focus and session_changed are likewise backgrounded (-b). They fire
	// on window/session switch and, like pane_focus, a synchronous run-shell here
	// blocks the tmux server's command queue until `demux event` returns (DB work
	// that can stall on the SQLite lock), freezing tmux on every switch. The
	// handler serializes itself with withEventLock so backgrounded invocations
	// cannot race each other's sidebar moves.
	const windowFocus = `run-shell -b 'demux event window_focus --pane-id=#{pane_id} --window-id=#{window_id} --session-id=#{session_id} 2>/dev/null; true'`
	const sessionChanged = `run-shell -b 'demux event session_changed --pane-id=#{pane_id} --window-id=#{window_id} --session-id=#{session_id} 2>/dev/null; true'`
	return []Hook{
		{Event: "after-select-pane", Command: paneFocus},
		{Event: "after-select-window", Command: windowFocus},
		{Event: "client-session-changed", Command: sessionChanged},
		{Event: "client-focus-in", Command: paneFocus},
		{Event: "after-kill-pane", Command: `run-shell -b 'demux event pane_closed --pane=#{hook_pane} --window=#{hook_window} 2>/dev/null; true'`},
		{Event: "pane-exited", Command: `run-shell -b 'demux event pane_exiting --pane=#{hook_pane} --window=#{hook_window} 2>/dev/null; true'`},
		{Event: "after-new-window", Command: `run-shell -b 'demux event new_window 2>/dev/null; true'`},
		{Event: "window-unlinked", Command: `run-shell -b 'demux event window_closed --window=#{hook_window} 2>/dev/null; true'`},
		{Event: "session-closed", Command: `run-shell -b 'demux event session_closed --session=#{hook_session} 2>/dev/null; true'`},
	}
}

// HooksHash returns a stable 12-hex-char hash of the desired hook set. It
// changes exactly when DesiredHooks changes, so an upgraded demux binary
// triggers exactly one reconcile. Stored in the @demux_hooks_version tmux
// option as the staleness key.
func HooksHash() string {
	var b strings.Builder
	for _, h := range DesiredHooks() {
		b.WriteString(h.Event)
		b.WriteByte('\n')
		b.WriteString(h.Command)
		b.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])[:12]
}
