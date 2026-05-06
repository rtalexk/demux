package procmatch_test

import (
	"testing"

	"github.com/rtalexk/demux/internal/proc"
	"github.com/rtalexk/demux/internal/procmatch"
)

func TestPatternZeroValue(t *testing.T) {
	var p procmatch.Pattern
	if p.Match != "" || p.Label != "" || p.FG != "" || p.BG != "" {
		t.Fatalf("expected zero-valued Pattern, got %+v", p)
	}
}

func TestLabelZeroValue(t *testing.T) {
	var l procmatch.Label
	if l.Text != "" || l.FG != "" || l.BG != "" {
		t.Fatalf("expected zero-valued Label, got %+v", l)
	}
}

func TestMatchProcess_ByName(t *testing.T) {
	p := proc.Process{Name: "node", Cmdline: "node"}
	patterns := []procmatch.Pattern{{Match: "node*", Label: "node"}}
	got, ok := procmatch.MatchProcess(p, patterns)
	if !ok || got.Text != "node" {
		t.Fatalf("expected node hit, got=%+v ok=%v", got, ok)
	}
}

func TestMatchProcess_ByCmdline(t *testing.T) {
	p := proc.Process{Name: "node", Cmdline: "/usr/bin/node /app/server.js --port 3000"}
	patterns := []procmatch.Pattern{{Match: "*server.js*", Label: "srv"}}
	got, ok := procmatch.MatchProcess(p, patterns)
	if !ok || got.Text != "srv" {
		t.Fatalf("expected srv hit, got=%+v ok=%v", got, ok)
	}
}

func TestMatchProcess_CaseInsensitive(t *testing.T) {
	p := proc.Process{Name: "Node", Cmdline: ""}
	patterns := []procmatch.Pattern{{Match: "NODE*", Label: "n"}}
	got, ok := procmatch.MatchProcess(p, patterns)
	if !ok || got.Text != "n" {
		t.Fatalf("expected case-insensitive hit, got=%+v ok=%v", got, ok)
	}
}

func TestMatchProcess_DeclarationOrderWins(t *testing.T) {
	p := proc.Process{Name: "claude", Cmdline: ""}
	patterns := []procmatch.Pattern{
		{Match: "claude*", Label: "first"},
		{Match: "claude", Label: "second"},
	}
	got, ok := procmatch.MatchProcess(p, patterns)
	if !ok || got.Text != "first" {
		t.Fatalf("expected first declaration to win, got=%+v ok=%v", got, ok)
	}
}

func TestMatchProcess_NoMatch(t *testing.T) {
	p := proc.Process{Name: "zsh", Cmdline: "/bin/zsh"}
	patterns := []procmatch.Pattern{{Match: "node*", Label: "node"}}
	if _, ok := procmatch.MatchProcess(p, patterns); ok {
		t.Fatalf("expected no match")
	}
}

func TestMatchProcess_BadGlobSkipped(t *testing.T) {
	p := proc.Process{Name: "node", Cmdline: ""}
	patterns := []procmatch.Pattern{
		{Match: "[", Label: "broken"},
		{Match: "node*", Label: "node"},
	}
	got, ok := procmatch.MatchProcess(p, patterns)
	if !ok || got.Text != "node" {
		t.Fatalf("expected fallthrough past bad glob, got=%+v ok=%v", got, ok)
	}
}

func TestMatchProcess_LabelCarriesColors(t *testing.T) {
	p := proc.Process{Name: "uv", Cmdline: ""}
	patterns := []procmatch.Pattern{{Match: "uv*", Label: "py", FG: "#fff", BG: "#000"}}
	got, ok := procmatch.MatchProcess(p, patterns)
	if !ok || got.Text != "py" || got.FG != "#fff" || got.BG != "#000" {
		t.Fatalf("expected colors propagated, got=%+v ok=%v", got, ok)
	}
}
