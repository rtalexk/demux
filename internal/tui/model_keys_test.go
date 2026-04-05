package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestResolveFilterKey(t *testing.T) {
	tests := []struct {
		keyStr string
		want   SidebarFilter
		wantOk bool
	}{
		{"t", FilterTmux, true},
		{"a", FilterAll, true},
		{"c", FilterConfig, true},
		{"w", FilterWorktree, true},
		{"!", FilterPriority, true},
		{"x", "", false},
		{"q", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.keyStr, func(t *testing.T) {
			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.keyStr)}
			got, ok := resolveFilterKey(msg)
			if ok != tt.wantOk {
				t.Errorf("ok: got %v, want %v", ok, tt.wantOk)
			}
			if ok && got != tt.want {
				t.Errorf("filter: got %q, want %q", got, tt.want)
			}
		})
	}
}
