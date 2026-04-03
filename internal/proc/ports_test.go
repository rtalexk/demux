package proc_test

import (
	"testing"

	"github.com/rtalexk/demux/internal/proc"
)

func TestParseLsofPorts(t *testing.T) {
	// Simulate `lsof -iTCP -sTCP:LISTEN -n -P` output
	raw := `COMMAND  PID USER   FD   TYPE DEVICE SIZE/OFF NODE NAME
node    1234  dev  23u  IPv4 0x1      0t0  TCP *:3000 (LISTEN)
node    5678  dev  24u  IPv6 0x2      0t0  TCP *:5173 (LISTEN)`

	ports, err := proc.ParseLsofPorts(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(ports) != 2 {
		t.Fatalf("expected 2, got %d", len(ports))
	}
	if ports[0].Port != 3000 {
		t.Errorf("expected 3000, got %d", ports[0].Port)
	}
	if ports[0].PID != 1234 {
		t.Errorf("expected 1234, got %d", ports[0].PID)
	}
	if ports[1].Port != 5173 {
		t.Errorf("expected 5173, got %d", ports[1].Port)
	}
}
