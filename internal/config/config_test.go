package config_test

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/rtalex/demux/internal/config"
)

func containsStr(s []string, v string) bool {
    for _, x := range s {
        if x == v {
            return true
        }
    }
    return false
}

func TestDefaults(t *testing.T) {
    cfg := config.Default()
    if cfg.RefreshIntervalMs != 2000 {
        t.Errorf("expected 2000, got %d", cfg.RefreshIntervalMs)
    }
    if cfg.SidebarWidth != 30 {
        t.Errorf("expected 30, got %d", cfg.SidebarWidth)
    }
    if cfg.Git.TimeoutMs != 500 {
        t.Errorf("expected 500, got %d", cfg.Git.TimeoutMs)
    }
    if cfg.Git.OnTimeout != "cached" {
        t.Errorf("expected cached, got %s", cfg.Git.OnTimeout)
    }
}

func TestDefaults_IgnoredProcesses(t *testing.T) {
    cfg := config.Default()
    expected := []string{"zsh", "bash", "fish", "sh", "dash", "nu", "pwsh"}
    if len(cfg.IgnoredProcesses) != len(expected) {
        t.Fatalf("expected %d ignored processes, got %d: %v", len(expected), len(cfg.IgnoredProcesses), cfg.IgnoredProcesses)
    }
    for i, v := range expected {
        if cfg.IgnoredProcesses[i] != v {
            t.Errorf("IgnoredProcesses[%d]: expected %q, got %q", i, v, cfg.IgnoredProcesses[i])
        }
    }
}

func TestDefaults_ProcessesConfig(t *testing.T) {
    procs := config.Default().Theme.Processes
    if len(procs.Editors) == 0 {
        t.Error("expected non-empty default editors list")
    }
    if len(procs.Agents) == 0 {
        t.Error("expected non-empty default agents list")
    }
    if len(procs.Servers) == 0 {
        t.Error("expected non-empty default servers list")
    }
    if len(procs.Shells) == 0 {
        t.Error("expected non-empty default shells list")
    }
    // spot-check key entries
    if !containsStr(procs.Editors, "nvim") {
        t.Error("expected nvim in default editors")
    }
    if !containsStr(procs.Agents, "claude") {
        t.Error("expected claude in default agents")
    }
    if !containsStr(procs.Servers, "node") {
        t.Error("expected node in default servers")
    }
    if !containsStr(procs.Shells, "zsh") {
        t.Error("expected zsh in default shells")
    }
}

func TestLoadFromFile(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "dmux.toml")
    os.WriteFile(path, []byte(`
refresh_interval_ms = 1000
sidebar_width = 40
ignored_sessions = ["scratch"]

[git]
enabled = false
timeout_ms = 250
`), 0644)

    cfg, err := config.Load(path)
    if err != nil {
        t.Fatal(err)
    }
    if cfg.RefreshIntervalMs != 1000 {
        t.Errorf("expected 1000, got %d", cfg.RefreshIntervalMs)
    }
    if cfg.SidebarWidth != 40 {
        t.Errorf("expected 40, got %d", cfg.SidebarWidth)
    }
    if len(cfg.IgnoredSessions) != 1 || cfg.IgnoredSessions[0] != "scratch" {
        t.Errorf("unexpected ignored_sessions: %v", cfg.IgnoredSessions)
    }
    if cfg.Git.Enabled {
        t.Error("expected git.enabled = false")
    }
}

func TestMissingFile(t *testing.T) {
    cfg, err := config.Load("/nonexistent/path/dmux.toml")
    if err != nil {
        t.Fatal(err)
    }
    if cfg.RefreshIntervalMs != 2000 {
        t.Errorf("expected defaults, got %d", cfg.RefreshIntervalMs)
    }
}

func TestLoadFromFile_IgnoredProcesses(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "demux.toml")
    os.WriteFile(path, []byte(`ignored_processes = ["bash", "zsh"]`), 0644)

    cfg, err := config.Load(path)
    if err != nil {
        t.Fatal(err)
    }
    if len(cfg.IgnoredProcesses) != 2 {
        t.Fatalf("expected 2 ignored processes, got %d: %v", len(cfg.IgnoredProcesses), cfg.IgnoredProcesses)
    }
    if cfg.IgnoredProcesses[0] != "bash" || cfg.IgnoredProcesses[1] != "zsh" {
        t.Errorf("unexpected ignored processes: %v", cfg.IgnoredProcesses)
    }
}

func TestLoadFromFile_ProcessesConfig(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "demux.toml")
    os.WriteFile(path, []byte(`
[theme.processes]
editors = ["hx"]
agents  = ["aider"]
servers = ["bun"]
shells  = ["fish"]
`), 0644)

    cfg, err := config.Load(path)
    if err != nil {
        t.Fatal(err)
    }
    procs := cfg.Theme.Processes
    if len(procs.Editors) != 1 || procs.Editors[0] != "hx" {
        t.Errorf("unexpected editors: %v", procs.Editors)
    }
    if len(procs.Agents) != 1 || procs.Agents[0] != "aider" {
        t.Errorf("unexpected agents: %v", procs.Agents)
    }
    if len(procs.Servers) != 1 || procs.Servers[0] != "bun" {
        t.Errorf("unexpected servers: %v", procs.Servers)
    }
    if len(procs.Shells) != 1 || procs.Shells[0] != "fish" {
        t.Errorf("unexpected shells: %v", procs.Shells)
    }
}
