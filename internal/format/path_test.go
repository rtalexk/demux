package format_test

import (
    "testing"

    "github.com/rtalex/demux/internal/config"
    "github.com/rtalex/demux/internal/format"
)

func TestShortenPath_NoAliases(t *testing.T) {
    got := format.ShortenPath("/home/user/projects/foo", nil)
    if got != "/home/user/projects/foo" {
        t.Errorf("expected unchanged, got %q", got)
    }
}

func TestShortenPath_NoMatch(t *testing.T) {
    aliases := []config.PathAlias{
        {Prefix: "/other", Replace: "o"},
    }
    got := format.ShortenPath("/home/user/projects/foo", aliases)
    if got != "/home/user/projects/foo" {
        t.Errorf("expected unchanged, got %q", got)
    }
}

func TestShortenPath_ExactMatch(t *testing.T) {
    aliases := []config.PathAlias{
        {Prefix: "/home/user", Replace: "~"},
    }
    got := format.ShortenPath("/home/user", aliases)
    if got != "~" {
        t.Errorf("expected ~, got %q", got)
    }
}

func TestShortenPath_PrefixMatch(t *testing.T) {
    aliases := []config.PathAlias{
        {Prefix: "/home/user", Replace: "~"},
    }
    got := format.ShortenPath("/home/user/projects/foo", aliases)
    if got != "~/projects/foo" {
        t.Errorf("expected ~/projects/foo, got %q", got)
    }
}

func TestShortenPath_LongestPrefixWins(t *testing.T) {
    // Aliases must arrive pre-sorted longest-first (as config.Load guarantees).
    aliases := []config.PathAlias{
        {Prefix: "/home/user/projects", Replace: "proj"},
        {Prefix: "/home/user", Replace: "~"},
    }
    got := format.ShortenPath("/home/user/projects/foo", aliases)
    if got != "proj/foo" {
        t.Errorf("expected proj/foo, got %q", got)
    }
}

func TestShortenPath_FirstAliasApplied(t *testing.T) {
    // Shorter prefix is first — it should still match (not skip to longer one).
    aliases := []config.PathAlias{
        {Prefix: "/home/user", Replace: "~"},
        {Prefix: "/home/user/projects", Replace: "proj"},
    }
    got := format.ShortenPath("/home/user/projects/foo", aliases)
    // First match wins: ~ wins here because it comes first in this list.
    if got != "~/projects/foo" {
        t.Errorf("expected ~/projects/foo, got %q", got)
    }
}
