package cmd

import (
	"testing"

	"github.com/rtalexk/demux/internal/proc"
	"github.com/rtalexk/demux/internal/tmux"
)

func TestResolvePortMap(t *testing.T) {
	ports := []proc.PortInfo{
		{PID: 100, Port: 3000},
		{PID: 200, Port: 8080},
	}
	got := resolvePortMap(ports)
	if got[100] != 3000 {
		t.Errorf("pid 100: got %d, want 3000", got[100])
	}
	if got[200] != 8080 {
		t.Errorf("pid 200: got %d, want 8080", got[200])
	}
	if len(got) != 2 {
		t.Errorf("len: got %d, want 2", len(got))
	}
}

// pids returns the PIDs of procs, for comparison in attribution tests.
func pids(procs []proc.Process) []int32 {
	out := make([]int32, len(procs))
	for i, p := range procs {
		out[i] = p.PID
	}
	return out
}

func equalPIDs(got []int32, want ...int32) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func indexProcs(procs []proc.Process) (map[int32]proc.Process, map[int32][]proc.Process) {
	byPID := make(map[int32]proc.Process, len(procs))
	for _, p := range procs {
		byPID[p.PID] = p
	}
	return byPID, proc.BuildTree(procs)
}

// Processes are attributed to the pane whose tree they belong to, not to every
// pane sharing a CWD.
func TestPaneProcs_AttributesByProcessTree(t *testing.T) {
	pane := tmux.Pane{Session: "s", WindowIndex: 1, PaneIndex: 0, CWD: "/proj", PanePID: 200}
	procs := []proc.Process{
		{PID: 100, PPID: 1, Name: "zsh"},
		{PID: 101, PPID: 100, Name: "nvim"},
		{PID: 200, PPID: 1, Name: "zsh"},
		{PID: 201, PPID: 200, Name: "opencode"},
	}
	cwdByPID := map[int32]string{100: "/proj", 101: "/proj", 200: "/proj", 201: "/proj"}
	byPID, tree := indexProcs(procs)
	got := pids(paneProcs(pane, procs, byPID, tree, cwdByPID))
	if !equalPIDs(got, 200, 201) {
		t.Fatalf("expected pids [200 201], got %v", got)
	}
}

// Panes running a command directly (no shell wrapper) list that command.
func TestPaneProcs_PaneRootProcess(t *testing.T) {
	pane := tmux.Pane{Session: "s", WindowIndex: 2, PaneIndex: 0, CWD: "/proj", PanePID: 300}
	procs := []proc.Process{{PID: 300, PPID: 1, Name: "opencode"}}
	byPID, tree := indexProcs(procs)
	got := pids(paneProcs(pane, procs, byPID, tree, map[int32]string{}))
	if !equalPIDs(got, 300) {
		t.Fatalf("expected pid [300], got %v", got)
	}
}

// With no pane PID the CWD heuristic still applies.
func TestPaneProcs_NoPanePID_FallsBackToCWD(t *testing.T) {
	pane := tmux.Pane{Session: "s", WindowIndex: 0, PaneIndex: 0, CWD: "/proj"}
	procs := []proc.Process{
		{PID: 400, PPID: 1, Name: "nvim"},
		{PID: 401, PPID: 1, Name: "other"},
	}
	cwdByPID := map[int32]string{400: "/proj", 401: "/elsewhere"}
	byPID, tree := indexProcs(procs)
	got := pids(paneProcs(pane, procs, byPID, tree, cwdByPID))
	if !equalPIDs(got, 400) {
		t.Fatalf("expected pid [400], got %v", got)
	}
}

// buildPaneRows emits a header row followed by one row per pane process.
func TestBuildPaneRows_HeaderPlusProcessRows(t *testing.T) {
	pane := tmux.Pane{Session: "s", WindowIndex: 1, PaneIndex: 0, CWD: "/proj", PanePID: 200}
	procs := []proc.Process{{PID: 201, PPID: 200, Name: "opencode"}}
	rows := buildPaneRows(pane, 1, "s", procs, map[int32]int{}, "—", false)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if f := rows[0].Fields(); f[3] != "(pane)" {
		t.Errorf("expected header row, got %v", f)
	}
	if f := rows[1].Fields(); f[3] != "opencode" || f[4] != "201" {
		t.Errorf("expected opencode/201 row, got %v", f)
	}
}
