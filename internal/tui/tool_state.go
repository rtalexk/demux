package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/rtalexk/demux/internal/config"
	"github.com/rtalexk/demux/internal/db"
)

type toolStateIdentity struct {
	marker   string
	label    string
	color    lipgloss.Color
	fallback bool
}

func resolveToolStateIdentity(st db.ToolState, cfg config.Config) toolStateIdentity {
	if st.Tool == "" || (st.Source == db.SourceUser && st.Value == db.StateFlagged) {
		return toolStateIdentity{}
	}

	if tool, ok := cfg.Tool(st.Tool); ok {
		label := tool.Name
		if label == "" {
			label = st.Tool
		}
		color := tool.Color
		if color == "" {
			color = cfg.Theme.ColorFgMuted
		}
		return toolStateIdentity{marker: tool.Icon, label: label, color: lipgloss.Color(color)}
	}

	return toolStateIdentity{
		marker:   "?",
		label:    shortenToolStateLabel(st.Tool),
		color:    lipgloss.Color(cfg.Theme.ColorFgMuted),
		fallback: true,
	}
}

func shortenToolStateLabel(id string) string {
	runes := []rune(id)
	if len(runes) <= 7 {
		return id
	}
	return string(runes[:6]) + "…"
}

func toolStateIndicator(st db.ToolState, cfg config.Config, bg lipgloss.Color) string {
	identity := resolveToolStateIdentity(st, cfg)
	icon := stateIcon(st.Value)
	if bg != "" {
		icon = stateIconOnBG(st.Value, bg)
	}
	if identity.marker == "" {
		return icon
	}

	markerStyle := lipgloss.NewStyle().Foreground(identity.color)
	if bg != "" {
		markerStyle = markerStyle.Background(bg)
	}
	marker := identity.marker
	if identity.fallback {
		marker += " " + identity.label
	}
	if bg != "" {
		return markerStyle.Render(marker+" ") + icon
	}
	return markerStyle.Render(marker) + " " + icon
}

func toolStateLabel(st db.ToolState, cfg config.Config) string {
	return resolveToolStateIdentity(st, cfg).label
}
