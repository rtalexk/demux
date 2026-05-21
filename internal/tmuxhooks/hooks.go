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

// DesiredHooks returns demux's full canonical hook set. The client-attached
// bootstrap hook is intentionally excluded: it is owned by the demux managed
// block in ~/.tmux.conf and is what triggers reconciliation, so the
// reconciler must never manage it.
func DesiredHooks() []Hook {
	const paneFocus = `run-shell 'demux event pane_focus --pane-id=#{pane_id} --window-id=#{window_id} --session-id=#{session_id} 2>/dev/null; true'`
	const follow = `run-shell 'demux sidebar follow 2>/dev/null; true'`
	return []Hook{
		{Event: "after-select-pane", Command: paneFocus},
		{Event: "after-select-window", Command: paneFocus},
		{Event: "after-select-window", Command: follow},
		{Event: "client-session-changed", Command: paneFocus},
		{Event: "client-session-changed", Command: follow},
		{Event: "client-focus-in", Command: paneFocus},
		{Event: "after-kill-pane", Command: `run-shell -b 'demux event pane_closed --pane=#{hook_pane} --window=#{hook_window} 2>/dev/null; true'`},
		{Event: "pane-exited", Command: `run-shell -b 'demux event pane_exiting --pane=#{hook_pane} --window=#{hook_window} 2>/dev/null; true'`},
		{Event: "after-new-window", Command: `run-shell -b 'demux sidebar follow 2>/dev/null; demux sidebar slots ensure 2>/dev/null; true'`},
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
