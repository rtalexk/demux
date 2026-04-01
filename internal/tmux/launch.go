package tmux

import (
    "fmt"
    "os"
    "os/exec"
)

// WindowSpec describes a window to create inside a tmux session.
type WindowSpec struct {
    Name           string
    AfterCreateCmd string
}

// NewSession creates a detached tmux session named `name` rooted at `path`,
// then switches the client to it. Returns an error if name or path is empty,
// or if the path does not exist on disk.
func NewSession(name, path string) error {
    if name == "" {
        return fmt.Errorf("session name is required")
    }
    if path == "" {
        return fmt.Errorf("session path is required")
    }
    if _, err := os.Stat(path); err != nil {
        return fmt.Errorf("session path %q: %w", path, err)
    }
    if err := exec.Command("tmux", "new-session", "-d", "-s", name, "-c", path).Run(); err != nil {
        return fmt.Errorf("tmux new-session: %w", err)
    }
    if err := exec.Command("tmux", "switch-client", "-t", name).Run(); err != nil {
        return fmt.Errorf("tmux switch-client: %w", err)
    }
    return nil
}

// CreateSessionWindows configures windows for an existing tmux session.
// The session already has one default window (index 0); this renames it to
// windows[0].Name and sends its AfterCreateCmd, then creates additional
// windows for windows[1:]. A nil or empty list is a no-op.
func CreateSessionWindows(sessionName string, windows []WindowSpec) error {
    for i, w := range windows {
        if i == 0 {
            if err := exec.Command("tmux", "rename-window", "-t", sessionName+":0", w.Name).Run(); err != nil {
                return fmt.Errorf("tmux rename-window %q: %w", w.Name, err)
            }
        } else {
            if err := exec.Command("tmux", "new-window", "-t", sessionName, "-n", w.Name).Run(); err != nil {
                return fmt.Errorf("tmux new-window %q: %w", w.Name, err)
            }
            if err := exec.Command("tmux", "rename-window", "-t", fmt.Sprintf("%s:%d", sessionName, i), w.Name).Run(); err != nil {
                return fmt.Errorf("tmux rename-window %q: %w", w.Name, err)
            }
        }
        if w.AfterCreateCmd != "" {
            target := fmt.Sprintf("%s:%d", sessionName, i)
            if err := exec.Command("tmux", "send-keys", "-t", target, w.AfterCreateCmd, "Enter").Run(); err != nil {
                return fmt.Errorf("tmux send-keys %q: %w", w.Name, err)
            }
        }
    }
    return nil
}
