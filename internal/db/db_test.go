package db_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rtalexk/demux/internal/db"
)

func TestOpen_DirPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "state.db")
	d, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	info, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	// 0700 has no bits in common with any standard umask, so the
	// kernel will not mask any bits away.
	if got := info.Mode().Perm(); got != 0700 {
		t.Errorf("dir perm = %04o, want 0700", got)
	}
}

func TestDefaultPath_ContainsExpectedSuffix(t *testing.T) {
	path, err := db.DefaultPath()
	if err != nil {
		t.Skipf("UserHomeDir not available: %v", err)
	}
	const suffix = ".local/share/demux/state.db"
	if !strings.HasSuffix(path, suffix) {
		t.Errorf("DefaultPath() = %q, want suffix %q", path, suffix)
	}
}
