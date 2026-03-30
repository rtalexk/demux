package proc_test

import (
    "os"
    "testing"

    "github.com/rtalex/demux/internal/proc"
)

func TestCWDCurrentProcess(t *testing.T) {
    pid := int32(os.Getpid())
    cwd, err := proc.CWD(pid)
    if err != nil {
        t.Skipf("CWD not resolvable on this platform: %v", err)
    }
    if cwd == "" {
        t.Error("expected non-empty cwd")
    }
}

func TestCWDAll_ContainsSelf(t *testing.T) {
    m, err := proc.CWDAll()
    if err != nil {
        t.Skipf("CWDAll not available: %v", err)
    }
    pid := int32(os.Getpid())
    cwd, ok := m[pid]
    if !ok {
        t.Fatalf("own PID %d not in CWDAll result", pid)
    }
    if cwd == "" {
        t.Error("expected non-empty cwd for own PID")
    }
}

func BenchmarkCWDAll(b *testing.B) {
    for b.Loop() {
        if _, err := proc.CWDAll(); err != nil {
            b.Fatal(err)
        }
    }
}
