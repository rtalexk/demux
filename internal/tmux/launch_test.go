package tmux

import (
    "testing"
)

func TestNewSession_PathRequired(t *testing.T) {
    err := NewSession("test-session", "")
    if err == nil {
        t.Error("expected error for empty path")
    }
}

func TestNewSession_NameRequired(t *testing.T) {
    err := NewSession("", "/tmp")
    if err == nil {
        t.Error("expected error for empty name")
    }
}
