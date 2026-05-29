package tmuxhooks

import (
	"regexp"
	"strings"
	"testing"
)

func TestDesiredHooksExcludesClientAttached(t *testing.T) {
	hooks := DesiredHooks()
	if len(hooks) == 0 {
		t.Fatal("DesiredHooks returned empty set")
	}
	if len(hooks) != 7 {
		t.Errorf("DesiredHooks: want 7 entries, got %d", len(hooks))
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

// TestInteractiveHooksAreBackgrounded guards the fix for tmux freezing
// mid-redraw on interactive events. after-select-pane and client-focus-in fire
// at the start of a mouse scroll/drag; after-select-window and
// client-session-changed fire on window/session switch. A synchronous (non -b)
// run-shell on any of them blocks the tmux server's command queue while `demux
// event` touches the DB (which can stall on the SQLite lock), tearing the
// on-screen paint. All four must be backgrounded.
func TestInteractiveHooksAreBackgrounded(t *testing.T) {
	mustBg := map[string]bool{
		"after-select-pane":      true,
		"client-focus-in":        true,
		"after-select-window":    true,
		"client-session-changed": true,
	}
	seen := map[string]bool{}
	for _, h := range DesiredHooks() {
		if mustBg[h.Event] {
			seen[h.Event] = true
			if !strings.HasPrefix(h.Command, "run-shell -b ") {
				t.Errorf("hook %q must be backgrounded (run-shell -b) to avoid blocking the tmux server; got %q", h.Event, h.Command)
			}
		}
	}
	for ev := range mustBg {
		if !seen[ev] {
			t.Errorf("expected hook %q in DesiredHooks but it is missing", ev)
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
