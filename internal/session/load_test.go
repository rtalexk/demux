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
    cfg, err := LoadConfigSessions(dir)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(cfg.Entries) != 0 {
        t.Errorf("expected 0 entries, got %d", len(cfg.Entries))
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
    cfg, err := LoadConfigSessions(dir)
    if err != nil {
        t.Fatal(err)
    }
    if len(cfg.Entries) != 1 {
        t.Fatalf("expected 1, got %d", len(cfg.Entries))
    }
    e := cfg.Entries[0]
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
    cfg, err := LoadConfigSessions(dir)
    if err != nil {
        t.Fatal(err)
    }
    if len(cfg.Entries) != 1 {
        t.Fatalf("expected 1 (private overrides), got %d", len(cfg.Entries))
    }
    if cfg.Entries[0].Path != "/private" {
        t.Errorf("expected /private, got %s", cfg.Entries[0].Path)
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
    cfg, err := LoadConfigSessions(dir)
    if err != nil {
        t.Fatal(err)
    }
    if len(cfg.Entries) != 1 {
        t.Errorf("expected 1 valid entry, got %d", len(cfg.Entries))
    }
}

func TestLoadConfigSessions_WindowTemplates(t *testing.T) {
    dir := t.TempDir()
    writeTOML(t, dir, "sessions.toml", `
[[window_templates]]
id              = "editor"
name            = "Editor"
after_create_cmd = "nvim ."

[[window_templates]]
id   = "generic"
name = "Generic"
from = "editor"

[[session]]
name    = "main"
alias   = "dotf"
path    = "/foo"
windows = ["editor", "generic"]
`)
    cfg, err := LoadConfigSessions(dir)
    if err != nil {
        t.Fatal(err)
    }
    if len(cfg.Entries) != 1 {
        t.Fatalf("expected 1 entry, got %d", len(cfg.Entries))
    }
    if len(cfg.Entries[0].Windows) != 2 {
        t.Errorf("expected 2 windows, got %d", len(cfg.Entries[0].Windows))
    }
    editor, ok := cfg.WindowTemplates["editor"]
    if !ok {
        t.Fatal("expected editor template")
    }
    if editor.Name != "Editor" {
        t.Errorf("expected name 'Editor', got %q", editor.Name)
    }
    if editor.AfterCreateCmd != "nvim ." {
        t.Errorf("expected 'nvim .', got %q", editor.AfterCreateCmd)
    }
    generic, ok := cfg.WindowTemplates["generic"]
    if !ok {
        t.Fatal("expected generic template")
    }
    if generic.Name != "Generic" {
        t.Errorf("expected name 'Generic', got %q", generic.Name)
    }
    // generic inherits after_create_cmd from editor
    if generic.AfterCreateCmd != "nvim ." {
        t.Errorf("generic should inherit 'nvim .', got %q", generic.AfterCreateCmd)
    }
    if generic.From != "" {
        t.Error("resolved template should have empty From field")
    }
}

func TestLoadConfigSessions_WindowTemplateOverride(t *testing.T) {
    dir := t.TempDir()
    writeTOML(t, dir, "sessions.toml", `
[[window_templates]]
id              = "base"
name            = "Base"
after_create_cmd = "echo base"

[[window_templates]]
id              = "derived"
name            = "Derived"
from            = "base"
after_create_cmd = "echo derived"
`)
    cfg, err := LoadConfigSessions(dir)
    if err != nil {
        t.Fatal(err)
    }
    derived := cfg.WindowTemplates["derived"]
    if derived.AfterCreateCmd != "echo derived" {
        t.Errorf("expected 'echo derived', got %q", derived.AfterCreateCmd)
    }
    if derived.Name != "Derived" {
        t.Errorf("expected name 'Derived', got %q", derived.Name)
    }
}
