package config_test

import (
	"testing"

	"github.com/rtalexk/demux/internal/config"
)

func TestValidate_defaults_clean(t *testing.T) {
	cfg := config.Default()
	issues := cfg.Validate()
	errors := filterLevel(issues, "error")
	if len(errors) != 0 {
		t.Errorf("Default config should have no errors, got: %v", errors)
	}
}

func TestValidate_bad_mode(t *testing.T) {
	cfg := config.Default()
	cfg.Mode = "ultrawide"
	issues := cfg.Validate()
	if !hasIssue(issues, "error", "mode") {
		t.Error("expected error for invalid mode")
	}
}

func TestValidate_bad_refresh(t *testing.T) {
	cfg := config.Default()
	cfg.RefreshIntervalMs = 10
	issues := cfg.Validate()
	if !hasIssue(issues, "error", "refresh_interval_ms") {
		t.Error("expected error for refresh_interval_ms < 100")
	}
}

func TestValidate_bad_color(t *testing.T) {
	cfg := config.Default()
	cfg.Theme.ColorBg = "notahex"
	issues := cfg.Validate()
	if !hasIssue(issues, "warn", "color_bg") {
		t.Error("expected warn for invalid color_bg")
	}
}

func TestValidate_bad_log_level(t *testing.T) {
	cfg := config.Default()
	cfg.Log.Level = "verbose"
	issues := cfg.Validate()
	if !hasIssue(issues, "error", "log.level") {
		t.Error("expected error for invalid log level")
	}
}

// helpers
func filterLevel(issues []config.ValidationIssue, level string) []config.ValidationIssue {
	var out []config.ValidationIssue
	for _, i := range issues {
		if i.Level == level {
			out = append(out, i)
		}
	}
	return out
}

func hasIssue(issues []config.ValidationIssue, level, field string) bool {
	for _, i := range issues {
		if i.Level == level && i.Field == field {
			return true
		}
	}
	return false
}
