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

func TestDetectLegacyHooks(t *testing.T) {
	content := strings.Join([]string{
		"set -g mouse on", // 1: not a hook
		`set-hook -g after-select-pane "run-shell 'demux event pane_focus'"`, // 2: legacy
		"# set-hook -g foo 'demux event bar'",                                // 3: comment, ignored
		`set-hook -ga after-new-window "run-shell 'demux sidebar follow'"`,   // 4: legacy
		`set-hook -g client-resized 'run-shell "other tool"'`,                // 5: not demux
	}, "\n")
	got := DetectLegacyHooks(content)
	if len(got) != 2 {
		t.Fatalf("want 2 legacy lines, got %d: %+v", len(got), got)
	}
	if got[0].Number != 2 || got[1].Number != 4 {
		t.Errorf("wrong line numbers: %+v", got)
	}
}

func TestDetectLegacyHooksIgnoresManagedBlock(t *testing.T) {
	content := WithManagedBlock("set -g mouse on\n")
	if got := DetectLegacyHooks(content); len(got) != 0 {
		t.Errorf("bootstrap line inside managed block must not be flagged: %+v", got)
	}
}

func TestRemoveLines(t *testing.T) {
	content := "a\nb\nc\nd\n"
	got := RemoveLines(content, []int{2, 4})
	if got != "a\nc\n" {
		t.Errorf("RemoveLines = %q, want %q", got, "a\nc\n")
	}
}
