package tmuxhooks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	markerBegin = "# >>> demux managed block >>>"
	markerEnd   = "# <<< demux managed block <<<"

	// bootstrapCommand is the run-shell payload of the bootstrap hook.
	bootstrapCommand = `run-shell -b 'demux event client_attached 2>/dev/null; true'`
)

// BootstrapEvent is the tmux hook event demux's bootstrap line registers.
const BootstrapEvent = "client-attached"

// BootstrapHook is the single set-hook line demux's managed block installs in
// ~/.tmux.conf. It fires demux's reconciler on every client attach.
var BootstrapHook = fmt.Sprintf("set-hook -g %s %q", BootstrapEvent, bootstrapCommand)

// BootstrapCommand returns the run-shell payload for live registration via
// `tmux set-hook -g client-attached <payload>`.
func BootstrapCommand() string { return bootstrapCommand }

// managedBlock returns the full marker-delimited block text (no trailing
// newline).
func managedBlock() string {
	return markerBegin + "\n" + BootstrapHook + "\n" + markerEnd
}

// ConfPath returns the path to the user's tmux config (~/.tmux.conf).
func ConfPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".tmux.conf"), nil
}

// LoadConf reads the tmux config at path, following a symlink to its real
// target. It returns the file content, the real (symlink-resolved) path to
// write back to, and whether the file existed.
func LoadConf(path string) (content, realPath string, existed bool, err error) {
	realPath = path
	if target, rerr := os.Readlink(path); rerr == nil {
		if filepath.IsAbs(target) {
			realPath = target
		} else {
			realPath = filepath.Join(filepath.Dir(path), target)
		}
	}
	b, rerr := os.ReadFile(realPath)
	if rerr != nil {
		if os.IsNotExist(rerr) {
			return "", realPath, false, nil
		}
		return "", realPath, false, fmt.Errorf("read %s: %w", realPath, rerr)
	}
	return string(b), realPath, true, nil
}

// WithManagedBlock returns content with demux's managed block inserted. If the
// markers already exist the block is replaced in place; otherwise it is
// appended with one blank line of separation.
func WithManagedBlock(content string) string {
	begin := strings.Index(content, markerBegin)
	end := strings.Index(content, markerEnd)
	if begin >= 0 && end > begin {
		return content[:begin] + managedBlock() + content[end+len(markerEnd):]
	}
	trimmed := strings.TrimRight(content, "\n")
	if trimmed == "" {
		return managedBlock() + "\n"
	}
	return trimmed + "\n\n" + managedBlock() + "\n"
}

// SaveConf writes content to realPath atomically (temp file + rename) after
// backing up the existing file to <realPath>.demux-bak.
func SaveConf(realPath, content string) error {
	if existing, err := os.ReadFile(realPath); err == nil {
		if err := os.WriteFile(realPath+".demux-bak", existing, 0o644); err != nil {
			return fmt.Errorf("write backup: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read for backup: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(realPath), ".demux-tmuxconf-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, realPath); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename temp: %w", err)
	}
	return nil
}
