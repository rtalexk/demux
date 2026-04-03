package tui

import (
	"strings"
	"testing"

	"github.com/rtalexk/demux/internal/git"
)

func TestDetailRender_clipsContentToHeight(t *testing.T) {
	d := DetailModel{
		selType:    DetailSession,
		sessionCWD: "/some/path",
		winCount:   3,
		procCount:  6,
		alertCount: 1,
	}
	// height=6 → inner=4, so content must be clipped to 4 lines
	rendered := d.Render(40, 6)
	inner := stripANSI(rendered)
	lines := strings.Split(strings.TrimRight(inner, "\n"), "\n")
	// subtract 2 border lines
	contentLines := len(lines) - 2
	if contentLines > 4 {
		t.Errorf("expected at most 4 content lines for height=6, got %d", contentLines)
	}
}

func TestDetailRender_emptyStateWhenNoSelection(t *testing.T) {
	d := DetailModel{}
	rendered := d.Render(40, 8)
	if !strings.Contains(stripANSI(rendered), "No selection") {
		t.Error("expected 'No selection' for empty DetailModel")
	}
}

func TestWorktreeValue_bareRepoConvention(t *testing.T) {
	// demux/.bare + demux/main/ → worktree dir == worktree name → show parent "demux"
	info := git.Info{Worktree: "main", RepoRoot: "/home/user/demux/main"}
	got := worktreeValue(info)
	if got != "main (demux)" {
		t.Errorf("bare-repo case: got %q, want %q", got, "main (demux)")
	}
}

func TestWorktreeValue_standardLinkedWorktree(t *testing.T) {
	// Non-bare linked worktree: working dir name differs from worktree name
	info := git.Info{Worktree: "feature", RepoRoot: "/home/user/myproject-feature"}
	got := worktreeValue(info)
	if got != "feature (myproject-feature)" {
		t.Errorf("standard worktree case: got %q, want %q", got, "feature (myproject-feature)")
	}
}

func TestWorktreeValue_noRepoRoot(t *testing.T) {
	info := git.Info{Worktree: "main", RepoRoot: ""}
	got := worktreeValue(info)
	if got != "main" {
		t.Errorf("no root case: got %q, want %q", got, "main")
	}
}

func TestDetailRender_showsSessionFields(t *testing.T) {
	d := DetailModel{
		selType:    DetailSession,
		sessionCWD: "/work/myses",
		winCount:   2,
		procCount:  4,
		alertCount: 0,
	}
	rendered := stripANSI(d.Render(60, 14))
	for _, want := range []string{"/work/myses", "2", "4"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("expected %q in session detail, got:\n%s", want, rendered)
		}
	}
}
