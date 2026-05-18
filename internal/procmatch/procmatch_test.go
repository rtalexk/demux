package procmatch_test

import (
	"testing"

	"github.com/rtalexk/demux/internal/proc"
	"github.com/rtalexk/demux/internal/procmatch"
	"github.com/rtalexk/demux/internal/tmux"
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

func makeProc(pid, ppid int32, name, cmd string) proc.Process {
	return proc.Process{PID: pid, PPID: ppid, Name: name, Cmdline: cmd}
}

func TestMatch_LevelZeroPaneRoot(t *testing.T) {
	panes := []tmux.Pane{{Session: "s1", PanePID: 100}}
	procs := []proc.Process{
		makeProc(100, 1, "claude", ""),
	}
	patterns := []procmatch.Pattern{{Match: "claude*", Label: "🤖"}}
	got := procmatch.Match(panes, procs, patterns, nil)
	if got["s1"].Text != "🤖" {
		t.Fatalf("expected 🤖, got %+v", got)
	}
}

func TestMatch_LevelOneChildWhenRootUnmatched(t *testing.T) {
	panes := []tmux.Pane{{Session: "s1", PanePID: 100}}
	procs := []proc.Process{
		makeProc(100, 1, "make", ""),
		makeProc(101, 100, "node", ""),
	}
	patterns := []procmatch.Pattern{{Match: "node*", Label: "n"}}
	got := procmatch.Match(panes, procs, patterns, nil)
	if got["s1"].Text != "n" {
		t.Fatalf("expected child match, got %+v", got)
	}
}

func TestMatch_ParentPrecedenceSkipsChildren(t *testing.T) {
	panes := []tmux.Pane{{Session: "s1", PanePID: 100}}
	procs := []proc.Process{
		makeProc(100, 1, "nvim", ""),
		makeProc(101, 100, "node", ""),
	}
	patterns := []procmatch.Pattern{
		{Match: "nvim*", Label: "nvim"},
		{Match: "node*", Label: "node"},
	}
	got := procmatch.Match(panes, procs, patterns, nil)
	if got["s1"].Text != "nvim" {
		t.Fatalf("expected parent to win, got %+v", got)
	}
}

func TestMatch_TopCountWinsAcrossPanes(t *testing.T) {
	panes := []tmux.Pane{
		{Session: "s1", PanePID: 100},
		{Session: "s1", PanePID: 200},
		{Session: "s1", PanePID: 300},
	}
	procs := []proc.Process{
		makeProc(100, 1, "node", ""),
		makeProc(200, 1, "node", ""),
		makeProc(300, 1, "uv", ""),
	}
	patterns := []procmatch.Pattern{
		{Match: "node*", Label: "node"},
		{Match: "uv*", Label: "py"},
	}
	got := procmatch.Match(panes, procs, patterns, nil)
	if got["s1"].Text != "node" {
		t.Fatalf("expected highest-count node, got %+v", got)
	}
}

func TestMatch_TieBreakByDeclarationOrder(t *testing.T) {
	panes := []tmux.Pane{
		{Session: "s1", PanePID: 100},
		{Session: "s1", PanePID: 200},
	}
	procs := []proc.Process{
		makeProc(100, 1, "node", ""),
		makeProc(200, 1, "uv", ""),
	}
	patterns := []procmatch.Pattern{
		{Match: "uv*", Label: "py"},
		{Match: "node*", Label: "node"},
	}
	got := procmatch.Match(panes, procs, patterns, nil)
	if got["s1"].Text != "py" {
		t.Fatalf("expected declaration-order tiebreak, got %+v", got)
	}
}

func TestMatch_NoDescentBeyondLevelOne(t *testing.T) {
	panes := []tmux.Pane{{Session: "s1", PanePID: 100}}
	procs := []proc.Process{
		makeProc(100, 1, "make", ""),
		makeProc(101, 100, "node", ""),
		makeProc(102, 101, "uv", ""),
		makeProc(103, 102, "python", ""),
	}
	patterns := []procmatch.Pattern{
		{Match: "uv*", Label: "py"},
		{Match: "node*", Label: "node"},
	}
	got := procmatch.Match(panes, procs, patterns, nil)
	if got["s1"].Text != "node" {
		t.Fatalf("expected level-1 node, got %+v", got)
	}
}

func TestMatch_PaneWithNoRootProcSkipped(t *testing.T) {
	panes := []tmux.Pane{
		{Session: "s1", PanePID: 999},
		{Session: "s1", PanePID: 100},
	}
	procs := []proc.Process{
		makeProc(100, 1, "node", ""),
	}
	patterns := []procmatch.Pattern{{Match: "node*", Label: "n"}}
	got := procmatch.Match(panes, procs, patterns, nil)
	if got["s1"].Text != "n" {
		t.Fatalf("expected node from valid pane, got %+v", got)
	}
}

func TestMatch_NoMatchProducesNoEntry(t *testing.T) {
	panes := []tmux.Pane{{Session: "s1", PanePID: 100}}
	procs := []proc.Process{makeProc(100, 1, "zsh", "")}
	patterns := []procmatch.Pattern{{Match: "node*", Label: "n"}}
	got := procmatch.Match(panes, procs, patterns, nil)
	if _, ok := got["s1"]; ok {
		t.Fatalf("expected no entry for s1, got %+v", got)
	}
}

func TestMatch_EmptyInputs(t *testing.T) {
	if got := procmatch.Match(nil, nil, nil, nil); len(got) != 0 {
		t.Fatalf("expected empty map, got %+v", got)
	}
}

func TestMatch_IgnoredShellPromotesChildren(t *testing.T) {
	panes := []tmux.Pane{{Session: "s1", PanePID: 100}}
	procs := []proc.Process{
		makeProc(100, 1, "zsh", "-zsh"),
		makeProc(101, 100, "make", "make dev"),
		makeProc(102, 101, "node", "node server.js"),
	}
	patterns := []procmatch.Pattern{{Match: "node*", Label: "node"}}
	got := procmatch.Match(panes, procs, patterns, []string{"zsh"})
	if got["s1"].Text != "node" {
		t.Fatalf("expected node label after shell promoted, got %+v", got)
	}
}

func TestMatch_IgnoredShellPreservesDepthCap(t *testing.T) {
	panes := []tmux.Pane{{Session: "s1", PanePID: 100}}
	procs := []proc.Process{
		makeProc(100, 1, "zsh", ""),
		makeProc(101, 100, "make", ""),
		makeProc(102, 101, "node", ""),
		makeProc(103, 102, "uv", ""),
	}
	patterns := []procmatch.Pattern{{Match: "uv*", Label: "py"}}
	got := procmatch.Match(panes, procs, patterns, []string{"zsh"})
	if _, ok := got["s1"]; ok {
		t.Fatalf("expected uv at effective depth 2 to be skipped, got %+v", got)
	}
}

func TestMatch_IgnoredChainPromotedTransparently(t *testing.T) {
	panes := []tmux.Pane{{Session: "s1", PanePID: 100}}
	procs := []proc.Process{
		makeProc(100, 1, "zsh", ""),
		makeProc(101, 100, "bash", ""),
		makeProc(102, 101, "make", ""),
		makeProc(103, 102, "node", ""),
	}
	patterns := []procmatch.Pattern{{Match: "node*", Label: "node"}}
	got := procmatch.Match(panes, procs, patterns, []string{"zsh", "bash"})
	if got["s1"].Text != "node" {
		t.Fatalf("expected chained shell promotion, got %+v", got)
	}
}

func TestMatch_IgnoredCaseInsensitive(t *testing.T) {
	panes := []tmux.Pane{{Session: "s1", PanePID: 100}}
	procs := []proc.Process{
		makeProc(100, 1, "Zsh", ""),
		makeProc(101, 100, "node", ""),
	}
	patterns := []procmatch.Pattern{{Match: "node*", Label: "node"}}
	got := procmatch.Match(panes, procs, patterns, []string{"ZSH"})
	if got["s1"].Text != "node" {
		t.Fatalf("expected case-insensitive ignored match, got %+v", got)
	}
}

func TestMatch_NilIgnoredBehavesLikeBefore(t *testing.T) {
	panes := []tmux.Pane{{Session: "s1", PanePID: 100}}
	procs := []proc.Process{
		makeProc(100, 1, "zsh", ""),
		makeProc(101, 100, "node", ""),
	}
	patterns := []procmatch.Pattern{{Match: "node*", Label: "node"}}
	got := procmatch.Match(panes, procs, patterns, nil)
	if got["s1"].Text != "node" {
		t.Fatalf("expected level-1 node hit with nil ignored, got %+v", got)
	}
}
