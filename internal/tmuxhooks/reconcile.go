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

var _ = demuxlog.Warn // referenced by Reconcile in the next task
