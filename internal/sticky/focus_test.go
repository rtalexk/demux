package sticky

import (
	"errors"
	"strings"
	"testing"
)

func TestActivePane(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{pane_id}"] = fakeReply{out: "%7\n"}
	s := &Sticky{T: f}
	got, err := s.activePane()
	if err != nil {
		t.Fatalf("activePane: %v", err)
	}
	if got != "%7" {
		t.Errorf("activePane: want %%7, got %q", got)
	}
}

func TestActivePane_TmuxFails(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{pane_id}"] = fakeReply{err: errors.New("no server")}
	s := &Sticky{T: f}
	if _, err := s.activePane(); err == nil {
		t.Error("expected error to surface")
	}
}

func TestFocusPane(t *testing.T) {
	f := newFakeTmux()
	s := &Sticky{T: f}
	s.focusPane("%42")
	if len(f.runs) != 1 || strings.Join(f.runs[0], " ") != "select-pane -t %42" {
		t.Errorf("focusPane: want one \"select-pane -t %%42\" call, got %v", f.runs)
	}
}

func TestFocusPane_EmptyNoop(t *testing.T) {
	f := newFakeTmux()
	s := &Sticky{T: f}
	s.focusPane("")
	if len(f.runs) != 0 {
		t.Errorf("focusPane(\"\"): want no tmux calls, got %v", f.runs)
	}
}
