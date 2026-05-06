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

	ColorProcLabelFG string `toml:"color_proc_label_fg"`
	ColorProcLabelBG string `toml:"color_proc_label_bg"`

	ColorGitDirty  string `toml:"color_git_dirty"`
	ColorGitBehind string `toml:"color_git_behind"`
	ColorGitAhead  string `toml:"color_git_ahead"`

	ColorStateWorking   string `toml:"color_state_working"`
	ColorStateWorkingBg string `toml:"color_state_working_bg"`
	ColorStateWaiting   string `toml:"color_state_waiting"`
	ColorStateWaitingBg string `toml:"color_state_waiting_bg"`
	ColorStateDone      string `toml:"color_state_done"`
	ColorStateDoneBg    string `toml:"color_state_done_bg"`
	ColorStateError     string `toml:"color_state_error"`
	ColorStateErrorBg   string `toml:"color_state_error_bg"`
	ColorStateFlagged   string `toml:"color_state_flagged"`
	ColorStateFlaggedBg string `toml:"color_state_flagged_bg"`
	ColorStateIdle      string `toml:"color_state_idle"`
	ColorStateIdleBg    string `toml:"color_state_idle_bg"`

	IconStateWorking string `toml:"icon_state_working"`
	IconStateWaiting string `toml:"icon_state_waiting"`
	IconStateDone    string `toml:"icon_state_done"`
	IconStateError   string `toml:"icon_state_error"`
	IconStateFlagged string `toml:"icon_state_flagged"`
	IconStateIdle    string `toml:"icon_state_idle"`
	IconStatusClean  string `toml:"icon_status_clean"`

	IconWatch  string `toml:"icon_watch"`
	ColorWatch string `toml:"color_watch"`

	IconTodo  string `toml:"icon_todo"`
	ColorTodo string `toml:"color_todo"`
	IconNote  string `toml:"icon_note"`

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

// ProcessLabel declares a sidebar process indicator: a glob-matched process
// is rendered using Label, optionally styled with FG/BG. Empty colours fall
// back to the theme defaults (ColorProcLabelFG / ColorProcLabelBG).
type ProcessLabel struct {
	Match string `toml:"match"`
	Label string `toml:"label"`
	FG    string `toml:"fg"`
	BG    string `toml:"bg"`
}

type SidebarConfig struct {
	DefaultFilter     string         `toml:"default_filter"`
	FocusOnOpen       string         `toml:"focus_on_open"`
	FocusSearchOnOpen bool           `toml:"focus_search_on_open"`
	SearchSort        string         `toml:"search_sort"`
	ShowLastSeen      bool           `toml:"show_last_seen"`
	ActiveSessionIcon string         `toml:"active_session_icon"`
	Sort              []string       `toml:"sort"`
	SwitchFocus       string         `toml:"switch_focus"`
	Width             int            `toml:"width"`
	Processes         []ProcessLabel `toml:"processes"`
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

type TuiConfig struct {
	AttentionFilterIncludeWorking bool   `toml:"attention_filter_include_working"`
	FlagDefaultMessage            string `toml:"flag_default_message"`
	DoneIdleAfterSecs             int    `toml:"done_idle_after_secs"` // 0 = disabled
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
	Tui               TuiConfig         `toml:"tui"`
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
			DefaultFilter:     "t",
			FocusOnOpen:       "",
			SearchSort:        "score",
			Sort:              []string{"priority", "last_seen", "alphabetical"},
			SwitchFocus:       "severity",
			Width:             DefaultSidebarWidth,
			ShowLastSeen:      true,
			ActiveSessionIcon: DefaultActiveSessionIcon,
		},
		StatusBar: StatusBarConfig{Show: true},
		Log:       LogConfig{Level: "warn"},
		Tui: TuiConfig{
			AttentionFilterIncludeWorking: false,
			FlagDefaultMessage:            "Come back",
			DoneIdleAfterSecs:             60,
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

			ColorProcLabelFG: "#cdd6f4",
			ColorProcLabelBG: "",

			ColorGitDirty:  "#f9e2af",
			ColorGitBehind: "#74c7ec",
			ColorGitAhead:  "#a6e3a1",

			ColorStateWorking:   "#7f849c",
			ColorStateWorkingBg: "#1e1e2e",
			ColorStateWaiting:   "#f9e2af",
			ColorStateWaitingBg: "#3d3500",
			ColorStateDone:      "#a6e3a1",
			ColorStateDoneBg:    "#1a3a1a",
			ColorStateError:     "#f38ba8",
			ColorStateErrorBg:   "#3d1020",
			ColorStateFlagged:   "#cba6f7",
			ColorStateFlaggedBg: "#2a1a4d",
			ColorStateIdle:      "#89dceb",
			ColorStateIdleBg:    "#0d2530",

			IconStateWorking: "⟳",
			IconStateWaiting: "●",
			IconStateDone:    "✔",
			IconStateError:   "✗",
			IconStateFlagged: "🔖",
			IconStateIdle:    "○",
			IconStatusClean:  "🟢",

			IconWatch:  "·",
			ColorWatch: "#89b4fa",

			IconTodo:  "☑︎",
			ColorTodo: "",
			IconNote:  "✏",

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

	filteredProcs := cfg.Sidebar.Processes[:0]
	for _, p := range cfg.Sidebar.Processes {
		if p.Match == "" || p.Label == "" {
			fmt.Fprintf(os.Stderr, "demux: ignoring sidebar.processes entry with empty match/label: %+v\n", p)
			continue
		}
		if _, err := filepath.Match(p.Match, ""); err != nil {
			fmt.Fprintf(os.Stderr, "demux: ignoring sidebar.processes entry with bad glob %q: %v\n", p.Match, err)
			continue
		}
		filteredProcs = append(filteredProcs, p)
	}
	cfg.Sidebar.Processes = filteredProcs

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
