package cmd

import (
    "testing"
)

func TestFetchGitForSessions_EmptyInput(t *testing.T) {
    results := fetchGitForSessions(nil, 500, "err")
    if len(results) != 0 {
        t.Errorf("expected empty map, got %d entries", len(results))
    }
}

func TestFetchGitForSessions_NonExistentDir(t *testing.T) {
    work := []sessionGitWork{
        {sessionName: "s1", primaryCWD: "/nonexistent/path/xyz"},
    }
    results := fetchGitForSessions(work, 100, "—")
    if _, ok := results["s1"]; ok {
        t.Error("expected no entry for failed fetch, got one")
    }
}
