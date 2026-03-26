package config

import (
	"errors"
	"os"

	"github.com/BurntSushi/toml"
)

type GitPRConfig struct {
	Enabled bool `toml:"enabled"`
}

type GitConfig struct {
	Enabled         bool        `toml:"enabled"`
	TimeoutMs       int         `toml:"timeout_ms"`
	OnTimeout       string      `toml:"on_timeout"`
	FallbackDisplay string      `toml:"fallback_display"`
	ErrorDisplay    string      `toml:"error_display"`
	PR              GitPRConfig `toml:"pr"`
}

type Config struct {
	RefreshIntervalMs int      `toml:"refresh_interval_ms"`
	IgnoredSessions   []string `toml:"ignored_sessions"`
	DefaultFormat     string   `toml:"default_format"`
	StatusBarFormat   string   `toml:"status_bar_format"`
	SidebarWidth      int      `toml:"sidebar_width"`
	Git               GitConfig `toml:"git"`
}

func Default() Config {
	return Config{
		RefreshIntervalMs: 2000,
		IgnoredSessions:   []string{},
		DefaultFormat:     "text",
		StatusBarFormat:   "tmux",
		SidebarWidth:      30,
		Git: GitConfig{
			Enabled:         true,
			TimeoutMs:       500,
			OnTimeout:       "cached",
			FallbackDisplay: "—",
			ErrorDisplay:    "git err",
			PR:              GitPRConfig{Enabled: false},
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	_, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	return cfg, nil
}

// DefaultPath returns ~/.config/dmux/dmux.toml
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return home + "/.config/dmux/dmux.toml"
}
