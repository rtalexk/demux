//go:build !windows

package cmd

import (
	"os"
	"path/filepath"
	"syscall"
)

// withFileLock runs fn while holding an exclusive advisory lock (flock) on the
// file at path, creating the file and its parent directory if needed. The lock
// is released when fn returns.
//
// Best-effort: if the directory or lock file cannot be created, or the lock
// cannot be taken, fn still runs (unguarded) rather than dropping the work.
func withFileLock(path string, fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fn()
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fn()
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fn()
	}
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }()
	return fn()
}
