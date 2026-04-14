package db

import (
	"testing"
)

func TestParseTargetID_Pane(t *testing.T) {
	target, err := ParseTargetID("%12")
	if err != nil {
		t.Fatalf("ParseTargetID(%q) failed: %v", "%12", err)
	}
	if target.Type != TargetTypePane {
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
	if target.Type != TargetTypeWindow {
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
	if target.Type != TargetTypeSession {
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

func TestTargetFormat_Pane(t *testing.T) {
	target := Target{Type: TargetTypePane, ID: "%3"}
	paneMap := map[string]string{"%3": "mySession:1.0"}
	got := target.Format(paneMap, nil, nil)
	if got != "win1·p0" {
		t.Errorf("Format() = %q, want %q", got, "win1·p0")
	}
}

func TestTargetFormat_Pane_FallbackToRawID(t *testing.T) {
	target := Target{Type: TargetTypePane, ID: "%3"}
	got := target.Format(nil, nil, nil)
	if got != "%3" {
		t.Errorf("Format() = %q, want %q", got, "%3")
	}
}

func TestTargetFormat_Window(t *testing.T) {
	target := Target{Type: TargetTypeWindow, ID: "@2"}
	windowMap := map[string]string{"@2": "mySession:3"}
	got := target.Format(nil, windowMap, nil)
	if got != "win3" {
		t.Errorf("Format() = %q, want %q", got, "win3")
	}
}

func TestTargetFormat_Window_FallbackToRawID(t *testing.T) {
	target := Target{Type: TargetTypeWindow, ID: "@2"}
	got := target.Format(nil, nil, nil)
	if got != "@2" {
		t.Errorf("Format() = %q, want %q", got, "@2")
	}
}

func TestTargetFormat_Session(t *testing.T) {
	target := Target{Type: TargetTypeSession, ID: "$1"}
	sessionMap := map[string]string{"$1": "mySession"}
	got := target.Format(nil, nil, sessionMap)
	if got != "mySession" {
		t.Errorf("Format() = %q, want %q", got, "mySession")
	}
}

func TestTargetFormat_Session_FallbackToRawID(t *testing.T) {
	target := Target{Type: TargetTypeSession, ID: "$1"}
	got := target.Format(nil, nil, nil)
	if got != "$1" {
		t.Errorf("Format() = %q, want %q", got, "$1")
	}
}

func TestTargetFormat_UnknownType(t *testing.T) {
	target := Target{Type: "bogus", ID: "xyz"}
	got := target.Format(nil, nil, nil)
	if got != "xyz" {
		t.Errorf("Format() = %q, want raw ID %q for unknown type", got, "xyz")
	}
}
