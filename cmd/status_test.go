package cmd

import (
	"strings"
	"testing"

	"github.com/rtalexk/demux/internal/config"
	"github.com/rtalexk/demux/internal/db"
)

func TestCountAlertsByLevel(t *testing.T) {
	alerts := []db.Alert{
		{Level: "info"},
		{Level: "info"},
		{Level: "warn"},
		{Level: "error"},
		{Level: "error"},
		{Level: "error"},
		{Level: "defer"},
	}
	infos, warns, errors, defers := countAlertsByLevel(alerts)
	if infos != 2 {
		t.Errorf("infos: want 2, got %d", infos)
	}
	if warns != 1 {
		t.Errorf("warns: want 1, got %d", warns)
	}
	if errors != 3 {
		t.Errorf("errors: want 3, got %d", errors)
	}
	if defers != 1 {
		t.Errorf("defers: want 1, got %d", defers)
	}
}

func TestTmuxStatusParts_SeverityOrder(t *testing.T) {
	cfg := config.Default()
	out := tmuxStatusParts(1, 1, 1, 1, cfg)
	errIdx := strings.Index(out, cfg.Theme.IconAlertError)
	warnIdx := strings.Index(out, cfg.Theme.IconAlertWarn)
	infoIdx := strings.Index(out, cfg.Theme.IconAlertInfo)
	deferIdx := strings.Index(out, cfg.Theme.IconAlertDefer)
	if errIdx == -1 || warnIdx == -1 || infoIdx == -1 || deferIdx == -1 {
		t.Fatalf("missing icon in output: %q", out)
	}
	if !(errIdx < warnIdx && warnIdx < infoIdx && infoIdx < deferIdx) {
		t.Errorf("wrong severity order in output: %q", out)
	}
}

func TestTmuxStatusParts_ZeroDefer(t *testing.T) {
	cfg := config.Default()
	out := tmuxStatusParts(0, 0, 1, 0, cfg)
	if strings.Contains(out, cfg.Theme.IconAlertDefer) {
		t.Errorf("defer icon should not appear when defers=0: %q", out)
	}
}

func TestFormatStatusOutput_PlainOrder(t *testing.T) {
	cfg := config.Default()
	out := formatStatusOutput("plain", 1, 1, 1, 1, cfg)
	errIdx := strings.Index(out, "errors=")
	warnIdx := strings.Index(out, "warns=")
	infoIdx := strings.Index(out, "infos=")
	deferIdx := strings.Index(out, "defers=")
	if !(errIdx < warnIdx && warnIdx < infoIdx && infoIdx < deferIdx) {
		t.Errorf("wrong severity order in plain output: %q", out)
	}
}

func TestFormatStatusOutput_PlainSkipsZeros(t *testing.T) {
	cfg := config.Default()
	out := formatStatusOutput("plain", 1, 0, 0, 0, cfg)
	if strings.Contains(out, "errors=") || strings.Contains(out, "warns=") || strings.Contains(out, "defers=") {
		t.Errorf("plain output should skip zero counts: %q", out)
	}
	if !strings.Contains(out, "infos=1") {
		t.Errorf("plain output missing infos=1: %q", out)
	}
}

func TestFormatStatusOutput_JSON(t *testing.T) {
	cfg := config.Default()
	out := formatStatusOutput("json", 1, 2, 3, 4, cfg)
	// defers is intentionally omitted from JSON output; schema is stable
	want := `{"infos":1,"warns":2,"errors":3}`
	if out != want {
		t.Errorf("json output: want %q, got %q", want, out)
	}
}
