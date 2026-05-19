package tmux

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type Pane struct {
	Session         string
	SessionID       string // $N — new
	WindowIndex     int
	WindowID        string // @N — new
	PaneIndex       int
	CWD             string
	PaneID          string // e.g. %12
	WindowName      string
	PanePID         int32 // PID of the shell process running in this pane
	SessionActivity int64 // Unix timestamp from #{session_activity}
}

// DisplayLabel returns the "session:windowIndex.paneIndex" string used as a human-readable display label.
func (p Pane) DisplayLabel() string {
	return fmt.Sprintf("%s:%d.%d", p.Session, p.WindowIndex, p.PaneIndex)
}

// ListPanes runs tmux list-panes and returns all panes across all sessions.
func ListPanes() ([]Pane, error) {
	out, err := exec.Command("tmux", "list-panes", "-a",
		"-F", "#{session_name}\t#{session_id}\t#{window_index}\t#{window_id}\t#{pane_index}\t#{pane_current_path}\t#{pane_id}\t#{window_name}\t#{pane_pid}\t#{session_activity}",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("tmux list-panes: %w", err)
	}
	return ParsePanes(string(out))
}

func ParsePanes(raw string) ([]Pane, error) {
	var panes []Pane
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 6 {
			continue
		}
		wi, _ := strconv.Atoi(parts[2])
		pi, _ := strconv.Atoi(parts[4])
		p := Pane{
			Session:     parts[0],
			SessionID:   parts[1],
			WindowIndex: wi,
			WindowID:    parts[3],
			PaneIndex:   pi,
			CWD:         parts[5],
		}
		if len(parts) >= 7 {
			p.PaneID = parts[6]
		}
		if len(parts) >= 8 {
			p.WindowName = parts[7]
		}
		if len(parts) >= 9 {
			if pid, err := strconv.ParseInt(strings.TrimSpace(parts[8]), 10, 32); err == nil {
				p.PanePID = int32(pid)
			}
		}
		if len(parts) >= 10 {
			if ts, err := strconv.ParseInt(strings.TrimSpace(parts[9]), 10, 64); err == nil {
				p.SessionActivity = ts
			}
		}
		panes = append(panes, p)
	}
	return panes, nil
}

// GroupBySessions organises panes into a map[session]map[windowIndex][]Pane.
func GroupBySessions(panes []Pane) map[string]map[int][]Pane {
	m := make(map[string]map[int][]Pane)
	for _, p := range panes {
		if m[p.Session] == nil {
			m[p.Session] = make(map[int][]Pane)
		}
		m[p.Session][p.WindowIndex] = append(m[p.Session][p.WindowIndex], p)
	}
	return m
}

// SessionActivityMap returns the most recent session_activity timestamp per
// session name, derived from the already-fetched pane list.
func SessionActivityMap(panes []Pane) map[string]time.Time {
	m := make(map[string]time.Time, len(panes))
	for _, p := range panes {
		if p.SessionActivity <= 0 {
			continue
		}
		t := time.Unix(p.SessionActivity, 0)
		if existing, ok := m[p.Session]; !ok || t.After(existing) {
			m[p.Session] = t
		}
	}
	return m
}

// ParseCurrentTarget parses the output of `tmux display-message -p "#{session_name}\t#{window_index}"`.
// Empty input is treated as an error because a real tmux invocation only produces empty output
// in degenerate states; the caller (CurrentTarget) surfaces the exec error before this is reached.
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

// SwitchClient runs tmux switch-client -t target.
func SwitchClient(target string) error {
	return exec.Command("tmux", "switch-client", "-t", target).Run()
}

// SelectNextPane runs tmux select-pane -t :.+ which is the same action as
// tmux's default `prefix + o` binding: move focus to the next pane in the
// current window. Used in sticky mode to jump out of the sidebar pane after
// the user attaches to a session.
func SelectNextPane() error {
	return exec.Command("tmux", "select-pane", "-t", ":.+").Run()
}

// PaneIDToTargetMap returns a map from pane_id (%N) to display label "session:windowIndex.paneIndex"
// for all panes with a non-empty PaneID in the provided slice.
func PaneIDToTargetMap(panes []Pane) map[string]string {
	m := make(map[string]string, len(panes))
	for _, p := range panes {
		if p.PaneID != "" {
			m[p.PaneID] = p.DisplayLabel()
		}
	}
	return m
}

// WindowIDToTargetMap returns a map from window_id (@N) to display label "session:windowIndex"
// for all panes with a non-empty WindowID in the provided slice.
func WindowIDToTargetMap(panes []Pane) map[string]string {
	m := make(map[string]string, len(panes))
	for _, p := range panes {
		if p.WindowID != "" {
			m[p.WindowID] = fmt.Sprintf("%s:%d", p.Session, p.WindowIndex)
		}
	}
	return m
}

// SessionIDToNameMap returns a map from session_id ($N) to session_name
// for all panes with a non-empty SessionID in the provided slice.
func SessionIDToNameMap(panes []Pane) map[string]string {
	m := make(map[string]string, len(panes))
	for _, p := range panes {
		if p.SessionID != "" && m[p.SessionID] == "" {
			m[p.SessionID] = p.Session
		}
	}
	return m
}

// SessionNameToIDMap returns a map from session_name to session_id ($N)
// for all panes with a non-empty SessionID in the provided slice.
func SessionNameToIDMap(panes []Pane) map[string]string {
	m := make(map[string]string, len(panes))
	for _, p := range panes {
		if p.SessionID != "" && m[p.Session] == "" {
			m[p.Session] = p.SessionID
		}
	}
	return m
}
