package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

func TestApplyStickyMode_DisablesQuitBinding(t *testing.T) {
	defer ResetStickyMode()
	ApplyStickyMode()

	qMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	if key.Matches(qMsg, keys.Quit.Binding) {
		t.Error("expected Quit to NOT match 'q' in sticky mode")
	}
	ctrlC := tea.KeyMsg{Type: tea.KeyCtrlC}
	if key.Matches(ctrlC, keys.Quit.Binding) {
		t.Error("expected Quit to NOT match ctrl+c in sticky mode")
	}
}

func TestApplyStickyMode_HidesQuitFromHelp(t *testing.T) {
	defer ResetStickyMode()
	ApplyStickyMode()

	for _, d := range allKeyDefs() {
		if strings.Contains(d.Binding.Help().Desc, "quit") {
			t.Errorf("help overlay still contains quit row in sticky mode: %+v", d)
		}
	}
}

func TestAllKeyDefs_NormalIncludesQuit(t *testing.T) {
	ResetStickyMode()

	found := false
	for _, d := range allKeyDefs() {
		if strings.Contains(d.Binding.Help().Desc, "quit") {
			found = true
		}
	}
	if !found {
		t.Error("expected default help overlay to include quit row")
	}
}

func TestResetStickyMode_RestoresQuit(t *testing.T) {
	ApplyStickyMode()
	ResetStickyMode()

	qMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	if !key.Matches(qMsg, keys.Quit.Binding) {
		t.Error("expected Quit to match 'q' after ResetStickyMode")
	}
}
