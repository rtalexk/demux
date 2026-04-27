package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// WindowSpec describes a window to create inside a tmux session.
type WindowSpec struct {
	Name           string
	AfterCreateCmd string
}

// NewSessionDetached creates a detached tmux session named `name` rooted at
// `path`. It does NOT switch or attach the client. Returns an error if name
// or path is empty, or if the path does not exist on disk.
func NewSessionDetached(name, path string) error {
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
	return nil
}

// Connect attaches the current process to the named tmux session, picking
// the right verb based on $TMUX:
//   - inside tmux ($TMUX set): runs `tmux switch-client -t name`.
//   - outside tmux: replaces the current process with `tmux attach-session -t name`
//     via syscall.Exec, so the user lands directly in tmux.
func Connect(name string) error {
	insideTmux := os.Getenv("TMUX") != ""
	args := resolveConnectArgs(insideTmux, name)
	if insideTmux {
		if err := exec.Command("tmux", args...).Run(); err != nil {
			return fmt.Errorf("tmux %s: %w", args[0], err)
		}
		return nil
	}
	tmuxPath, err := exec.LookPath("tmux")
	if err != nil {
		return fmt.Errorf("tmux not found in PATH: %w", err)
	}
	argv := append([]string{"tmux"}, args...)
	if err := syscall.Exec(tmuxPath, argv, os.Environ()); err != nil {
		return fmt.Errorf("tmux attach-session: %w", err)
	}
	return nil
}

// resolveConnectArgs returns the tmux subcommand and args for Connect,
// based on whether the caller is currently inside a tmux client.
func resolveConnectArgs(insideTmux bool, name string) []string {
	if insideTmux {
		return []string{"switch-client", "-t", name}
	}
	return []string{"attach-session", "-t", name}
}

// CreateSessionWindows configures windows for an existing tmux session.
// The session already has one default window (index 0); this renames it to
// windows[0].Name and sends its AfterCreateCmd, then creates additional
// windows for windows[1:]. path is the starting directory for new windows;
// when non-empty it is passed as -c to tmux new-window. A nil or empty list is a no-op.
func CreateSessionWindows(sessionName, path string, windows []WindowSpec) error {
	for i, w := range windows {
		if i == 0 {
			if err := exec.Command("tmux", "rename-window", "-t", sessionName+":0", w.Name).Run(); err != nil {
				return fmt.Errorf("tmux rename-window %q: %w", w.Name, err)
			}
		} else {
			if err := exec.Command("tmux", "new-window", "-t", sessionName, "-n", w.Name, "-c", path).Run(); err != nil {
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
