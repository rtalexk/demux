package sticky

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	demuxlog "github.com/rtalexk/demux/internal/log"
)

// splitSidebarPane runs the standard sidebar split-window invocation
// (full-height, horizontal, before target, detached, exact width) and then
// rebalances pane widths so the sidebar's footprint is absorbed by the
// pane(s) immediately adjacent to it - not by tmux's default victim, which
// for a left-side `-b` split is typically the rightmost pane.
//
// Without this rebalance, repeatedly opening + closing the sidebar ratchets
// the rightmost pane smaller on every cycle: open shrinks the rightmost
// (tmux default), close grows the leftmost-adjacent (also tmux default on
// pane removal). The asymmetry compounds. With this rebalance both directions
// move width through the same neighbour, so cycling open/close is stable.
//
// Algorithm: snapshot each pane's pane_left + pane_width before the split,
// run split-window, then resize every original pane (rightmost-first by
// pane_left) back to its recorded width - skipping the pane(s) in the
// originally-leftmost column, which absorb the sidebar's width. Iterating
// rightmost-first lets the width deficit propagate one neighbour at a time
// until it lands on the adjacent column.
//
// Returns the new pane id.
func (s *Sticky) splitSidebarPane(windowTarget string, width int, cmd string) (string, error) {
	// Best-effort: if geometry can't be read, skip rebalance but still split.
	// The rebalance is a cosmetic adjustment; failing to perform it must not
	// block the user from opening the sidebar.
	pre, minLeft, geomErr := s.listPaneGeometry(windowTarget)
	if geomErr != nil {
		demuxlog.Debug("splitSidebarPane: geometry read failed, skipping rebalance", "target", windowTarget, "err", geomErr)
	}

	out, err := s.T.Output(
		"split-window", "-f", "-h", "-b", "-d",
		"-l", strconv.Itoa(width),
		"-t", windowTarget,
		"-P", "-F", "#{pane_id}",
		cmd,
	)
	if err != nil {
		return "", fmt.Errorf("tmux split-window: %w", err)
	}
	newPane := strings.TrimSpace(out)
	if newPane == "" {
		return "", fmt.Errorf("tmux split-window returned empty pane id")
	}

	// Rightmost-first so each resize takes from its left neighbour, walking the
	// deficit toward the leftmost-original column.
	sort.SliceStable(pre, func(i, j int) bool { return pre[i].left > pre[j].left })
	for _, p := range pre {
		if p.left == minLeft {
			continue
		}
		_ = s.T.Run("resize-pane", "-t", p.id, "-x", strconv.Itoa(p.width))
	}
	return newPane, nil
}

type paneGeometry struct {
	id    string
	left  int
	width int
}

// listPaneGeometry returns one entry per pane in windowTarget plus the
// smallest pane_left value (the originally-leftmost column). Lines that
// don't parse cleanly are skipped; a window with no parseable panes returns
// (nil, 0, nil) and callers should treat that as "nothing to rebalance."
func (s *Sticky) listPaneGeometry(windowTarget string) ([]paneGeometry, int, error) {
	out, err := s.T.Output("list-panes", "-t", windowTarget, "-F", "#{pane_id} #{pane_left} #{pane_width}")
	if err != nil {
		return nil, 0, fmt.Errorf("tmux list-panes: %w", err)
	}
	var panes []paneGeometry
	minLeft := -1
	for _, line := range nonEmptyLines(out) {
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		left, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}
		w, err := strconv.Atoi(parts[2])
		if err != nil {
			continue
		}
		panes = append(panes, paneGeometry{id: parts[0], left: left, width: w})
		if minLeft < 0 || left < minLeft {
			minLeft = left
		}
	}
	if minLeft < 0 {
		minLeft = 0
	}
	return panes, minLeft, nil
}
