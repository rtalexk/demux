package config

import (
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "strings"

    "github.com/BurntSushi/toml"
)

var defaultSessionSortOrder = []string{"priority", "last_seen", "alphabetical"}

func normalizeSortKeys(user []string) []string {
    valid := map[string]bool{"priority": true, "last_seen": true, "alphabetical": true}
    seen := map[string]bool{}
    result := make([]string, 0, len(defaultSessionSortOrder))
    for _, k := range user {
        if valid[k] && !seen[k] {
            result = append(result, k)
            seen[k] = true
        }
    }
    for _, k := range defaultSessionSortOrder {
        if !seen[k] {
            result = append(result, k)
        }
    }
    return result
}

type PathAlias struct {
    Prefix  string `toml:"prefix"`
    Replace string `toml:"replace"`
}

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

// ThemeConfig holds all theme colour values and process type classification.
type ThemeConfig struct {
    Processes ProcessesConfig `toml:"processes"`

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
    RefreshIntervalMs  int         `toml:"refresh_interval_ms"`
    IgnoredSessions    []string    `toml:"ignored_sessions"`
    IgnoredProcesses   []string    `toml:"ignored_processes"`
    DefaultFormat      string      `toml:"default_format"`
    StatusBarFormat    string      `toml:"status_bar_format"`
    SidebarWidth       int         `toml:"sidebar_width"`
    AlertFilterWindows  string      `toml:"alert_filter_windows"` // "all" or "alerts_only"
    FocusOnOpen         string      `toml:"focus_on_open"`
    FocusOnOpenFallback string      `toml:"focus_on_open_fallback"`
    PanePathRightAlign  bool        `toml:"pane_path_right_align"`
    SessionsCollapsed   bool        `toml:"sessions_collapsed"`
    SessionSort         []string    `toml:"session_sort"`
    Git                 GitConfig   `toml:"git"`
    Theme              ThemeConfig `toml:"theme"`
    PathAliases        []PathAlias `toml:"path_aliases"`
}

func Default() Config {
    return Config{
        RefreshIntervalMs: 2000,
        IgnoredSessions:   []string{},
        IgnoredProcesses:  []string{"zsh", "bash", "fish", "sh", "dash", "nu", "pwsh"},
        DefaultFormat:     "text",
        StatusBarFormat:   "tmux",
        SidebarWidth:       30,
        AlertFilterWindows:  "all",
        FocusOnOpen:         "current_window",
        FocusOnOpenFallback: "current_window",
        SessionSort:        []string{"priority", "last_seen", "alphabetical"},
        Git: GitConfig{
            Enabled:         true,
            TimeoutMs:       500,
            OnTimeout:       "cached",
            FallbackDisplay: "—",
            ErrorDisplay:    "git err",
            PR:              GitPRConfig{Enabled: false},
        },
        Theme: ThemeConfig{
            ColorBg:       "#0d0d14",
            ColorSurface:  "#13131a",
            ColorRaised:   "#1e1e2e",
            ColorSelected: "#2a2a4a",
            ColorBorder:   "#313244",

            ColorFgPrimary: "#cdd6f4",
            ColorFgSubtext: "#a6adc8",
            ColorFgMuted:   "#9399b2",
            ColorFgDim:     "#6c7086",
            ColorFgGhost:   "#45475a",

            ColorSession:    "#89b4fa",
            ColorProcClaude: "#cba6f7",
            ColorProcServer: "#89dceb",
            ColorProcEditor: "#b4befe",
            ColorProcChild:  "#a6adc8",

            ColorGitDirty:  "#f9e2af",
            ColorGitBehind: "#74c7ec",
            ColorGitAhead:  "#a6e3a1",

            ColorAlertInfo:  "#89b4fa",
            ColorAlertWarn:  "#f9e2af",
            ColorAlertError: "#f38ba8",

            ColorPort:    "#a6e3a1",
            ColorPortBg:  "#1a3a2a",
            ColorClean:   "#a6e3a1",
            ColorCpuLow:  "#7f849c",
            ColorCpuMed:  "#f9e2af",
            ColorCpuHigh: "#f38ba8",

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
    // Expand env vars in path aliases, drop empty prefixes, sort longest-first.
    filtered := cfg.PathAliases[:0]
    for _, a := range cfg.PathAliases {
        a.Prefix = strings.ReplaceAll(os.ExpandEnv(a.Prefix), `\ `, " ")
        a.Replace = strings.ReplaceAll(os.ExpandEnv(a.Replace), `\ `, " ")
        if a.Prefix != "" {
            filtered = append(filtered, a)
        }
    }
    cfg.PathAliases = filtered
    sort.Slice(cfg.PathAliases, func(i, j int) bool {
        return len(cfg.PathAliases[i].Prefix) > len(cfg.PathAliases[j].Prefix)
    })
    cfg.SessionSort = normalizeSortKeys(cfg.SessionSort)
    return cfg, nil
}

// DefaultPath returns ~/.config/demux/demux.toml, or an error if the
// home directory cannot be determined.
func DefaultPath() (string, error) {
    home, err := os.UserHomeDir()
    if err != nil {
        return "", fmt.Errorf("home dir: %w", err)
    }
    return filepath.Join(home, ".config", "demux", "demux.toml"), nil
}
