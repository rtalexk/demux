package cmd

import (
    "fmt"

    "github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
    Use:   "config",
    Short: "Config-related subcommands",
}

var configInitCmd = &cobra.Command{
    Use:   "init",
    Short: "Print an example config file with defaults",
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Print(defaultConfigTOML)
        return nil
    },
}

const defaultConfigTOML = `# demux configuration
# Default path: ~/.config/demux/demux.toml

# How often to refresh session/process data (milliseconds)
refresh_interval_ms = 2000

# Sessions to hide from the sidebar
ignored_sessions = []

# Processes to hide from the process list (children are promoted up)
ignored_processes = ["zsh", "bash", "fish", "sh", "dash", "nu", "pwsh"]

# Default output format for CLI commands: text | table | json
default_format = "text"

# Status bar format string (tmux #(...) syntax)
status_bar_format = "tmux"

# Sidebar width in columns
sidebar_width = 30

# When the ! alert filter is active, which windows to show: all | alerts_only
alert_filter_windows = "all"

# Path prefix aliases — shorten verbose absolute paths in the TUI.
# Both prefix and replace support environment variables.
# Longest matching prefix wins. Example:
# [[path_aliases]]
# prefix = "$HOME"
# replace = "~"

[git]
# Enable git status indicators in the sidebar
enabled = true

# Timeout for git operations (milliseconds)
timeout_ms = 500

# What to show when git times out: cached | hide | error
on_timeout = "cached"

# Text shown when git info is unavailable
fallback_display = "—"

# Text shown when git returns an error
error_display = "git err"

[git.pr]
# Enable pull-request info in the detail panel
enabled = false

# ---------------------------------------------------------------------------
# Theme (Catppuccin Mocha)
# ---------------------------------------------------------------------------
[theme]
# Structure & chrome
color_bg       = "#0d0d14"
color_surface  = "#13131a"
color_raised   = "#1e1e2e"
color_selected = "#2a2a4a"
color_border   = "#313244"

# Text hierarchy
color_fg_primary = "#cdd6f4"
color_fg_subtext = "#a6adc8"
color_fg_muted   = "#9399b2"
color_fg_dim     = "#6c7086"
color_fg_ghost   = "#45475a"

# Process type colours
color_session     = "#89b4fa"
color_proc_claude = "#cba6f7"
color_proc_server = "#89dceb"
color_proc_editor = "#b4befe"
color_proc_child  = "#a6adc8"

# Git status indicators
color_git_dirty  = "#f9e2af"
color_git_behind = "#74c7ec"
color_git_ahead  = "#a6e3a1"

# Alert icons
color_alert_info  = "#89b4fa"
color_alert_warn  = "#f9e2af"
color_alert_error = "#f38ba8"

# Semantic colours
color_port     = "#a6e3a1"
color_port_bg  = "#1a3a2a"
color_clean    = "#a6e3a1"
color_cpu_low  = "#7f849c"
color_cpu_med  = "#f9e2af"
color_cpu_high = "#f38ba8"

# Process type classification — controls how process names are coloured
[theme.processes]
editors = ["nvim", "vim", "vi", "nano", "emacs", "hx", "micro", "helix"]
agents  = ["claude", "aider", "cursor", "copilot", "continue", "cody"]
servers = [
  "railway", "rails", "node", "deno", "bun",
  "python", "python3", "uvicorn", "gunicorn", "fastapi", "django", "flask",
  "cargo", "go", "air", "watchexec",
  "vite", "webpack", "next", "nuxt",
  "caddy", "nginx", "httpd",
]
shells = ["zsh", "bash", "sh", "fish", "dash", "nu", "pwsh"]
`

func init() {
    configCmd.AddCommand(configInitCmd)
    rootCmd.AddCommand(configCmd)
}
