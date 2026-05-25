package cmd

import (
	"strings"
	"testing"

	"github.com/rtalexk/demux/internal/session"
	"github.com/rtalexk/demux/internal/sticky"
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

// Integration regression for the `demux session connect <config>` race: the
// after-new-window hook must be suppressed during the launch sequence
// (NewSessionDetached + CreateSessionWindows) and restored before the
// client-side Connect. Without this, backgrounded `demux event new_window`
// handlers race the eventual client-session-changed handler and strand the
// sidebar in the source session.
func TestLaunchConfigSessionAndConnect_SuppressesAfterNewWindow(t *testing.T) {
	tmux := &stubTmux{
		outputs: map[string]string{
			"show-hooks -g after-new-window": `after-new-window[0] run-shell -b "demux event new_window 2>/dev/null; true"` + "\n",
		},
	}

	type callOrder struct {
		name string
		idx  int
	}
	var order []callOrder
	track := func(name string) {
		order = append(order, callOrder{name: name, idx: len(tmux.runs)})
	}

	oldLaunch := launchConfigSession
	oldConnect := tmuxConnect
	oldClient := stickyClient
	launchConfigSession = func(name, path string, windowIDs []string, templates map[string]session.WindowTemplate) error {
		track("launch")
		return nil
	}
	tmuxConnect = func(name string) error {
		track("connect")
		return nil
	}
	stickyClient = func() *sticky.Sticky {
		return &sticky.Sticky{T: tmux}
	}
	t.Cleanup(func() {
		launchConfigSession = oldLaunch
		tmuxConnect = oldConnect
		stickyClient = oldClient
	})

	ce := &session.ConfigEntry{Name: "dotf-main", Path: "/tmp"}
	if err := launchConfigSessionAndConnect(ce, session.SessionsConfig{}); err != nil {
		t.Fatalf("launchConfigSessionAndConnect: %v", err)
	}

	if len(order) != 2 || order[0].name != "launch" || order[1].name != "connect" {
		t.Fatalf("want launch then connect, got: %+v", order)
	}

	idxUnset, idxRestore := -1, -1
	for i, r := range tmux.runs {
		j := strings.Join(r, " ")
		if j == "set-hook -gu after-new-window" {
			idxUnset = i
		}
		if strings.HasPrefix(j, "set-hook -ga after-new-window") {
			idxRestore = i
		}
	}
	if idxUnset < 0 {
		t.Fatalf("after-new-window must be suppressed before launch, got runs: %v", tmux.runs)
	}
	if idxRestore < 0 {
		t.Fatalf("after-new-window must be restored after launch (defer), got runs: %v", tmux.runs)
	}

	launchIdx := order[0].idx
	connectIdx := order[1].idx
	if !(idxUnset < launchIdx && launchIdx <= idxRestore && idxRestore <= connectIdx) {
		t.Errorf("want order: set-hook -gu (%d) < launch (%d) <= set-hook -ga (%d) <= connect (%d); runs: %v",
			idxUnset, launchIdx, idxRestore, connectIdx, tmux.runs)
	}
}
