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
