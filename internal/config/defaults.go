package config

import "time"

// Exported defaults for use by other packages (e.g. TUI tick interval).
const (
	DefaultRefreshIntervalMs = 3000
	DefaultGitTimeoutMs      = 500
	DefaultSidebarWidth      = 35
	DefaultActiveSessionIcon = "►"

	// SidebarViewRow and SidebarViewCard are the accepted values for
	// SidebarConfig.SessionView.  TUI consumers should reference these
	// constants rather than inline string literals.
	SidebarViewRow  = "row"
	SidebarViewCard = "card"

	// CardSeparatorNone, CardSeparatorBlank, and CardSeparatorRule are the
	// accepted values for SidebarConfig.CardSeparator.
	CardSeparatorNone  = "none"
	CardSeparatorBlank = "blank"
	CardSeparatorRule  = "rule"

	// MinRefreshIntervalMs is the minimum accepted value for RefreshIntervalMs.
	MinRefreshIntervalMs = 100
	// MinSidebarWidth is the minimum accepted value for Sidebar.Width.
	MinSidebarWidth = 10
	// MinGitTimeoutWarnMs is the threshold below which a git timeout warning is issued.
	MinGitTimeoutWarnMs = 50

	// DefaultStickySidebarWidth is the initial width (in columns) used the first
	// time the sticky sidebar pane is created. After creation, the user resizes
	// the pane normally; Follow preserves whatever width is current at move time.
	DefaultStickySidebarWidth = 35
	// MinStickySidebarWidth is the minimum accepted value for Sidebar.Sticky.Width.
	MinStickySidebarWidth = 10
)

// TickInterval is the TUI refresh tick derived from the default refresh interval.
var TickInterval = time.Duration(DefaultRefreshIntervalMs) * time.Millisecond
