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

// DefaultTheme returns the Catppuccin Mocha palette.
func DefaultTheme() Theme {
	return Theme{
		ColorBg:       "#0d0d14",
		ColorSurface:  "#13131a",
		ColorRaised:   "#1e1e2e",
		ColorSelected: "#2a2a4a",
		ColorBorder:   "#313244",

		ColorFgPrimary: "#cdd6f4",
		ColorFgSubtext: "#a6adc8",
		ColorFgMuted:   "#7f849c",
		ColorFgDim:     "#45455a",
		ColorFgGhost:   "#313244",

		ColorSession:    "#89b4fa",
		ColorProcClaude: "#cba6f7",
		ColorProcServer: "#89dceb",
		ColorProcEditor: "#b4befe",
		ColorProcChild:  "#a6adc8",

		ColorGitDirty:  "#f9e2af",
		ColorGitBehind: "#74c7ec",
		ColorGitAhead:  "#a6e3a1",

		ColorAlertInfo:  "#89b4fa",
		ColorAlertWarn:  "#f9e2af",
		ColorAlertError: "#f38ba8",

		ColorPort:    "#a6e3a1",
		ColorPortBg:  "#1a3a2a",
		ColorClean:   "#a6e3a1",
		ColorCpuLow:  "#7f849c",
		ColorCpuMed:  "#f9e2af",
		ColorCpuHigh: "#f38ba8",
	}
}

// ThemeFromConfig applies any non-empty overrides from the config on top of the default theme.
func ThemeFromConfig(tc config.ThemeConfig) Theme {
	t := DefaultTheme()
	applyColor := func(dst *lipgloss.Color, src string) {
		if src != "" {
			*dst = lipgloss.Color(src)
		}
	}
	applyColor(&t.ColorBg, tc.ColorBg)
	applyColor(&t.ColorSurface, tc.ColorSurface)
	applyColor(&t.ColorRaised, tc.ColorRaised)
	applyColor(&t.ColorSelected, tc.ColorSelected)
	applyColor(&t.ColorBorder, tc.ColorBorder)

	applyColor(&t.ColorFgPrimary, tc.ColorFgPrimary)
	applyColor(&t.ColorFgSubtext, tc.ColorFgSubtext)
	applyColor(&t.ColorFgMuted, tc.ColorFgMuted)
	applyColor(&t.ColorFgDim, tc.ColorFgDim)
	applyColor(&t.ColorFgGhost, tc.ColorFgGhost)

	applyColor(&t.ColorSession, tc.ColorSession)
	applyColor(&t.ColorProcClaude, tc.ColorProcClaude)
	applyColor(&t.ColorProcServer, tc.ColorProcServer)
	applyColor(&t.ColorProcEditor, tc.ColorProcEditor)
	applyColor(&t.ColorProcChild, tc.ColorProcChild)

	applyColor(&t.ColorGitDirty, tc.ColorGitDirty)
	applyColor(&t.ColorGitBehind, tc.ColorGitBehind)
	applyColor(&t.ColorGitAhead, tc.ColorGitAhead)

	applyColor(&t.ColorAlertInfo, tc.ColorAlertInfo)
	applyColor(&t.ColorAlertWarn, tc.ColorAlertWarn)
	applyColor(&t.ColorAlertError, tc.ColorAlertError)

	applyColor(&t.ColorPort, tc.ColorPort)
	applyColor(&t.ColorPortBg, tc.ColorPortBg)
	applyColor(&t.ColorClean, tc.ColorClean)
	applyColor(&t.ColorCpuLow, tc.ColorCpuLow)
	applyColor(&t.ColorCpuMed, tc.ColorCpuMed)
	applyColor(&t.ColorCpuHigh, tc.ColorCpuHigh)
	return t
}
