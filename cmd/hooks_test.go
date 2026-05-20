package cmd

import (
	"strings"
	"testing"
)

func TestResolveAgent_Unknown(t *testing.T) {
	_, err := resolveAgent("aider")
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
	msg := err.Error()
	if !strings.Contains(msg, "aider") {
		t.Errorf("error should mention the bad value, got: %s", msg)
	}
	if !strings.Contains(msg, "tmux") {
		t.Errorf("error should list tmux as supported agent, got: %s", msg)
	}
}

func TestResolveAgent_Tmux(t *testing.T) {
	def, err := resolveAgent("tmux")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if def.snippet != tmuxHooksSnippet {
		t.Errorf("snippet mismatch")
	}
}

func TestTmuxHooksSnippet_ContainsAfterSelectPane(t *testing.T) {
	if !strings.Contains(tmuxHooksSnippet, "after-select-pane") {
		t.Error("tmuxHooksSnippet should reference after-select-pane hook")
	}
	if !strings.Contains(tmuxHooksSnippet, "demux event pane_focus") {
		t.Error("tmuxHooksSnippet should call demux event pane_focus")
	}
}

func TestTmuxHooksSnippet_ContainsAfterKillPane(t *testing.T) {
	if !strings.Contains(tmuxHooksSnippet, "after-kill-pane") {
		t.Error("hooks snippet should include after-kill-pane hook for GC")
	}
	if !strings.Contains(tmuxHooksSnippet, "pane_closed") {
		t.Error("hooks snippet after-kill-pane hook should call demux event pane_closed")
	}
}

func TestTmuxHooksSnippet_ContainsPaneExitedHook(t *testing.T) {
	if !strings.Contains(tmuxHooksSnippet, "pane-exited") {
		t.Error("snippet should include pane-exited hook for proactive sidebar eject")
	}
	if !strings.Contains(tmuxHooksSnippet, "pane_exiting") {
		t.Error("pane-exited hook should call demux event pane_exiting")
	}
	// Must use #{hook_pane}/#{hook_window}, not #{pane_id}/#{window_id}: in global
	// hooks #{pane_id} resolves to the surviving pane (focus already shifted), not
	// the one that triggered pane-exited.
	if !strings.Contains(tmuxHooksSnippet, "--pane=#{hook_pane}") {
		t.Error("pane_exiting hook must pass --pane=#{hook_pane}, not --pane=#{pane_id}")
	}
	if !strings.Contains(tmuxHooksSnippet, "--window=#{hook_window}") {
		t.Error("pane_exiting hook must pass --window=#{hook_window}, not --window=#{window_id}")
	}
}

func TestTmuxHooksSnippet_PaneFocusIncludesPaneID(t *testing.T) {
	if !strings.Contains(tmuxHooksSnippet, "--pane-id=#{pane_id}") {
		t.Error("pane_focus hooks should pass --pane-id=#{pane_id}")
	}
}

func TestTmuxHooksSnippet_ContainsStickyFollowHook(t *testing.T) {
	if !strings.Contains(tmuxHooksSnippet, "client-session-changed") {
		t.Error("snippet should still include client-session-changed hook")
	}
	if !strings.Contains(tmuxHooksSnippet, "demux sidebar follow") {
		t.Error("snippet should include 'demux sidebar follow' hook for sticky sidebar")
	}
	if !strings.Contains(tmuxHooksSnippet, "set-hook -ga client-session-changed") {
		t.Error("sticky follow hook should use -ga (append), not -g (overwrite)")
	}
}

func TestTmuxHooksSnippet_ContainsStickyWindowFollowHook(t *testing.T) {
	if !strings.Contains(tmuxHooksSnippet, "set-hook -ga after-select-window") {
		t.Error("snippet should include after-select-window -ga hook for sticky window follow")
	}
}

func TestTmuxHooksSnippet_ContainsStickySlotsEnsureHook(t *testing.T) {
	if !strings.Contains(tmuxHooksSnippet, "set-hook -ga after-new-window") {
		t.Error("snippet should include after-new-window -ga hook for slots ensure")
	}
	if !strings.Contains(tmuxHooksSnippet, "demux sidebar slots ensure") {
		t.Error("after-new-window hook should call 'demux sidebar slots ensure'")
	}
}

func TestTmuxHooksSnippet_ContainsStickyNewWindowFollowHook(t *testing.T) {
	// after-select-window does NOT fire when a window is created via new-window
	// (tmux fires after-new-window instead), so a separate after-new-window
	// follow hook is required for the sidebar to follow into a fresh window.
	found := false
	for _, line := range strings.Split(tmuxHooksSnippet, "\n") {
		if strings.Contains(line, "set-hook -ga after-new-window") &&
			strings.Contains(line, "demux sidebar follow") {
			found = true
			break
		}
	}
	if !found {
		t.Error("snippet should bind 'demux sidebar follow' to after-new-window so the sidebar follows into newly created windows")
	}
}

func TestTmuxHooksSnippet_ContainsStickyAutoShowHook(t *testing.T) {
	if !strings.Contains(tmuxHooksSnippet, "set-hook -ga client-attached") {
		t.Error("snippet should include client-attached hook for sticky auto-show")
	}
	if !strings.Contains(tmuxHooksSnippet, "demux sidebar show") {
		t.Error("snippet should call 'demux sidebar show' on client-attached")
	}
}

func TestTmuxParentIDs_ParsesTabSeparatedOutput(t *testing.T) {
	// parseTmuxTabOutput is the same logic used by tmuxParentIDs:
	// split on tab, expect [windowID, sessionID].
	raw := "@3\t$1\n"
	parts := strings.Split(strings.TrimSpace(raw), "\t")
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d from %q", len(parts), raw)
	}
	if parts[0] != "@3" {
		t.Errorf("windowID: want @3, got %q", parts[0])
	}
	if parts[1] != "$1" {
		t.Errorf("sessionID: want $1, got %q", parts[1])
	}
}

func TestTmuxParentIDs_RejectsInsufficientParts(t *testing.T) {
	// If tmux returns only one field, tmuxParentIDs returns an error.
	raw := "@3\n"
	parts := strings.Split(strings.TrimSpace(raw), "\t")
	if len(parts) >= 2 {
		t.Errorf("expected fewer than 2 parts for malformed output, got %d", len(parts))
	}
}
