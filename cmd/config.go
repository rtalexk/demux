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

# Default output format for CLI commands: text | table | json
default_format = "text"

# Status bar format string (tmux #(...) syntax)
status_bar_format = "tmux"

# Sidebar width in columns
sidebar_width = 30

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
`

func init() {
    configCmd.AddCommand(configInitCmd)
    rootCmd.AddCommand(configCmd)
}
