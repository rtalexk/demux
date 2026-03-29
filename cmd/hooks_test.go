package cmd

import (
    "strings"
    "testing"
)

func TestResolveAgent_Claude(t *testing.T) {
    def, err := resolveAgent("claude")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if def.stopMsg != "Claude finished" {
        t.Errorf("stopMsg = %q, want %q", def.stopMsg, "Claude finished")
    }
    if def.notifyFallback != "Claude notification" {
        t.Errorf("notifyFallback = %q, want %q", def.notifyFallback, "Claude notification")
    }
    if def.snippet != claudeHooksSnippet {
        t.Errorf("snippet mismatch")
    }
}

func TestResolveAgent_Unknown(t *testing.T) {
    _, err := resolveAgent("aider")
    if err == nil {
        t.Fatal("expected error for unknown agent")
    }
    msg := err.Error()
    if !strings.Contains(msg, "aider") {
        t.Errorf("error should mention the bad value, got: %s", msg)
    }
    if !strings.Contains(msg, "claude") {
        t.Errorf("error should list supported agents, got: %s", msg)
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
    if def.stopMsg != "" {
        t.Errorf("stopMsg = %q, want empty", def.stopMsg)
    }
    if def.notifyFallback != "" {
        t.Errorf("notifyFallback = %q, want empty", def.notifyFallback)
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
