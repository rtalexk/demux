//go:build windows

package cmd

// withFileLock runs fn directly on Windows. The tmux event hooks that need
// cross-process serialization (after-kill-pane, pane-exited) only run under
// tmux, which is unix-only, so no lock is taken here.
func withFileLock(_ string, fn func() error) error {
	return fn()
}
