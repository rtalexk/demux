package db

import (
	"testing"
)

func TestParseTargetID_Pane(t *testing.T) {
	target, err := ParseTargetID("%12")
	if err != nil {
		t.Fatalf("ParseTargetID(%q) failed: %v", "%12", err)
	}
	if target.Type != "pane" {
		t.Errorf("type: want pane, got %q", target.Type)
	}
	if target.ID != "%12" {
		t.Errorf("ID: want %%12, got %q", target.ID)
	}
	if target.PaneID != "%12" {
		t.Errorf("PaneID: want %%12, got %q", target.PaneID)
	}
}

func TestParseTargetID_Window(t *testing.T) {
	target, err := ParseTargetID("@5")
	if err != nil {
		t.Fatalf("ParseTargetID(%q) failed: %v", "@5", err)
	}
	if target.Type != "window" {
		t.Errorf("type: want window, got %q", target.Type)
	}
	if target.ID != "@5" {
		t.Errorf("ID: want @5, got %q", target.ID)
	}
	if target.WindowID != "@5" {
		t.Errorf("WindowID: want @5, got %q", target.WindowID)
	}
}

func TestParseTargetID_Session(t *testing.T) {
	target, err := ParseTargetID("$1")
	if err != nil {
		t.Fatalf("ParseTargetID(%q) failed: %v", "$1", err)
	}
	if target.Type != "session" {
		t.Errorf("type: want session, got %q", target.Type)
	}
	if target.ID != "$1" {
		t.Errorf("ID: want $1, got %q", target.ID)
	}
	if target.SessionID != "$1" {
		t.Errorf("SessionID: want $1, got %q", target.SessionID)
	}
}

func TestParseTargetID_InvalidPrefix(t *testing.T) {
	_, err := ParseTargetID("invalid")
	if err == nil {
		t.Error("ParseTargetID(invalid) should fail")
	}
}

func TestParseTargetID_EmptyID(t *testing.T) {
	_, err := ParseTargetID("")
	if err == nil {
		t.Error("ParseTargetID() should fail on empty string")
	}
}
