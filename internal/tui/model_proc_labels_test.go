package tui

import (
	"testing"

	"github.com/rtalexk/demux/internal/config"
	"github.com/rtalexk/demux/internal/proc"
	"github.com/rtalexk/demux/internal/tmux"
)

func TestModel_HandleProcDataMsg_PopulatesProcLabels(t *testing.T) {
	cfg := config.Default()
	cfg.Sidebar.Processes = []config.ProcessLabel{
		{Match: "claude*", Label: "🤖"},
	}
	m := New(cfg, nil)
	m.panes = []tmux.Pane{{Session: "s1", PanePID: 100}}
	m.procGen = 1
	msg := procDataMsg{
		gen:    1,
		procs:  []proc.Process{{PID: 100, PPID: 1, Name: "claude"}},
		cwdMap: map[int32]string{},
	}
	updated, _ := m.handleProcDataMsg(msg)
	if got := updated.sidebar.procLabels["s1"].Text; got != "🤖" {
		t.Fatalf("expected 🤖, got %q", got)
	}
}
