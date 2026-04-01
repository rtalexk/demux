package config

import "time"

// Exported defaults for use by other packages (e.g. TUI tick interval).
const (
    DefaultRefreshIntervalMs = 3000
    DefaultGitTimeoutMs      = 500
    DefaultSidebarWidth      = 35

    // MinRefreshIntervalMs is the minimum accepted value for RefreshIntervalMs.
    MinRefreshIntervalMs = 100
    // MinSidebarWidth is the minimum accepted value for Sidebar.Width.
    MinSidebarWidth = 10
    // MinGitTimeoutWarnMs is the threshold below which a git timeout warning is issued.
    MinGitTimeoutWarnMs = 50
)

// TickInterval is the TUI refresh tick derived from the default refresh interval.
var TickInterval = time.Duration(DefaultRefreshIntervalMs) * time.Millisecond
