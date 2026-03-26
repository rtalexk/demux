package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rtalex/demux/internal/config"
)

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
