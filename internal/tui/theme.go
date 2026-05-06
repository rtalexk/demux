package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/rtalexk/demux/internal/config"
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

	ColorProcLabelFG lipgloss.Color
	ColorProcLabelBG lipgloss.Color

	// Git status
	ColorGitDirty  lipgloss.Color
	ColorGitBehind lipgloss.Color
	ColorGitAhead  lipgloss.Color

	// States
	ColorStateWorking   lipgloss.Color
	ColorStateWorkingBg lipgloss.Color
	ColorStateWaiting   lipgloss.Color
	ColorStateWaitingBg lipgloss.Color
	ColorStateDone      lipgloss.Color
	ColorStateDoneBg    lipgloss.Color
	ColorStateError     lipgloss.Color
	ColorStateErrorBg   lipgloss.Color
	ColorStateFlagged   lipgloss.Color
	ColorStateFlaggedBg lipgloss.Color
	ColorStateIdle      lipgloss.Color
	ColorStateIdleBg    lipgloss.Color

	IconStateWorking string
	IconStateWaiting string
	IconStateDone    string
	IconStateError   string
	IconStateFlagged string
	IconStateIdle    string

	IconWatch  string
	ColorWatch lipgloss.Color

	IconTodo  string
	ColorTodo lipgloss.Color
	IconNote  string

	IconTmuxSession string
	IconCfgSession  string

	// Semantic
	ColorPort    lipgloss.Color
	ColorPortBg  lipgloss.Color
	ColorClean   lipgloss.Color
	ColorCpuLow  lipgloss.Color
	ColorCpuMed  lipgloss.Color
	ColorCpuHigh lipgloss.Color

	// Search
	ColorFgSearchHighlight lipgloss.Color
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

		ColorProcLabelFG: lipgloss.Color(tc.ColorProcLabelFG),
		ColorProcLabelBG: lipgloss.Color(tc.ColorProcLabelBG),

		ColorGitDirty:  lipgloss.Color(tc.ColorGitDirty),
		ColorGitBehind: lipgloss.Color(tc.ColorGitBehind),
		ColorGitAhead:  lipgloss.Color(tc.ColorGitAhead),

		ColorStateWorking:   lipgloss.Color(tc.ColorStateWorking),
		ColorStateWorkingBg: lipgloss.Color(tc.ColorStateWorkingBg),
		ColorStateWaiting:   lipgloss.Color(tc.ColorStateWaiting),
		ColorStateWaitingBg: lipgloss.Color(tc.ColorStateWaitingBg),
		ColorStateDone:      lipgloss.Color(tc.ColorStateDone),
		ColorStateDoneBg:    lipgloss.Color(tc.ColorStateDoneBg),
		ColorStateError:     lipgloss.Color(tc.ColorStateError),
		ColorStateErrorBg:   lipgloss.Color(tc.ColorStateErrorBg),
		ColorStateFlagged:   lipgloss.Color(tc.ColorStateFlagged),
		ColorStateFlaggedBg: lipgloss.Color(tc.ColorStateFlaggedBg),
		ColorStateIdle:      lipgloss.Color(tc.ColorStateIdle),
		ColorStateIdleBg:    lipgloss.Color(tc.ColorStateIdleBg),

		IconStateWorking: tc.IconStateWorking,
		IconStateWaiting: tc.IconStateWaiting,
		IconStateDone:    tc.IconStateDone,
		IconStateError:   tc.IconStateError,
		IconStateFlagged: tc.IconStateFlagged,
		IconStateIdle:    tc.IconStateIdle,

		IconWatch:  tc.IconWatch,
		ColorWatch: lipgloss.Color(tc.ColorWatch),

		IconTodo:  tc.IconTodo,
		ColorTodo: lipgloss.Color(tc.ColorTodo),
		IconNote:  tc.IconNote,

		IconTmuxSession: tc.IconTmuxSession,
		IconCfgSession:  tc.IconCfgSession,

		ColorPort:    lipgloss.Color(tc.ColorPort),
		ColorPortBg:  lipgloss.Color(tc.ColorPortBg),
		ColorClean:   lipgloss.Color(tc.ColorClean),
		ColorCpuLow:  lipgloss.Color(tc.ColorCpuLow),
		ColorCpuMed:  lipgloss.Color(tc.ColorCpuMed),
		ColorCpuHigh: lipgloss.Color(tc.ColorCpuHigh),

		ColorFgSearchHighlight: lipgloss.Color(tc.ColorFgSearchHighlight),
	}
}
