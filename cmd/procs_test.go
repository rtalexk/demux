package cmd

import (
	"testing"

	"github.com/rtalexk/demux/internal/proc"
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
