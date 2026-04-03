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
	ShowSpinner     bool        `toml:"show_spinner"`
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

	ColorAlertInfo    string `toml:"color_alert_info"`
	ColorAlertWarn    string `toml:"color_alert_warn"`
	ColorAlertError   string `toml:"color_alert_error"`
	ColorAlertDefer   string `toml:"color_alert_defer"`
	ColorAlertInfoBg  string `toml:"color_alert_info_bg"`
	ColorAlertWarnBg  string `toml:"color_alert_warn_bg"`
	ColorAlertErrorBg string `toml:"color_alert_error_bg"`
	ColorAlertDeferBg string `toml:"color_alert_defer_bg"`

	IconAlertInfo  string `toml:"icon_alert_info"`
	IconAlertWarn  string `toml:"icon_alert_warn"`
	IconAlertError string `toml:"icon_alert_error"`
	IconAlertDefer string `toml:"icon_alert_defer"`

	IconTmuxSession string `toml:"icon_tmux_session"`
	IconCfgSession  string `toml:"icon_cfg_session"`

	ColorPort    string `toml:"color_port"`
	ColorPortBg  string `toml:"color_port_bg"`
	ColorClean   string `toml:"color_clean"`
	ColorCpuLow  string `toml:"color_cpu_low"`
	ColorCpuMed  string `toml:"color_cpu_med"`
	ColorCpuHigh string `toml:"color_cpu_high"`

	ColorFgSearchHighlight string `toml:"color_fg_search_highlight"`
}

// ProcessesConfig defines which process names belong to each display category.
// Entries are matched case-insensitively against the process's friendly name.
type ProcessesConfig struct {
	Editors []string `toml:"editors"`
	Agents  []string `toml:"agents"`
	Servers []string `toml:"servers"`
	Shells  []string `toml:"shells"`
}

type SidebarConfig struct {
	DefaultFilter     string   `toml:"default_filter"`
	FocusOnOpen       string   `toml:"focus_on_open"`
	FocusSearchOnOpen bool     `toml:"focus_search_on_open"`
	SearchSort        string   `toml:"search_sort"`
	ShowLastSeen      bool     `toml:"show_last_seen"`
	Sort              []string `toml:"sort"`
	SwitchFocus       string   `toml:"switch_focus"`
	Width             int      `toml:"width"`
}

type ProcessListConfig struct {
	PathRightAlign bool `toml:"path_right_align"`
}

type StatusBarConfig struct {
	Show bool `toml:"show"`
}

type LogConfig struct {
	Level string `toml:"level"` // off|error|warn|info|debug
}

type AlertsConfig struct {
	DeferDefaultReason string `toml:"defer_default_reason"`
}

type Config struct {
	RefreshIntervalMs int               `toml:"refresh_interval_ms"`
	IgnoredSessions   []string          `toml:"ignored_sessions"`
	IgnoredProcesses  []string          `toml:"ignored_processes"`
	DefaultFormat     string            `toml:"default_format"`
	Mode              string            `toml:"mode"`
	Sidebar           SidebarConfig     `toml:"sidebar"`
	ProcessList       ProcessListConfig `toml:"process_list"`
	StatusBar         StatusBarConfig   `toml:"status_bar"`
	Log               LogConfig         `toml:"log"`
	Alerts            AlertsConfig      `toml:"alerts"`
	Git               GitConfig         `toml:"git"`
	Theme             ThemeConfig       `toml:"theme"`
	PathAliases       []PathAlias       `toml:"path_aliases"`
}

func Default() Config {
	return Config{
		RefreshIntervalMs: DefaultRefreshIntervalMs,
		IgnoredSessions:   []string{},
		IgnoredProcesses:  []string{"zsh", "bash", "fish", "sh", "dash", "nu", "pwsh"},
		DefaultFormat:     "text",
		Mode:              "full",
		Sidebar: SidebarConfig{
			DefaultFilter: "t",
			FocusOnOpen:   "alert_session",
			SearchSort:    "score",
			Sort:          []string{"priority", "last_seen", "alphabetical"},
			SwitchFocus:   "severity",
			Width:         DefaultSidebarWidth,
			ShowLastSeen:  true,
		},
		StatusBar: StatusBarConfig{Show: true},
		Log:       LogConfig{Level: "warn"},
		Alerts: AlertsConfig{
			DeferDefaultReason: "Come back",
		},
		Git: GitConfig{
			Enabled:         true,
			ShowSpinner:     true,
			TimeoutMs:       DefaultGitTimeoutMs,
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

			ColorAlertInfo:    "#89b4fa",
			ColorAlertWarn:    "#f9e2af",
			ColorAlertError:   "#f38ba8",
			ColorAlertDefer:   "#b4befe",
			ColorAlertInfoBg:  "#1a2a4d",
			ColorAlertWarnBg:  "#3d3500",
			ColorAlertErrorBg: "#3d1020",
			ColorAlertDeferBg: "#1e1e2e",

			IconAlertInfo:  "ℹ️",
			IconAlertWarn:  "⚠️",
			IconAlertError: "🚨",
			IconAlertDefer: "🔖",

			IconTmuxSession: "⊞",
			IconCfgSession:  "⚙︎",

			ColorPort:    "#a6e3a1",
			ColorPortBg:  "#1a3a2a",
			ColorClean:   "#a6e3a1",
			ColorCpuLow:  "#7f849c",
			ColorCpuMed:  "#f9e2af",
			ColorCpuHigh: "#f38ba8",

			ColorFgSearchHighlight: "#f9e2af",

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
	cfg.Sidebar.Sort = normalizeSortKeys(cfg.Sidebar.Sort)
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
