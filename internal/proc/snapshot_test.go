package proc_test

import (
	"os"
	"runtime"
	"testing"

	"github.com/rtalexk/demux/internal/proc"
)

func TestSnapshot(t *testing.T) {
	procs, err := proc.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(procs) == 0 {
		t.Error("expected at least one process")
	}
	found := false
	for _, p := range procs {
		if p.PID == int32(os.Getpid()) {
			found = true
			break
		}
	}
	if !found {
		t.Error("current process not found in snapshot")
	}
}

func TestKill_InvalidPID(t *testing.T) {
	err := proc.Kill(-999)
	if err == nil {
		t.Error("expected error killing invalid PID, got nil")
	}
}

func TestEnviron_CurrentProcess(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("gopsutil Environ not implemented on macOS")
	}
	envs, err := proc.Environ(int32(os.Getpid()))
	if err != nil {
		t.Fatalf("Environ current process: %v", err)
	}
	if len(envs) == 0 {
		t.Error("expected at least one env var for current process")
	}
}

func TestEnviron_InvalidPID(t *testing.T) {
	_, err := proc.Environ(-999)
	if err == nil {
		t.Error("expected error for invalid PID, got nil")
	}
}

func TestBuildTree(t *testing.T) {
	procs := []proc.Process{
		{PID: 1, PPID: 0, Name: "init"},
		{PID: 10, PPID: 1, Name: "shell"},
		{PID: 20, PPID: 10, Name: "node"},
	}
	tree := proc.BuildTree(procs)
	children := tree[10]
	if len(children) != 1 || children[0].PID != 20 {
		t.Errorf("unexpected children of pid 10: %v", children)
	}
	if len(tree[1]) != 1 || tree[1][0].PID != 10 {
		t.Errorf("unexpected children of pid 1: %v", tree[1])
	}
}
