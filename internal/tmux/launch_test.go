package tmux

import (
	"reflect"
	"testing"
)

func TestNewSessionDetached_PathRequired(t *testing.T) {
	if err := NewSessionDetached("test-session", ""); err == nil {
		t.Error("expected error for empty path")
	}
}

func TestNewSessionDetached_NameRequired(t *testing.T) {
	if err := NewSessionDetached("", "/tmp"); err == nil {
		t.Error("expected error for empty name")
	}
}

func TestNewSessionDetached_PathNotExist(t *testing.T) {
	if err := NewSessionDetached("test-session", "/tmp/demux-no-such-dir-xyz"); err == nil {
		t.Error("expected error for nonexistent path")
	}
}

func TestResolveConnectArgs_TmuxSet(t *testing.T) {
	got := resolveConnectArgs(true, "myapp")
	want := []string{"switch-client", "-t", "myapp"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("inside tmux: got %v, want %v", got, want)
	}
}

func TestResolveConnectArgs_TmuxUnset(t *testing.T) {
	got := resolveConnectArgs(false, "myapp")
	want := []string{"attach-session", "-t", "myapp"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("outside tmux: got %v, want %v", got, want)
	}
}
