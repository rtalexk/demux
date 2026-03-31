package session

import (
    "os"
    "path/filepath"
    "testing"
)

func writeTOML(t *testing.T, dir, name, content string) {
    t.Helper()
    if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
        t.Fatal(err)
    }
}

func TestLoadConfigSessions_MissingFiles(t *testing.T) {
    dir := t.TempDir()
    entries, err := LoadConfigSessions(dir)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(entries) != 0 {
        t.Errorf("expected 0 entries, got %d", len(entries))
    }
}

func TestLoadConfigSessions_SessionsOnly(t *testing.T) {
    dir := t.TempDir()
    writeTOML(t, dir, "sessions.toml", `
[[session]]
name     = "main"
alias    = "dotf"
path     = "/foo"
worktree = true
labels   = ["work"]
icon     = "x"
`)
    entries, err := LoadConfigSessions(dir)
    if err != nil {
        t.Fatal(err)
    }
    if len(entries) != 1 {
        t.Fatalf("expected 1, got %d", len(entries))
    }
    e := entries[0]
    if e.DisplayName() != "dotf-main" {
        t.Errorf("expected dotf-main, got %s", e.DisplayName())
    }
    if !e.Worktree {
        t.Error("expected Worktree=true")
    }
    if len(e.Labels) != 1 || e.Labels[0] != "work" {
        t.Error("expected label work")
    }
}

func TestLoadConfigSessions_PrivateOverrides(t *testing.T) {
    dir := t.TempDir()
    writeTOML(t, dir, "sessions.toml", `
[[session]]
name  = "main"
alias = "dotf"
path  = "/public"
`)
    writeTOML(t, dir, "private.toml", `
[[session]]
name  = "main"
alias = "dotf"
path  = "/private"
`)
    entries, err := LoadConfigSessions(dir)
    if err != nil {
        t.Fatal(err)
    }
    if len(entries) != 1 {
        t.Fatalf("expected 1 (private overrides), got %d", len(entries))
    }
    if entries[0].Path != "/private" {
        t.Errorf("expected /private, got %s", entries[0].Path)
    }
}

func TestLoadConfigSessions_SkipsInvalidEntries(t *testing.T) {
    dir := t.TempDir()
    writeTOML(t, dir, "sessions.toml", `
[[session]]
name  = "main"
alias = ""
path  = "/foo"

[[session]]
name  = "other"
alias = "ok"
path  = "/bar"
`)
    entries, err := LoadConfigSessions(dir)
    if err != nil {
        t.Fatal(err)
    }
    if len(entries) != 1 {
        t.Errorf("expected 1 valid entry, got %d", len(entries))
    }
}
