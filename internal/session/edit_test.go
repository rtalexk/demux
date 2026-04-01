package session

import (
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestAppendEntry_CreatesFile(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "sessions.toml")
    e := ConfigEntry{Name: "myproj-main", Group: "myproj", Path: "/home/user/myproj"}
    if err := AppendEntry(path, e); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    // Round-trip: verify the stored data is correct.
    cfg, err := LoadConfigSessions(dir)
    if err != nil {
        t.Fatalf("load back: %v", err)
    }
    if len(cfg.Entries) != 1 {
        t.Fatalf("expected 1 entry, got %d", len(cfg.Entries))
    }
    got := cfg.Entries[0]
    if got.Name != "myproj-main" {
        t.Errorf("name: got %q, want %q", got.Name, "myproj-main")
    }
    if got.Group != "myproj" {
        t.Errorf("group: got %q, want %q", got.Group, "myproj")
    }
    if got.Path != "/home/user/myproj" {
        t.Errorf("path: got %q, want %q", got.Path, "/home/user/myproj")
    }
    // Zero-value optional fields should not be written to the file.
    data, err := os.ReadFile(path)
    if err != nil {
        t.Fatalf("read file: %v", err)
    }
    s := string(data)
    if strings.Contains(s, "worktree") {
        t.Error("worktree=false should be omitted")
    }
    if strings.Contains(s, "labels") {
        t.Error("empty labels should be omitted")
    }
    if strings.Contains(s, "icon") {
        t.Error("empty icon should be omitted")
    }
    if strings.Contains(s, "windows") {
        t.Error("empty windows should be omitted")
    }
}

func TestAppendEntry_AppendsToExistingFile(t *testing.T) {
    dir := t.TempDir()
    writeTOML(t, dir, "sessions.toml", `[[session]]
name  = "ex-existing"
group = "ex"
path  = "/ex"
`)
    e := ConfigEntry{Name: "nw-new", Group: "nw", Path: "/new"}
    if err := AppendEntry(filepath.Join(dir, "sessions.toml"), e); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    cfg, err := LoadConfigSessions(dir)
    if err != nil {
        t.Fatalf("load back: %v", err)
    }
    names := make(map[string]bool, len(cfg.Entries))
    for _, entry := range cfg.Entries {
        names[entry.Name] = true
    }
    if !names["ex-existing"] {
        t.Error("existing entry should be preserved")
    }
    if !names["nw-new"] {
        t.Error("new entry should be appended")
    }
}

func TestAppendEntry_RejectsDuplicate(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "sessions.toml")
    writeTOML(t, dir, "sessions.toml", `[[session]]
name  = "myproj-main"
group = "myproj"
path  = "/foo"
`)
    e := ConfigEntry{Name: "myproj-main", Group: "myproj", Path: "/bar"}
    err := AppendEntry(path, e)
    if err == nil {
        t.Error("expected duplicate error, got nil")
    } else if !strings.Contains(err.Error(), "already exists") {
        t.Errorf("expected 'already exists' in error, got: %v", err)
    }
}

func TestAppendEntry_OptionalFields(t *testing.T) {
    dir := t.TempDir()
    e := ConfigEntry{
        Name:     "myproj-main",
        Group:    "myproj",
        Path:     "/home/user/myproj",
        Worktree: true,
        Labels:   []string{"work", "rust"},
        Icon:     "󰅩",
    }
    if err := AppendEntry(filepath.Join(dir, "sessions.toml"), e); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    cfg, err := LoadConfigSessions(dir)
    if err != nil {
        t.Fatalf("load back: %v", err)
    }
    if len(cfg.Entries) != 1 {
        t.Fatalf("expected 1 entry, got %d", len(cfg.Entries))
    }
    got := cfg.Entries[0]
    if !got.Worktree {
        t.Error("expected Worktree=true")
    }
    if len(got.Labels) != 2 || got.Labels[0] != "work" || got.Labels[1] != "rust" {
        t.Errorf("expected labels [work rust], got %v", got.Labels)
    }
    if got.Icon != "󰅩" {
        t.Errorf("expected icon 󰅩, got %q", got.Icon)
    }
}

func TestAppendEntry_Windows(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "sessions.toml")
    e := ConfigEntry{
        Name:    "myproj-main",
        Group:   "myproj",
        Path:    "/home/user/myproj",
        Windows: []string{"editor", "terminal"},
    }
    if err := AppendEntry(path, e); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    // round-trip: load back and verify
    entries, loadErr := LoadConfigSessions(dir)
    if loadErr != nil {
        t.Fatalf("load: %v", loadErr)
    }
    if len(entries.Entries) != 1 {
        t.Fatalf("expected 1 entry, got %d", len(entries.Entries))
    }
    got := entries.Entries[0].Windows
    if len(got) != 2 || got[0] != "editor" || got[1] != "terminal" {
        t.Errorf("round-trip windows mismatch: %v", got)
    }
}

func TestRemoveEntry_RemovesMatchingBlock(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "sessions.toml")
    writeTOML(t, dir, "sessions.toml", `[[session]]
name  = "proj-main"
group = "proj"
path  = "/proj"

[[session]]
name  = "oth-other"
group = "oth"
path  = "/other"
`)
    if err := RemoveEntry(path, "proj-main"); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    data, err := os.ReadFile(path)
    if err != nil {
        t.Fatalf("read file: %v", err)
    }
    s := string(data)
    if strings.Contains(s, `name  = "proj-main"`) {
        t.Error("removed entry should not be present")
    }
    if !strings.Contains(s, `name  = "oth-other"`) {
        t.Error("other entry should be preserved")
    }
}

func TestRemoveEntry_NotFound(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "sessions.toml")
    writeTOML(t, dir, "sessions.toml", `[[session]]
name  = "proj-main"
group = "proj"
path  = "/proj"
`)
    err := RemoveEntry(path, "wrong-name")
    if err == nil {
        t.Error("expected not-found error")
    }
}

func TestRemoveEntry_FileNotExist(t *testing.T) {
    path := filepath.Join(t.TempDir(), "missing.toml")
    err := RemoveEntry(path, "proj-main")
    if err == nil {
        t.Error("expected error for missing file")
    }
}

func TestRemoveEntry_OnlyEntry(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "sessions.toml")
    writeTOML(t, dir, "sessions.toml", `[[session]]
name  = "proj-main"
group = "proj"
path  = "/proj"
`)
    if err := RemoveEntry(path, "proj-main"); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    data, err := os.ReadFile(path)
    if err != nil {
        t.Fatalf("read file: %v", err)
    }
    if strings.Contains(string(data), "[[session]]") {
        t.Error("session block should be gone")
    }
}
