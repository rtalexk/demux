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
refresh_interval_ms = 3000

# Sessions to hide from the sidebar
ignored_sessions = []

# Processes to hide from the process list (children are promoted up)
ignored_processes = ["zsh", "bash", "fish", "sh", "dash", "nu", "pwsh"]

# Default output format for CLI commands: text | table | json
default_format = "text"

# UI layout mode: full | compact
# compact renders only the sidebar + search (no process list or detail panel)
# Pair compact mode with a narrower tmux popup, e.g.:
#   bind-key D display-popup -w 40 -h 80% -E "demux --compact"
mode = "full"

# Path prefix aliases — shorten verbose absolute paths in the TUI.
# Both prefix and replace support environment variables.
# Longest matching prefix wins. Example:
# [[path_aliases]]
# prefix = "$HOME"
# replace = "~"

[sidebar]
# Session files: ~/.config/demux/sessions.toml and ~/.config/demux/private.toml
# contain user-defined sessions. See docs for the [[session]] format.

# Sidebar width in columns
width = 35

# Show last-seen age indicator on each live session row (e.g. " 5s", "12m", " 3h", "14d")
show_last_seen = true

# Icon shown next to the currently attached tmux session
active_session_icon = "►"

# Where to position the sidebar cursor on open: current_session | first_session | state_session
# state_session focuses the first session with a visible state (based on configured sort order)
focus_on_open = "current_session"

# Start with cursor in the search input (also available as --search flag)
focus_search_on_open = false

# Initial sidebar filter on startup: t (tmux) | a (all) | g (config) | w (worktree) | ! (priority)
default_filter = "t"

# Session sort order — first key wins, missing keys filled from default order
# Valid keys: priority | last_seen | alphabetical
sort = ["priority", "last_seen", "alphabetical"]

# Which target to focus when switching to a session: default | newest | oldest
#   default  — let tmux use its last-focused window
#   newest   — navigate to most recently updated state window
#   oldest   — navigate to oldest updated state window
switch_focus = "default"

# Session view: row (single row, default) | card (two-row layout).
# session_view is the fallback for both modes; the *_mode_* overrides win
# when set. Use "" (or omit) to inherit from session_view.
session_view = "row"
session_view_mode_compact = ""
session_view_mode_full = ""

# Separator row between cards (card view only):
#   none   — no row, denser layout
#   blank  — empty row (default)
#   rule   — horizontal rule ('─') in the border color
card_separator = "blank"

# Sticky sidebar - a per-client tmux pane that runs 'demux --sticky' and
# follows the attached client across session switches.
# Controlled via:
#   demux sidebar toggle   (or show/hide)
# See README "Sticky sidebar" section for the tmux hook snippet that auto-shows
# the sidebar on attach.
[sidebar.sticky]

# Initial width (columns) the first time the pane is created. After that, the
# user resizes the pane normally; the width is preserved on every session move.
width = 35

# Opt in to slots mode: pre-create a reserved sidebar pane in every window
# (tagged with the @demux_slot tmux option and titled "demux-slot"). The
# sticky sidebar swaps between slots on window/session switch, avoiding the
# brief layout flicker of join-pane. Cost: one extra pane per window.
slots = false

# Focus the sidebar pane when it opens (split and slots mode alike).
focus_on_open = true

# When 'demux sidebar toggle' runs while the sidebar is open but not focused,
# move focus into it instead of closing it; toggle again from inside to close.
focus_before_toggle_close = false

# Automatically show the sticky sidebar when a tmux client attaches.
# Requires the demux managed block in ~/.tmux.conf (see "demux hooks install").
auto_show = false

# Sidebar process labels — render a single highest-count label per session.
# Globs match either the process name or any whitespace-separated token of the
# full cmdline (case-insensitive). Only the pane root and its direct children
# are inspected; deeper procs are ignored. If a parent matches a pattern, its
# children are skipped. On a count tie, the entry declared first wins.
# match accepts either a single glob string or an array of globs that share
# the same label/fg/bg.
# Example:
# [[sidebar.processes]]
# match = "claude*"
# label = "🤖"
#
# [[sidebar.processes]]
# match = "node*"
# label = "node"
#
# [[sidebar.processes]]
# match = ["python*", "uvicorn", "uv"]
# label = "py"
# fg = "#fee685"
# bg = "#3b3000"

[process_list]
# Right-align pane CWD paths in the process list, with a fill separator
path_right_align = false

[tui]
# Include working sessions in the ! attention filter (default: false — only error/flagged/waiting)
attention_filter_include_working = false

# Default message stored when flagging a session with 'd' in the sidebar
flag_default_message = "Come back"

# Seconds after which a "done" state is rendered as "idle" in the TUI (0 = disabled).
# The DB record is not changed; this is purely a visual override.
done_idle_after_secs = 60

[log]
# Log level: off | error | warn | info | debug
# Logs are written to ~/.local/share/demux/demux.log
level = "warn"

[status_bar]
# Show the status bar at the bottom of the screen
show = true

[git]
# Enable git status indicators in the sidebar
enabled = true

# Show a loading spinner in the status bar while git data is being fetched
show_spinner = false

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

# Default fg/bg for sidebar process labels (per-entry [[sidebar.processes]] overrides win)
color_proc_label_fg = "#cdd6f4"
color_proc_label_bg = ""

# Git status indicators
color_git_dirty  = "#f9e2af"
color_git_behind = "#74c7ec"
color_git_ahead  = "#a6e3a1"

# State icons and colours
icon_state_working  = "⟳"
color_state_working    = "#a6e3a1"
color_state_working_bg = "#1a3a2a"

icon_state_waiting  = "●"
color_state_waiting    = "#f9e2af"
color_state_waiting_bg = "#3d3500"

icon_state_done  = "✓"
color_state_done    = "#a6e3a1"
color_state_done_bg = "#1a3a2a"

icon_state_error = "✗"
color_state_error    = "#f38ba8"
color_state_error_bg = "#3d1020"

icon_state_flagged  = "⚑"
color_state_flagged    = "#cba6f7"
color_state_flagged_bg = "#2a1a4d"

icon_watch  = "·"
color_watch = "#89b4fa"

icon_todo  = "☑︎"
icon_note  = "✏"
color_todo = ""

icon_state_idle  = "○"
color_state_idle    = "#89dceb"
color_state_idle_bg = "#0d2530"

# Icon shown by "demux status" when there are no active states
icon_status_clean = "🟢"

# Session source icons
icon_tmux_session = "⊞"
icon_cfg_session  = "⚙︎"

# Semantic colours
color_port     = "#a6e3a1"
color_port_bg  = "#1a3a2a"
color_clean    = "#a6e3a1"
color_cpu_low  = "#7f849c"
color_cpu_med  = "#f9e2af"
color_cpu_high = "#f38ba8"

# Search match highlight color
color_fg_search_highlight = "#f9e2af"

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
