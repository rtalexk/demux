package tui

import (
	"testing"

	"github.com/rtalexk/demux/internal/proc"
	"github.com/rtalexk/demux/internal/tmux"
)

func TestHandleProcDataMsg_PopulatesSessionStats(t *testing.T) {
	var m Model
	m.cfg.Sidebar.ShowSessionStats = true
	m.procGen = 0
	m.panes = []tmux.Pane{{Session: "s", PanePID: 100}}

	msg := procDataMsg{
		gen:    0,
		procs:  []proc.Process{{PID: 100, PPID: 1, CPU: 7, MemRSS: 50 * 1024 * 1024, Name: "zsh"}},
		cwdMap: map[int32]string{},
	}
	m2, _ := m.handleProcDataMsg(msg)
	st, ok := m2.sidebar.stats["s"]
	if !ok {
		t.Fatalf("sidebar stats not populated for session 's'")
	}
	if st.CPUNow != 7 {
		t.Errorf("CPUNow = %v, want 7", st.CPUNow)
	}
}

func TestHandleProcDataMsg_DisabledSkipsStats(t *testing.T) {
	var m Model
	m.cfg.Sidebar.ShowSessionStats = false
	m.panes = []tmux.Pane{{Session: "s", PanePID: 100}}
	msg := procDataMsg{gen: 0, procs: []proc.Process{{PID: 100, PPID: 1, CPU: 7, MemRSS: 1, Name: "zsh"}}, cwdMap: map[int32]string{}}
	m2, _ := m.handleProcDataMsg(msg)
	if _, ok := m2.sidebar.stats["s"]; ok {
		t.Errorf("stats should be empty when ShowSessionStats is false")
	}
}
