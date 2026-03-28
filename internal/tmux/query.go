package tmux

import (
    "fmt"
    "os/exec"
    "strconv"
    "strings"
    "time"
)

type Pane struct {
    Session     string
    WindowIndex int
    PaneIndex   int
    CWD         string
    PaneID      string // e.g. %12
    WindowName  string
    PanePID         int32  // PID of the shell process running in this pane
    SessionActivity int64  // Unix timestamp from #{session_activity}
}

// ListPanes runs tmux list-panes and returns all panes across all sessions.
func ListPanes() ([]Pane, error) {
    out, err := exec.Command("tmux", "list-panes", "-a",
        "-F", "#{session_name}\t#{window_index}\t#{pane_index}\t#{pane_current_path}\t#{pane_id}\t#{window_name}\t#{pane_pid}\t#{session_activity}",
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
        if len(parts) < 4 {
            continue
        }
        wi, _ := strconv.Atoi(parts[1])
        pi, _ := strconv.Atoi(parts[2])
        p := Pane{
            Session:     parts[0],
            WindowIndex: wi,
            PaneIndex:   pi,
            CWD:         parts[3],
        }
        if len(parts) >= 5 {
            p.PaneID = parts[4]
        }
        if len(parts) >= 6 {
            p.WindowName = parts[5]
        }
        if len(parts) >= 7 {
            if pid, err := strconv.ParseInt(strings.TrimSpace(parts[6]), 10, 32); err == nil {
                p.PanePID = int32(pid)
            }
        }
        if len(parts) >= 8 {
            if ts, err := strconv.ParseInt(strings.TrimSpace(parts[7]), 10, 64); err == nil {
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

// SwitchClient runs tmux switch-client -t target.
func SwitchClient(target string) error {
    return exec.Command("tmux", "switch-client", "-t", target).Run()
}
