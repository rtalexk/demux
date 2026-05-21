package tmuxhooks

import (
	"regexp"
	"strings"

	demuxlog "github.com/rtalexk/demux/internal/log"
)

// hookLineRE matches one line of `tmux show-hooks -g` output, e.g.
//
//	after-new-window[0] run-shell -b "demux ..."
var hookLineRE = regexp.MustCompile(`^([a-zA-Z-]+)\[\d+\]\s+(.*)$`)

// parseShowHooks groups `tmux show-hooks -g` output by event name, preserving
// per-event order. The value is the list of hook command strings.
func parseShowHooks(out string) map[string][]string {
	result := map[string][]string{}
	for _, line := range strings.Split(out, "\n") {
		m := hookLineRE.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if m == nil {
			continue
		}
		result[m[1]] = append(result[m[1]], m[2])
	}
	return result
}

// isDemuxEntry reports whether a hook command was registered by demux.
func isDemuxEntry(cmd string) bool {
	return strings.Contains(cmd, "demux event ") ||
		strings.Contains(cmd, "demux sidebar ")
}

const versionOption = "@demux_hooks_version"

// Reconcile registers demux's desired hook set into the running tmux server.
// For each event it clears the hook array, then re-adds the user's own
// (non-demux) entries followed by demux's canonical entries — so the result is
// idempotent and can never stack duplicates. Per-event errors are logged and
// skipped (e.g. a tmux build without the pane-exited hook) so one unsupported
// event never aborts the rest.
func Reconcile(t Tmux) error {
	out, err := t.Output("show-hooks", "-g")
	if err != nil {
		return err
	}
	current := parseShowHooks(out)

	desired := map[string][]string{}
	var events []string
	for _, h := range DesiredHooks() {
		if _, seen := desired[h.Event]; !seen {
			events = append(events, h.Event)
		}
		desired[h.Event] = append(desired[h.Event], h.Command)
	}

	for _, event := range events {
		var keep []string
		for _, cmd := range current[event] {
			if !isDemuxEntry(cmd) {
				keep = append(keep, cmd) // preserve the user's own hooks
			}
		}
		newEntries := append(keep, desired[event]...)

		if err := t.Run("set-hook", "-gu", event); err != nil {
			demuxlog.Warn("tmuxhooks: clear hook failed", "event", event, "err", err)
			continue
		}
		for _, cmd := range newEntries {
			if err := t.Run("set-hook", "-ga", event, cmd); err != nil {
				demuxlog.Warn("tmuxhooks: set hook failed", "event", event, "err", err)
			}
		}
	}
	return nil
}

// CurrentVersion returns the @demux_hooks_version tmux option, or "" if unset.
func CurrentVersion(t Tmux) string {
	out, err := t.Output("show", "-gv", versionOption)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// MarkVersion records the current desired-hook hash in @demux_hooks_version.
func MarkVersion(t Tmux) error {
	return t.Run("set", "-g", versionOption, HooksHash())
}

// Sync reconciles hooks only when the recorded version differs from the
// current desired-hook hash. It is the fast path for the client-attached
// bootstrap: a single tmux query when nothing changed.
func Sync(t Tmux) error {
	if CurrentVersion(t) == HooksHash() {
		return nil
	}
	if err := Reconcile(t); err != nil {
		return err
	}
	return MarkVersion(t)
}
