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

// ThemeConfig holds optional per-color overrides.
// Any field left empty inherits the default Catppuccin Mocha value.
type ThemeConfig struct {
    ColorBg       string `toml:"color_bg"`
    ColorSurface  string `toml:"color_surface"`
    ColorRaised   string `toml:"color_raised"`
    ColorSelected string `toml:"color_selected"`
    ColorBorder   string `toml:"color_border"`

    ColorFgPrimary string `toml:"color_fg_primary"`
    ColorFgSubtext string `toml:"color_fg_subtext"`
    ColorFgMuted   string `toml:"color_fg_muted"`
    ColorFgDim     string `toml:"color_fg_dim"`
    ColorFgGhost   string `toml:"color_fg_ghost"`

    ColorSession    string `toml:"color_session"`
    ColorProcClaude string `toml:"color_proc_claude"`
    ColorProcServer string `toml:"color_proc_server"`
    ColorProcEditor string `toml:"color_proc_editor"`
    ColorProcChild  string `toml:"color_proc_child"`

    ColorGitDirty  string `toml:"color_git_dirty"`
    ColorGitBehind string `toml:"color_git_behind"`
    ColorGitAhead  string `toml:"color_git_ahead"`

    ColorAlertInfo  string `toml:"color_alert_info"`
    ColorAlertWarn  string `toml:"color_alert_warn"`
    ColorAlertError string `toml:"color_alert_error"`

    ColorPort    string `toml:"color_port"`
    ColorPortBg  string `toml:"color_port_bg"`
    ColorClean   string `toml:"color_clean"`
    ColorCpuLow  string `toml:"color_cpu_low"`
    ColorCpuMed  string `toml:"color_cpu_med"`
    ColorCpuHigh string `toml:"color_cpu_high"`
}

// ProcessesConfig defines which process names belong to each display category.
// Entries are matched case-insensitively against the process's friendly name.
type ProcessesConfig struct {
    Editors []string `toml:"editors"`
    Agents  []string `toml:"agents"`
    Servers []string `toml:"servers"`
    Shells  []string `toml:"shells"`
}

type Config struct {
    RefreshIntervalMs int             `toml:"refresh_interval_ms"`
    IgnoredSessions   []string        `toml:"ignored_sessions"`
    DefaultFormat     string          `toml:"default_format"`
    StatusBarFormat   string          `toml:"status_bar_format"`
    SidebarWidth      int             `toml:"sidebar_width"`
    Git               GitConfig       `toml:"git"`
    Theme             ThemeConfig     `toml:"theme"`
    Processes         ProcessesConfig `toml:"processes"`
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
        Processes: ProcessesConfig{
            Editors: []string{"nvim", "vim", "vi", "nano", "emacs", "hx", "micro", "helix"},
            Agents:  []string{"claude", "aider", "cursor", "copilot", "continue", "cody"},
            Servers: []string{
                "railway", "rails", "node", "deno", "bun",
                "python", "python3", "uvicorn", "gunicorn", "fastapi", "django", "flask",
                "cargo", "go", "air", "watchexec",
                "vite", "webpack", "next", "nuxt",
                "caddy", "nginx", "httpd",
            },
            Shells: []string{"zsh", "bash", "sh", "fish", "dash", "nu", "pwsh"},
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

// DefaultPath returns ~/.config/demux/demux.toml
func DefaultPath() string {
    home, _ := os.UserHomeDir()
    return home + "/.config/demux/demux.toml"
}
