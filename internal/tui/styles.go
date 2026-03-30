package tui

import (
    "github.com/charmbracelet/lipgloss"
    "github.com/rtalex/demux/internal/config"
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
    helpStyle   lipgloss.Style
    filterStyle lipgloss.Style
    yankStyle   lipgloss.Style

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

    helpStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.ColorSession).Padding(1, 2)
    filterStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.ColorSession).Padding(0, 1)
    yankStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.ColorSession).Padding(1, 2)

    sessionStyle    = lipgloss.NewStyle().Bold(true).Foreground(t.ColorSession)
    paneHeaderStyle   = lipgloss.NewStyle().Bold(true).Foreground(t.ColorFgSubtext)
    windowHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(t.ColorFgPrimary)
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
}
