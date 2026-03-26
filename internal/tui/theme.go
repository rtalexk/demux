package tui

import (
    "github.com/charmbracelet/lipgloss"
    "github.com/rtalex/demux/internal/config"
)

// Theme holds all colour values used across the TUI.
type Theme struct {
    // Structure & chrome
    ColorBg       lipgloss.Color
    ColorSurface  lipgloss.Color
    ColorRaised   lipgloss.Color
    ColorSelected lipgloss.Color
    ColorBorder   lipgloss.Color

    // Text hierarchy
    ColorFgPrimary lipgloss.Color
    ColorFgSubtext lipgloss.Color
    ColorFgMuted   lipgloss.Color
    ColorFgDim     lipgloss.Color
    ColorFgGhost   lipgloss.Color

    // Process types
    ColorSession    lipgloss.Color
    ColorProcClaude lipgloss.Color
    ColorProcServer lipgloss.Color
    ColorProcEditor lipgloss.Color
    ColorProcChild  lipgloss.Color

    // Git status
    ColorGitDirty  lipgloss.Color
    ColorGitBehind lipgloss.Color
    ColorGitAhead  lipgloss.Color

    // Alerts
    ColorAlertInfo  lipgloss.Color
    ColorAlertWarn  lipgloss.Color
    ColorAlertError lipgloss.Color

    // Semantic
    ColorPort    lipgloss.Color
    ColorPortBg  lipgloss.Color
    ColorClean   lipgloss.Color
    ColorCpuLow  lipgloss.Color
    ColorCpuMed  lipgloss.Color
    ColorCpuHigh lipgloss.Color
}

// ThemeFromConfig builds a Theme directly from the config values.
func ThemeFromConfig(tc config.ThemeConfig) Theme {
    return Theme{
        ColorBg:       lipgloss.Color(tc.ColorBg),
        ColorSurface:  lipgloss.Color(tc.ColorSurface),
        ColorRaised:   lipgloss.Color(tc.ColorRaised),
        ColorSelected: lipgloss.Color(tc.ColorSelected),
        ColorBorder:   lipgloss.Color(tc.ColorBorder),

        ColorFgPrimary: lipgloss.Color(tc.ColorFgPrimary),
        ColorFgSubtext: lipgloss.Color(tc.ColorFgSubtext),
        ColorFgMuted:   lipgloss.Color(tc.ColorFgMuted),
        ColorFgDim:     lipgloss.Color(tc.ColorFgDim),
        ColorFgGhost:   lipgloss.Color(tc.ColorFgGhost),

        ColorSession:    lipgloss.Color(tc.ColorSession),
        ColorProcClaude: lipgloss.Color(tc.ColorProcClaude),
        ColorProcServer: lipgloss.Color(tc.ColorProcServer),
        ColorProcEditor: lipgloss.Color(tc.ColorProcEditor),
        ColorProcChild:  lipgloss.Color(tc.ColorProcChild),

        ColorGitDirty:  lipgloss.Color(tc.ColorGitDirty),
        ColorGitBehind: lipgloss.Color(tc.ColorGitBehind),
        ColorGitAhead:  lipgloss.Color(tc.ColorGitAhead),

        ColorAlertInfo:  lipgloss.Color(tc.ColorAlertInfo),
        ColorAlertWarn:  lipgloss.Color(tc.ColorAlertWarn),
        ColorAlertError: lipgloss.Color(tc.ColorAlertError),

        ColorPort:    lipgloss.Color(tc.ColorPort),
        ColorPortBg:  lipgloss.Color(tc.ColorPortBg),
        ColorClean:   lipgloss.Color(tc.ColorClean),
        ColorCpuLow:  lipgloss.Color(tc.ColorCpuLow),
        ColorCpuMed:  lipgloss.Color(tc.ColorCpuMed),
        ColorCpuHigh: lipgloss.Color(tc.ColorCpuHigh),
    }
}
