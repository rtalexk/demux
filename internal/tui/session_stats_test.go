package tui

import (
	"testing"

	"github.com/rtalexk/demux/internal/proc"
	"github.com/rtalexk/demux/internal/tmux"
)

func TestComputeSessionTotals_SumsPaneTrees(t *testing.T) {
	// pane A shell pid 100 -> child 101 (cpu 10, mem 200MB)
	// pane B shell pid 200 -> child 201 (cpu 5,  mem 100MB)
	procs := []proc.Process{
		{PID: 100, PPID: 1, CPU: 1, MemRSS: 10 * 1024 * 1024, Name: "zsh"},
		{PID: 101, PPID: 100, CPU: 10, MemRSS: 200 * 1024 * 1024, Name: "node"},
		{PID: 200, PPID: 1, CPU: 1, MemRSS: 10 * 1024 * 1024, Name: "zsh"},
		{PID: 201, PPID: 200, CPU: 5, MemRSS: 100 * 1024 * 1024, Name: "go"},
	}
	panes := []tmux.Pane{
		{Session: "s", PanePID: 100},
		{Session: "s", PanePID: 200},
	}
	tree := proc.BuildTree(procs)
	byPID := indexByPID(procs)

	cpu, mem, caf := computeSessionTotals(panes, byPID, tree)
	if cpu != 17 { // 1+10+1+5
		t.Errorf("cpu = %v, want 17", cpu)
	}
	if mem != 320*1024*1024 {
		t.Errorf("mem = %v, want 320MB", mem)
	}
	if caf {
		t.Errorf("caffeinated = true, want false")
	}
}

func TestComputeSessionTotals_DetectsCaffeinate(t *testing.T) {
	procs := []proc.Process{
		{PID: 100, PPID: 1, CPU: 1, MemRSS: 1, Name: "zsh"},
		{PID: 101, PPID: 100, CPU: 0, MemRSS: 1, Name: "caffeinate"},
	}
	panes := []tmux.Pane{{Session: "s", PanePID: 100}}
	tree := proc.BuildTree(procs)
	byPID := indexByPID(procs)

	_, _, caf := computeSessionTotals(panes, byPID, tree)
	if !caf {
		t.Errorf("caffeinated = false, want true (caffeinate under pane)")
	}
}

func TestComputeSessionTotals_IgnoresCaffeinateOutsidePaneTree(t *testing.T) {
	// caffeinate (pid 900) is NOT a descendant of any pane shell.
	procs := []proc.Process{
		{PID: 100, PPID: 1, CPU: 1, MemRSS: 1, Name: "zsh"},
		{PID: 900, PPID: 1, CPU: 0, MemRSS: 1, Name: "caffeinate"},
	}
	panes := []tmux.Pane{{Session: "s", PanePID: 100}}
	tree := proc.BuildTree(procs)
	byPID := indexByPID(procs)

	_, _, caf := computeSessionTotals(panes, byPID, tree)
	if caf {
		t.Errorf("caffeinated = true, want false (caffeinate outside pane tree)")
	}
}

func TestHumanizeBytes(t *testing.T) {
	cases := []struct {
		in   uint64
		want string
	}{
		{320 * 1024 * 1024, "320MB"},
		{1024 * 1024 * 1024, "1.0GB"},
		{1536 * 1024 * 1024, "1.5GB"},
		{0, "0MB"},
	}
	for _, c := range cases {
		if got := humanizeBytes(c.in); got != c.want {
			t.Errorf("humanizeBytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSessionStatsModel_RollingPeakDecays(t *testing.T) {
	var m SessionStatsModel
	mkProcs := func(cpu float64) []proc.Process {
		return []proc.Process{{PID: 100, PPID: 1, CPU: cpu, MemRSS: 1, Name: "zsh"}}
	}
	panes := []tmux.Pane{{Session: "s", PanePID: 100}}

	m.Update(panes, mkProcs(50)) // spike
	for i := 0; i < statsPeakWindow; i++ {
		m.Update(panes, mkProcs(10))
	}
	st := m.Snapshot()["s"]
	if st.CPUNow != 10 {
		t.Errorf("CPUNow = %v, want 10", st.CPUNow)
	}
	if st.CPUPeak != 10 {
		t.Errorf("CPUPeak = %v, want 10 (50 should have decayed out of the %d-sample window)", st.CPUPeak, statsPeakWindow)
	}
}

func TestSessionStatsModel_PeakHoldsWithinWindow(t *testing.T) {
	var m SessionStatsModel
	panes := []tmux.Pane{{Session: "s", PanePID: 100}}
	m.Update(panes, []proc.Process{{PID: 100, PPID: 1, CPU: 50, MemRSS: 1, Name: "zsh"}})
	m.Update(panes, []proc.Process{{PID: 100, PPID: 1, CPU: 10, MemRSS: 1, Name: "zsh"}})
	st := m.Snapshot()["s"]
	if st.CPUPeak != 50 {
		t.Errorf("CPUPeak = %v, want 50 (still within window)", st.CPUPeak)
	}
	if st.CPUNow != 10 {
		t.Errorf("CPUNow = %v, want 10", st.CPUNow)
	}
}

func TestSessionStatsModel_PrunesVanishedSessions(t *testing.T) {
	var m SessionStatsModel
	m.Update([]tmux.Pane{{Session: "gone", PanePID: 100}},
		[]proc.Process{{PID: 100, PPID: 1, CPU: 1, MemRSS: 1, Name: "zsh"}})
	m.Update([]tmux.Pane{{Session: "live", PanePID: 200}},
		[]proc.Process{{PID: 200, PPID: 1, CPU: 1, MemRSS: 1, Name: "zsh"}})
	snap := m.Snapshot()
	if _, ok := snap["gone"]; ok {
		t.Errorf("session 'gone' should have been pruned")
	}
	if _, ok := snap["live"]; !ok {
		t.Errorf("session 'live' should be present")
	}
}
