package format_test

import (
	"strings"
	"testing"

	"github.com/rtalexk/demux/internal/format"
)

type row struct{ A, B string }

func (r row) Fields() []string { return []string{r.A, r.B} }

func TestTextFormat(t *testing.T) {
	rows := []format.Row{row{"hello", "world"}, row{"foo", "bar"}}
	out := format.Text([]string{"COL1", "COL2"}, rows)
	if !strings.Contains(out, "hello") || !strings.Contains(out, "world") {
		t.Errorf("unexpected output: %s", out)
	}
	if !strings.Contains(out, "COL1") {
		t.Errorf("header missing in text output: %s", out)
	}
}

func TestTableFormat(t *testing.T) {
	rows := []format.Row{row{"hello", "world"}}
	out := format.Table([]string{"COL1", "COL2"}, rows, false) // false = no TTY bold
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines (header + row), got %d: %s", len(lines), out)
	}
	if !strings.Contains(lines[0], "COL1") {
		t.Errorf("header missing COL1: %s", lines[0])
	}
	if !strings.Contains(lines[1], "hello") {
		t.Errorf("row missing hello: %s", lines[1])
	}
}

func TestTableBoldTTY(t *testing.T) {
	rows := []format.Row{row{"v", "w"}}
	out := format.Table([]string{"A", "B"}, rows, true) // TTY mode
	if !strings.Contains(out, "\033[1m") {
		t.Errorf("expected bold ANSI in TTY mode: %s", out)
	}
}

func TestJSONFormat(t *testing.T) {
	rows := []format.Row{row{"hello", "world"}}
	out := format.JSON([]string{"col1", "col2"}, rows)
	if !strings.Contains(out, `"col1"`) || !strings.Contains(out, `"hello"`) {
		t.Errorf("unexpected json: %s", out)
	}
	// should be newline-delimited (one JSON object per line)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 line for 1 row, got %d", len(lines))
	}
}

func TestRenderDispatcher(t *testing.T) {
	rows := []format.Row{row{"a", "b"}}
	headers := []string{"X", "Y"}

	if out := format.Render("text", headers, rows, false); !strings.Contains(out, "a") {
		t.Errorf("text render failed: %s", out)
	}
	if out := format.Render("table", headers, rows, false); !strings.Contains(out, "X") {
		t.Errorf("table render failed: %s", out)
	}
	if out := format.Render("json", headers, rows, false); !strings.Contains(out, `"X"`) {
		t.Errorf("json render failed: %s", out)
	}
	// unknown format falls back to text
	if out := format.Render("unknown", headers, rows, false); !strings.Contains(out, "a") {
		t.Errorf("unknown format fallback failed: %s", out)
	}
}
