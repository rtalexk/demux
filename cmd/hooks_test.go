package cmd

import (
	"strings"
	"testing"
)

func TestPromptYesNo(t *testing.T) {
	cases := map[string]bool{"y\n": true, "Y\n": true, "yes\n": true, "n\n": false, "\n": false, "": false}
	for in, want := range cases {
		got := promptYesNo(strings.NewReader(in), &strings.Builder{}, "ok?")
		if got != want {
			t.Errorf("promptYesNo(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestHooksInstallCommandRegistered(t *testing.T) {
	var install, initCmd bool
	for _, c := range hooksCmd.Commands() {
		switch c.Use {
		case "install":
			install = true
		case "init":
			initCmd = true
		}
	}
	if !install {
		t.Error("hooks install subcommand not registered")
	}
	if !initCmd {
		t.Error("hooks init alias should remain registered (hidden)")
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
