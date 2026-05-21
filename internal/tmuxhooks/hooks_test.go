package tmuxhooks

import (
	"regexp"
	"testing"
)

func TestDesiredHooksExcludesClientAttached(t *testing.T) {
	hooks := DesiredHooks()
	if len(hooks) == 0 {
		t.Fatal("DesiredHooks returned empty set")
	}
	if len(hooks) != 9 {
		t.Errorf("DesiredHooks: want 9 entries, got %d", len(hooks))
	}
	for _, h := range hooks {
		if h.Event == "client-attached" {
			t.Errorf("client-attached must not be in DesiredHooks (it is the bootstrap)")
		}
		if h.Event == "" || h.Command == "" {
			t.Errorf("hook has empty field: %+v", h)
		}
	}
}

func TestDesiredHooksCoversExpectedEvents(t *testing.T) {
	want := []string{
		"after-select-pane", "after-select-window", "client-session-changed",
		"client-focus-in", "after-kill-pane", "pane-exited", "after-new-window",
	}
	got := map[string]bool{}
	for _, h := range DesiredHooks() {
		got[h.Event] = true
	}
	for _, ev := range want {
		if !got[ev] {
			t.Errorf("DesiredHooks missing event %q", ev)
		}
	}
}

func TestHooksHashIsStableHex12(t *testing.T) {
	a, b := HooksHash(), HooksHash()
	if a != b {
		t.Errorf("HooksHash not deterministic: %q != %q", a, b)
	}
	if !regexp.MustCompile(`^[0-9a-f]{12}$`).MatchString(a) {
		t.Errorf("HooksHash %q is not 12 lowercase hex chars", a)
	}
}
