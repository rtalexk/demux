package tmux

import (
    "fmt"
    "os"
    "os/exec"
)

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
