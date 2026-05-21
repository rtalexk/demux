//go:build !windows

package cmd

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestWithFileLock_SerializesConcurrentCallers verifies the file lock makes
// concurrent callers run their critical sections one at a time. The
// after-kill-pane / pane-exited hooks run `run-shell -b`, which spawns handler
// processes concurrently; this lock is what keeps their sticky sweeps
// sequential. Without it, maxInside would climb toward the goroutine count.
func TestWithFileLock_SerializesConcurrentCallers(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "event.lock")

	var mu sync.Mutex
	inside, maxInside := 0, 0

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = withFileLock(lockPath, func() error {
				mu.Lock()
				inside++
				if inside > maxInside {
					maxInside = inside
				}
				mu.Unlock()

				time.Sleep(5 * time.Millisecond)

				mu.Lock()
				inside--
				mu.Unlock()
				return nil
			})
		}()
	}
	wg.Wait()

	if maxInside != 1 {
		t.Errorf("withFileLock allowed %d concurrent callers, want 1", maxInside)
	}
}

// TestWithFileLock_RunsFnAndReturnsItsError verifies the lock wrapper is
// transparent: it runs fn exactly once and propagates its return value.
func TestWithFileLock_RunsFnAndReturnsItsError(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "event.lock")

	calls := 0
	sentinel := errors.New("sentinel")
	err := withFileLock(lockPath, func() error {
		calls++
		return sentinel
	})
	if calls != 1 {
		t.Errorf("fn ran %d times, want 1", calls)
	}
	if err != sentinel {
		t.Errorf("withFileLock returned %v, want %v", err, sentinel)
	}
}
