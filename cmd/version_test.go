package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionDefault(t *testing.T) {
	if Version != "dev" {
		t.Errorf("default version: want %q, got %q", "dev", Version)
	}
}

func TestPrintVersion_OutputsVersion(t *testing.T) {
	orig := Version
	Version = "1.2.3"
	defer func() { Version = orig }()

	var buf bytes.Buffer
	printVersion(&buf)

	if got := strings.TrimSpace(buf.String()); got != "1.2.3" {
		t.Errorf("want %q, got %q", "1.2.3", got)
	}
}
