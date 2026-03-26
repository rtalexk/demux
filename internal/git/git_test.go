package git_test

import (
    "testing"

    "github.com/rtalex/demux/internal/git"
)

func TestParseStatusAheadBehindDirty(t *testing.T) {
    raw := "## main...origin/main [ahead 2, behind 1]\n M file.go\n"
    info, err := git.ParseStatus(raw)
    if err != nil {
        t.Fatal(err)
    }
    if info.Branch != "main" {
        t.Errorf("expected main, got %s", info.Branch)
    }
    if info.Ahead != 2 {
        t.Errorf("expected ahead=2, got %d", info.Ahead)
    }
    if info.Behind != 1 {
        t.Errorf("expected behind=1, got %d", info.Behind)
    }
    if !info.Dirty {
        t.Error("expected dirty=true")
    }
}

func TestParseStatusClean(t *testing.T) {
    raw := "## feat/thing...origin/feat/thing\n"
    info, err := git.ParseStatus(raw)
    if err != nil {
        t.Fatal(err)
    }
    if info.Dirty {
        t.Error("expected clean")
    }
    if info.Ahead != 0 || info.Behind != 0 {
        t.Errorf("expected 0/0, got %d/%d", info.Ahead, info.Behind)
    }
    if info.Branch != "feat/thing" {
        t.Errorf("expected feat/thing, got %s", info.Branch)
    }
}

func TestParseStatusDetached(t *testing.T) {
    raw := "## HEAD (no branch)\n"
    info, err := git.ParseStatus(raw)
    if err != nil {
        t.Fatal(err)
    }
    if info.Branch != "HEAD" {
        t.Errorf("expected HEAD, got %s", info.Branch)
    }
}

func TestParseStatusNoRemote(t *testing.T) {
    // local branch with no remote tracking
    raw := "## localonly\n"
    info, err := git.ParseStatus(raw)
    if err != nil {
        t.Fatal(err)
    }
    if info.Branch != "localonly" {
        t.Errorf("expected localonly, got %s", info.Branch)
    }
    if info.Ahead != 0 || info.Behind != 0 {
        t.Errorf("expected 0/0, got %d/%d", info.Ahead, info.Behind)
    }
}

func TestIsDescendant(t *testing.T) {
    if !git.IsDescendant("/home/dev/project/ui", "/home/dev/project") {
        t.Error("expected ui to be descendant of project")
    }
    if git.IsDescendant("/home/dev/other", "/home/dev/project") {
        t.Error("expected other NOT to be descendant of project")
    }
    if git.IsDescendant("/home/dev/project", "/home/dev/project") {
        t.Error("exact match should not count as descendant")
    }
    if git.IsDescendant("/home/dev/projectx", "/home/dev/project") {
        t.Error("prefix-but-not-path-component should not count as descendant")
    }
    if git.IsDescendant("", "/home/dev/project") {
        t.Error("empty child should not be descendant")
    }
}
