package tui

import (
    "github.com/charmbracelet/lipgloss"
    "github.com/rtalexk/demux/internal/config"
)

// activeTheme is set once at startup by initStyles.
var activeTheme Theme

// Process-type name lists. Populated by initStyles from config.
var (
    activeProcEditors  []string
    activeProcAgents   []string
    activeProcServers  []string
    activeProcShells   []string
    activeIgnoredProcs []string
)

// All package-level lipgloss styles. Populated by initStyles.
var (
    // Panel borders
    borderActive   lipgloss.Style
    borderInactive lipgloss.Style
    detailBorder   lipgloss.Style

    procBorderActive   lipgloss.Style
    procBorderInactive lipgloss.Style

    // Overlays
    helpStyle     lipgloss.Style
    helpDescStyle lipgloss.Style
    yankStyle     lipgloss.Style

    // Sidebar text
    sessionStyle lipgloss.Style

    // Process list text
    paneHeaderStyle   lipgloss.Style
    windowHeaderStyle lipgloss.Style
    panePathStyle     lipgloss.Style
    paneSepStyle    lipgloss.Style
    procLine1Style  lipgloss.Style
    procLine2Style  lipgloss.Style
    paneIdleStyle   lipgloss.Style

    // Shared text
    hintStyle        lipgloss.Style
    detailLabelStyle lipgloss.Style
    detailValueStyle lipgloss.Style
    noSelectionStyle lipgloss.Style
    spinnerStyle     lipgloss.Style
    statLabelStyle   lipgloss.Style
    statValueStyle   lipgloss.Style

    // Selection
    selectedBG       lipgloss.Style
    selectedInactive lipgloss.Style

    // Tree connectors
    treeConnectorStyle lipgloss.Style

    // Git indicators
    gitAheadStyle  lipgloss.Style
    gitBehindStyle lipgloss.Style
    gitDirtyStyle  lipgloss.Style

    // Session icon
    sessionIconStyle lipgloss.Style
)

// initStyles rebuilds every style using the given theme and merges proc-type
// name lists with any user-configured extras. Call once from New().
func initStyles(t Theme, procs config.ProcessesConfig, ignoredProcs []string) {
    activeTheme = t

    activeProcEditors  = procs.Editors
    activeProcAgents   = procs.Agents
    activeProcServers  = procs.Servers
    activeProcShells   = procs.Shells
    activeIgnoredProcs = ignoredProcs

    borderActive   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.ColorSession)
    borderInactive = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.ColorBorder)
    detailBorder   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.ColorBorder)

    procBorderActive   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.ColorSession)
    procBorderInactive = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.ColorBorder)

    helpStyle     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.ColorSession).Padding(1, 2)
    helpDescStyle = lipgloss.NewStyle().Foreground(t.ColorFgMuted)
    yankStyle     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.ColorSession).Padding(1, 2)

    sessionStyle    = lipgloss.NewStyle().Bold(true).Foreground(t.ColorSession)
    paneHeaderStyle   = lipgloss.NewStyle().Bold(true).Foreground(t.ColorFgSubtext)
    windowHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(t.ColorFgSubtext)
    panePathStyle     = lipgloss.NewStyle().Foreground(t.ColorSession)
    paneSepStyle    = lipgloss.NewStyle().Foreground(t.ColorFgGhost)
    procLine1Style  = lipgloss.NewStyle().Foreground(t.ColorFgPrimary)
    procLine2Style  = lipgloss.NewStyle().Foreground(t.ColorFgMuted)
    paneIdleStyle   = lipgloss.NewStyle().Foreground(t.ColorFgDim).Italic(true)

    hintStyle        = lipgloss.NewStyle().Foreground(t.ColorFgMuted)
    detailLabelStyle = lipgloss.NewStyle().Foreground(t.ColorFgMuted).Width(10)
    detailValueStyle = lipgloss.NewStyle().Foreground(t.ColorFgPrimary)
    noSelectionStyle = lipgloss.NewStyle().Foreground(t.ColorFgMuted).Italic(true)
    spinnerStyle     = lipgloss.NewStyle().Foreground(t.ColorFgMuted)
    statLabelStyle   = lipgloss.NewStyle().Foreground(t.ColorFgDim)
    statValueStyle   = lipgloss.NewStyle().Foreground(t.ColorFgMuted)

    selectedBG       = lipgloss.NewStyle().Background(t.ColorSelected).Foreground(t.ColorFgPrimary)
    selectedInactive = lipgloss.NewStyle().Foreground(t.ColorSession)

    treeConnectorStyle = lipgloss.NewStyle().Foreground(t.ColorFgMuted)

    gitAheadStyle  = lipgloss.NewStyle().Foreground(t.ColorGitAhead)
    gitBehindStyle = lipgloss.NewStyle().Foreground(t.ColorGitBehind)
    gitDirtyStyle  = lipgloss.NewStyle().Foreground(t.ColorGitDirty)

    sessionIconStyle = lipgloss.NewStyle().Foreground(t.ColorFgMuted)
}

// alertIcon renders the configured icon for the given alert level, colored by theme.
func alertIcon(level string) string {
    switch level {
    case "info":
        return lipgloss.NewStyle().Foreground(activeTheme.ColorAlertInfo).Render(activeTheme.IconAlertInfo)
    case "warn":
        return lipgloss.NewStyle().Foreground(activeTheme.ColorAlertWarn).Render(activeTheme.IconAlertWarn)
    case "error":
        return lipgloss.NewStyle().Foreground(activeTheme.ColorAlertError).Bold(true).Render(activeTheme.IconAlertError)
    case "defer":
        return lipgloss.NewStyle().Foreground(activeTheme.ColorAlertDefer).Render(activeTheme.IconAlertDefer)
    }
    return ""
}

// alertIconOnBG renders the configured icon with a background color applied.
func alertIconOnBG(level string, bg lipgloss.Color) string {
    switch level {
    case "info":
        return lipgloss.NewStyle().Foreground(activeTheme.ColorAlertInfo).Background(bg).Render(activeTheme.IconAlertInfo)
    case "warn":
        return lipgloss.NewStyle().Foreground(activeTheme.ColorAlertWarn).Background(bg).Render(activeTheme.IconAlertWarn)
    case "error":
        return lipgloss.NewStyle().Foreground(activeTheme.ColorAlertError).Bold(true).Background(bg).Render(activeTheme.IconAlertError)
    case "defer":
        return lipgloss.NewStyle().Foreground(activeTheme.ColorAlertDefer).Background(bg).Render(activeTheme.IconAlertDefer)
    }
    return ""
}

// alertBadge renders the alert reason with a severity-based background color.
func alertBadge(level, reason string) string {
    var fg, bg lipgloss.Color
    switch level {
    case "info":
        fg, bg = activeTheme.ColorAlertInfo, activeTheme.ColorAlertInfoBg
    case "warn":
        fg, bg = activeTheme.ColorAlertWarn, activeTheme.ColorAlertWarnBg
    case "error":
        fg, bg = activeTheme.ColorAlertError, activeTheme.ColorAlertErrorBg
    case "defer":
        fg, bg = activeTheme.ColorAlertDefer, activeTheme.ColorAlertDeferBg
    default:
        return reason
    }
    return lipgloss.NewStyle().Foreground(fg).Background(bg).Render(" " + reason + " ")
}
