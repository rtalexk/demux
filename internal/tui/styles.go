package tui

import (
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/rtalexk/demux/internal/config"
	"github.com/rtalexk/demux/internal/db"
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
	confirmStyle  lipgloss.Style

	// Sidebar text
	sessionStyle lipgloss.Style

	// Process list text
	paneHeaderStyle   lipgloss.Style
	windowHeaderStyle lipgloss.Style
	panePathStyle     lipgloss.Style
	paneSepStyle      lipgloss.Style
	procLine1Style    lipgloss.Style
	procLine2Style    lipgloss.Style
	paneIdleStyle     lipgloss.Style

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
	selectedSession  lipgloss.Style
	selectedInactive lipgloss.Style

	// Tree connectors
	treeConnectorStyle lipgloss.Style

	// Git indicators
	gitAheadStyle  lipgloss.Style
	gitBehindStyle lipgloss.Style
	gitDirtyStyle  lipgloss.Style

	// Session icon
	sessionIconStyle lipgloss.Style

	// Watch indicator
	watchStyle lipgloss.Style

	// Todo indicator
	todoStyle lipgloss.Style
)

// initStyles rebuilds every style using the given theme and merges proc-type
// name lists with any user-configured extras. Call once from New().
func initStyles(t Theme, procs config.ProcessesConfig, ignoredProcs []string) {
	activeTheme = t

	activeProcEditors = procs.Editors
	activeProcAgents = procs.Agents
	activeProcServers = procs.Servers
	activeProcShells = procs.Shells
	activeIgnoredProcs = ignoredProcs

	borderActive = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.ColorSession)
	borderInactive = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.ColorBorder)
	detailBorder = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.ColorBorder)

	procBorderActive = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.ColorSession)
	procBorderInactive = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.ColorBorder)

	helpStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.ColorSession).Padding(1, 2)
	helpDescStyle = lipgloss.NewStyle().Foreground(t.ColorFgMuted)
	confirmStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.ColorSession).Padding(1, 1)

	sessionStyle = lipgloss.NewStyle().Bold(true).Foreground(t.ColorSession)
	paneHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(t.ColorFgSubtext)
	windowHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(t.ColorFgSubtext)
	panePathStyle = lipgloss.NewStyle().Foreground(t.ColorSession)
	paneSepStyle = lipgloss.NewStyle().Foreground(t.ColorFgGhost)
	procLine1Style = lipgloss.NewStyle().Foreground(t.ColorFgPrimary)
	procLine2Style = lipgloss.NewStyle().Foreground(t.ColorFgMuted)
	paneIdleStyle = lipgloss.NewStyle().Foreground(t.ColorFgDim).Italic(true)

	hintStyle = lipgloss.NewStyle().Foreground(t.ColorFgMuted)
	detailLabelStyle = lipgloss.NewStyle().Foreground(t.ColorFgMuted).Width(detailLabelWidth)
	detailValueStyle = lipgloss.NewStyle().Foreground(t.ColorFgPrimary)
	noSelectionStyle = lipgloss.NewStyle().Foreground(t.ColorFgMuted).Italic(true)
	spinnerStyle = lipgloss.NewStyle().Foreground(t.ColorFgMuted)
	statLabelStyle = lipgloss.NewStyle().Foreground(t.ColorFgDim)
	statValueStyle = lipgloss.NewStyle().Foreground(t.ColorFgMuted)

	selectedBG = lipgloss.NewStyle().Background(t.ColorSelected).Foreground(t.ColorFgPrimary)
	selectedSession = lipgloss.NewStyle().Foreground(t.ColorSession).Background(t.ColorSelected)
	selectedInactive = lipgloss.NewStyle().Foreground(t.ColorSession)

	treeConnectorStyle = lipgloss.NewStyle().Foreground(t.ColorFgMuted)

	gitAheadStyle = lipgloss.NewStyle().Foreground(t.ColorGitAhead)
	gitBehindStyle = lipgloss.NewStyle().Foreground(t.ColorGitBehind)
	gitDirtyStyle = lipgloss.NewStyle().Foreground(t.ColorGitDirty)

	sessionIconStyle = lipgloss.NewStyle().Foreground(t.ColorFgMuted)

	watchStyle = lipgloss.NewStyle().Foreground(t.ColorWatch)
	todoStyle = lipgloss.NewStyle().Foreground(t.ColorTodo)
}

// ageDrivenValue returns the effective display value for a state.
// If the state is Done and thresholdSecs > 0 and enough time has passed since
// it was last updated, it returns StateIdle so the TUI renders an idle icon
// without touching the DB record.
func ageDrivenValue(st db.ToolState, thresholdSecs int) db.StateValue {
	if thresholdSecs > 0 && st.Value == db.StateDone {
		if time.Since(st.UpdatedAt) >= time.Duration(thresholdSecs)*time.Second {
			return db.StateIdle
		}
	}
	return st.Value
}

// stateIcon renders the configured icon for the given state value, colored by theme.
func stateIcon(value db.StateValue) string {
	switch value {
	case db.StateWorking:
		return lipgloss.NewStyle().Foreground(activeTheme.ColorStateWorking).Render(activeTheme.IconStateWorking)
	case db.StateWaiting:
		return lipgloss.NewStyle().Foreground(activeTheme.ColorStateWaiting).Render(activeTheme.IconStateWaiting)
	case db.StateDone:
		return lipgloss.NewStyle().Foreground(activeTheme.ColorStateDone).Render(activeTheme.IconStateDone)
	case db.StateError:
		return lipgloss.NewStyle().Foreground(activeTheme.ColorStateError).Bold(true).Render(activeTheme.IconStateError)
	case db.StateFlagged:
		return lipgloss.NewStyle().Foreground(activeTheme.ColorStateFlagged).Render(activeTheme.IconStateFlagged)
	case db.StateIdle:
		return lipgloss.NewStyle().Foreground(activeTheme.ColorStateIdle).Render(activeTheme.IconStateIdle)
	}
	return ""
}

// stateIconOnBG renders the state icon with a background color applied.
func stateIconOnBG(value db.StateValue, bg lipgloss.Color) string {
	switch value {
	case db.StateWorking:
		return lipgloss.NewStyle().Foreground(activeTheme.ColorStateWorking).Background(bg).Render(activeTheme.IconStateWorking)
	case db.StateWaiting:
		return lipgloss.NewStyle().Foreground(activeTheme.ColorStateWaiting).Background(bg).Render(activeTheme.IconStateWaiting)
	case db.StateDone:
		return lipgloss.NewStyle().Foreground(activeTheme.ColorStateDone).Background(bg).Render(activeTheme.IconStateDone)
	case db.StateError:
		return lipgloss.NewStyle().Foreground(activeTheme.ColorStateError).Bold(true).Background(bg).Render(activeTheme.IconStateError)
	case db.StateFlagged:
		return lipgloss.NewStyle().Foreground(activeTheme.ColorStateFlagged).Background(bg).Render(activeTheme.IconStateFlagged)
	case db.StateIdle:
		return lipgloss.NewStyle().Foreground(activeTheme.ColorStateIdle).Background(bg).Render(activeTheme.IconStateIdle)
	}
	return ""
}

// stateBadge renders the state message with a state-based background color.
func stateBadge(value db.StateValue, msg string) string {
	if msg == "" {
		return ""
	}
	var fg, bg lipgloss.Color
	switch value {
	case db.StateWorking:
		fg, bg = activeTheme.ColorStateWorking, activeTheme.ColorStateWorkingBg
	case db.StateWaiting:
		fg, bg = activeTheme.ColorStateWaiting, activeTheme.ColorStateWaitingBg
	case db.StateDone:
		fg, bg = activeTheme.ColorStateDone, activeTheme.ColorStateDoneBg
	case db.StateError:
		fg, bg = activeTheme.ColorStateError, activeTheme.ColorStateErrorBg
	case db.StateFlagged:
		fg, bg = activeTheme.ColorStateFlagged, activeTheme.ColorStateFlaggedBg
	case db.StateIdle:
		fg, bg = activeTheme.ColorStateIdle, activeTheme.ColorStateIdleBg
	default:
		return msg
	}
	return lipgloss.NewStyle().Foreground(fg).Background(bg).Render(" " + msg + " ")
}

// paneStateIndicator returns the inline state text for a pane header row.
// Returns "" when st is nil. Shows icon + message badge; falls back to the
// state name when no custom message is set.
func paneStateIndicator(st *db.ToolState) string {
	if st == nil {
		return ""
	}
	icon := stateIcon(st.Value)
	msg := st.Message
	if msg == "" {
		msg = st.Value.String()
	}
	return icon + " " + stateBadge(st.Value, msg)
}
