package git_test

import (
    "testing"

    "github.com/rtalexk/demux/internal/git"
)

func TestFetchConcurrent_empty(t *testing.T) {
    result := git.FetchConcurrent(nil, 200)
    if len(result) != 0 {
        t.Errorf("expected empty result, got %d entries", len(result))
    }
}

func TestFetchConcurrent_invalidPath(t *testing.T) {
    work := []git.ConcurrentWork{
        {Key: "a", Dir: "/nonexistent/path/xyz"},
    }
    result := git.FetchConcurrent(work, 200)
    // Should return empty map (errors are silently skipped — callers log if needed)
    if len(result) != 0 {
        t.Errorf("expected empty result for invalid path, got %d entries", len(result))
    }
}
