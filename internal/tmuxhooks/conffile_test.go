package tmuxhooks

import (
	"os"
	"strings"
	"testing"
)

func TestWithManagedBlockAppendsWhenAbsent(t *testing.T) {
	got := WithManagedBlock("set -g mouse on\n")
	if !strings.Contains(got, markerBegin) || !strings.Contains(got, markerEnd) {
		t.Fatalf("managed block markers missing:\n%s", got)
	}
	if !strings.Contains(got, "set -g mouse on") {
		t.Errorf("original content lost:\n%s", got)
	}
	if !strings.Contains(got, BootstrapHook) {
		t.Errorf("bootstrap line missing:\n%s", got)
	}
}

func TestWithManagedBlockReplacesInPlace(t *testing.T) {
	first := WithManagedBlock("# top\n")
	second := WithManagedBlock(first)
	if first != second {
		t.Errorf("WithManagedBlock not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	if strings.Count(second, markerBegin) != 1 {
		t.Errorf("want exactly one managed block, got %d", strings.Count(second, markerBegin))
	}
}

func TestSaveConfBacksUpAndWrites(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/.tmux.conf"
	if err := os.WriteFile(path, []byte("original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SaveConf(path, "updated\n"); err != nil {
		t.Fatalf("SaveConf: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "updated\n" {
		t.Errorf("file not updated: %q", got)
	}
	bak, err := os.ReadFile(path + ".demux-bak")
	if err != nil {
		t.Fatalf("backup not written: %v", err)
	}
	if string(bak) != "original\n" {
		t.Errorf("backup wrong: %q", bak)
	}
}

func TestLoadConfResolvesSymlink(t *testing.T) {
	dir := t.TempDir()
	real := dir + "/real.conf"
	link := dir + "/link.conf"
	if err := os.WriteFile(real, []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	content, realPath, existed, err := LoadConf(link)
	if err != nil || !existed {
		t.Fatalf("LoadConf: existed=%v err=%v", existed, err)
	}
	if content != "hi\n" {
		t.Errorf("content = %q", content)
	}
	if realPath != real {
		t.Errorf("realPath = %q, want %q", realPath, real)
	}
}
