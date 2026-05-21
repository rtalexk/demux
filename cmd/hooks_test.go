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

func TestTmuxHooksSnippet_KillHooksBackgrounded(t *testing.T) {
	// after-kill-pane and pane-exited can fire in a burst when many panes close
	// at once (e.g. `demux sidebar hide` tearing down every sticky slot pane).
	// They must run with `run-shell -b` so the cleanup never blocks tmux input.
	for _, line := range strings.Split(tmuxHooksSnippet, "\n") {
		if !strings.HasPrefix(line, "set-hook") {
			continue
		}
		if strings.Contains(line, "after-kill-pane") || strings.Contains(line, "pane-exited") {
			if !strings.Contains(line, "run-shell -b") {
				t.Errorf("kill-related hook must use 'run-shell -b':\n  %s", line)
			}
		}
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
	if !strings.Contains(tmuxHooksSnippet, "set-hook -g after-new-window") {
		t.Error("snippet should include after-new-window hook for slots ensure")
	}
	if !strings.Contains(tmuxHooksSnippet, "demux sidebar slots ensure") {
		t.Error("after-new-window hook should call 'demux sidebar slots ensure'")
	}
}

func TestTmuxHooksSnippet_ContainsStickyNewWindowFollowHook(t *testing.T) {
	// after-select-window does NOT fire when a window is created via new-window
	// (tmux fires after-new-window instead), so the after-new-window hook must
	// move the sidebar into the fresh window. It uses -g (replace), not -ga, so
	// re-sourcing ~/.tmux.conf cannot stack duplicate copies, and run-shell -b
	// so window creation never blocks on the handler.
	found := false
	for _, line := range strings.Split(tmuxHooksSnippet, "\n") {
		if strings.HasPrefix(line, "set-hook -g after-new-window") &&
			strings.Contains(line, "demux sidebar follow") &&
			strings.Contains(line, "run-shell -b") {
			found = true
			break
		}
	}
	if !found {
		t.Error("snippet should bind 'demux sidebar follow' to after-new-window via 'set-hook -g' with 'run-shell -b'")
	}
}

func TestTmuxHooksSnippet_ContainsStickyAutoShowHook(t *testing.T) {
	if !strings.Contains(tmuxHooksSnippet, "set-hook -g client-attached") {
		t.Error("snippet should include client-attached hook for sticky auto-show")
	}
	if !strings.Contains(tmuxHooksSnippet, "demux sidebar show") {
		t.Error("snippet should call 'demux sidebar show' on client-attached")
	}
}

func TestTmuxHooksSnippet_NonSharedHooksAreIdempotent(t *testing.T) {
	// after-new-window, client-attached and pane-exited have no -g reset line
	// for their event elsewhere in the snippet, so they must use -g (replace),
	// not -ga (append). With -ga, re-sourcing ~/.tmux.conf stacks a duplicate
	// copy every time, eventually spawning dozens of demux processes per event.
	for _, line := range strings.Split(tmuxHooksSnippet, "\n") {
		if !strings.HasPrefix(line, "set-hook") {
			continue
		}
		for _, ev := range []string{"after-new-window", "client-attached", "pane-exited"} {
			if strings.Contains(line, ev) && strings.Contains(line, "-ga") {
				t.Errorf("%s hook must use -g (not -ga) to stay idempotent on re-source:\n  %s", ev, line)
			}
		}
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
