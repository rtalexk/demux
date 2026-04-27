package cmd

import (
	"strings"
	"testing"

	"github.com/rtalexk/demux/internal/session"
)

func TestResolveTarget_PositionalFound(t *testing.T) {
	sessions := []session.Session{
		{DisplayName: "alpha", IsLive: true},
		{DisplayName: "beta", IsConfig: true, Config: &session.ConfigEntry{Name: "beta", Path: "/tmp"}},
	}
	got, err := resolveConnectTarget(sessions, "beta")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.DisplayName != "beta" {
		t.Errorf("got %q, want beta", got.DisplayName)
	}
}

func TestResolveTarget_PositionalUnknown(t *testing.T) {
	sessions := []session.Session{{DisplayName: "alpha", IsLive: true}}
	_, err := resolveConnectTarget(sessions, "missing")
	if err == nil {
		t.Fatal("expected error for unknown name")
	}
	if !strings.Contains(err.Error(), `"missing"`) {
		t.Errorf("error should mention name, got: %v", err)
	}
}

func TestValidateConnectArgs_RequiresNameOrFuzzy(t *testing.T) {
	if err := validateConnectArgs("", false); err == nil {
		t.Error("expected error when neither name nor --fuzzy provided")
	}
}

func TestValidateConnectArgs_BothNameAndFuzzy(t *testing.T) {
	if err := validateConnectArgs("alpha", true); err == nil {
		t.Error("expected error when both name and --fuzzy provided")
	}
}

func TestValidateConnectArgs_NameOnly(t *testing.T) {
	if err := validateConnectArgs("alpha", false); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateConnectArgs_FuzzyOnly(t *testing.T) {
	if err := validateConnectArgs("", true); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
