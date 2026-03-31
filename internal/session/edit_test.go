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
    e := ConfigEntry{Name: "main", Alias: "myproj", Path: "/home/user/myproj"}
    if err := AppendEntry(path, e); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    data, err := os.ReadFile(path)
    if err != nil {
        t.Fatalf("read file: %v", err)
    }
    s := string(data)
    if !strings.Contains(s, `[[session]]`) {
        t.Error("missing [[session]] header")
    }
    if !strings.Contains(s, `name  = "main"`) {
        t.Error("missing name field")
    }
    if !strings.Contains(s, `alias = "myproj"`) {
        t.Error("missing alias field")
    }
    if !strings.Contains(s, `path  = "/home/user/myproj"`) {
        t.Error("missing path field")
    }
    if strings.Contains(s, "worktree") {
        t.Error("worktree=false should be omitted")
    }
    if strings.Contains(s, "labels") {
        t.Error("empty labels should be omitted")
    }
    if strings.Contains(s, "icon") {
        t.Error("empty icon should be omitted")
    }
}

func TestAppendEntry_AppendsToExistingFile(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "sessions.toml")
    writeTOML(t, dir, "sessions.toml", `[[session]]
name  = "existing"
alias = "ex"
path  = "/ex"
`)
    e := ConfigEntry{Name: "new", Alias: "nw", Path: "/new"}
    if err := AppendEntry(path, e); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    data, err := os.ReadFile(path)
    if err != nil {
        t.Fatalf("read file: %v", err)
    }
    s := string(data)
    if !strings.Contains(s, `name  = "existing"`) {
        t.Error("existing entry should be preserved")
    }
    if !strings.Contains(s, `name  = "new"`) {
        t.Error("new entry should be appended")
    }
}

func TestAppendEntry_RejectsDuplicate(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "sessions.toml")
    writeTOML(t, dir, "sessions.toml", `[[session]]
name  = "main"
alias = "myproj"
path  = "/foo"
`)
    e := ConfigEntry{Name: "main", Alias: "myproj", Path: "/bar"}
    err := AppendEntry(path, e)
    if err == nil {
        t.Error("expected duplicate error, got nil")
    } else if !strings.Contains(err.Error(), "already exists") {
        t.Errorf("expected 'already exists' in error, got: %v", err)
    }
}

func TestAppendEntry_OptionalFields(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "sessions.toml")
    e := ConfigEntry{
        Name:     "main",
        Alias:    "myproj",
        Path:     "/home/user/myproj",
        Worktree: true,
        Labels:   []string{"work", "rust"},
        Icon:     "󰅩",
    }
    if err := AppendEntry(path, e); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    data, err := os.ReadFile(path)
    if err != nil {
        t.Fatalf("read file: %v", err)
    }
    s := string(data)
    if !strings.Contains(s, "worktree = true") {
        t.Error("worktree=true should be present")
    }
    if !strings.Contains(s, `labels   = ["work", "rust"]`) {
        t.Error("labels should be present")
    }
    if !strings.Contains(s, `icon     = "󰅩"`) {
        t.Error("icon should be present")
    }
}
