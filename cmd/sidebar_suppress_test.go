package cmd

import (
	"errors"
	"strings"
	"testing"
)

// stubTmux is a minimal sticky.Tmux for testing hook suppression.
type stubTmux struct {
	outputs map[string]string
	outErrs map[string]error
	runs    [][]string
}

func (s *stubTmux) key(args []string) string { return strings.Join(args, " ") }

func (s *stubTmux) Run(args ...string) error {
	s.runs = append(s.runs, append([]string(nil), args...))
	return nil
}

func (s *stubTmux) Output(args ...string) (string, error) {
	k := s.key(args)
	if err, ok := s.outErrs[k]; ok {
		return "", err
	}
	if v, ok := s.outputs[k]; ok {
		return v, nil
	}
	return "", errors.New("no scripted output for: " + k)
}

func TestWithHooksSuppressed_ClearsAndRestores(t *testing.T) {
	tmux := &stubTmux{
		outputs: map[string]string{
			"show-hooks -g after-kill-pane": `after-kill-pane[0] run-shell -b "demux event pane_closed --pane=#{hook_pane} 2>/dev/null; true"
`,
			"show-hooks -g pane-exited": `pane-exited[0] run-shell -b "demux event pane_exiting --pane=#{hook_pane} 2>/dev/null; true"
pane-exited[1] run-shell "logger second hook"
`,
		},
	}

	called := false
	if err := withHooksSuppressed(tmux, []string{"after-kill-pane", "pane-exited"}, func() error {
		called = true
		return nil
	}); err != nil {
		t.Fatalf("withHooksSuppressed: %v", err)
	}
	if !called {
		t.Fatal("fn was not called")
	}

	wantOrder := []string{
		"set-hook -gu after-kill-pane",
		"set-hook -gu pane-exited",
		`set-hook -ga after-kill-pane run-shell -b "demux event pane_closed --pane=#{hook_pane} 2>/dev/null; true"`,
		`set-hook -ga pane-exited run-shell -b "demux event pane_exiting --pane=#{hook_pane} 2>/dev/null; true"`,
		`set-hook -ga pane-exited run-shell "logger second hook"`,
	}
	got := make([]string, len(tmux.runs))
	for i, r := range tmux.runs {
		got[i] = strings.Join(r, " ")
	}
	for _, want := range wantOrder {
		found := false
		for _, g := range got {
			if g == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing tmux call %q\nactual: %v", want, got)
		}
	}
	// Critical ordering: every clear must precede its matching restore.
	idxClearKill := indexOf(got, "set-hook -gu after-kill-pane")
	idxRestoreKill := indexOf(got, `set-hook -ga after-kill-pane run-shell -b "demux event pane_closed --pane=#{hook_pane} 2>/dev/null; true"`)
	if idxClearKill < 0 || idxRestoreKill < 0 || idxClearKill > idxRestoreKill {
		t.Errorf("after-kill-pane: clear must precede restore; clear=%d restore=%d", idxClearKill, idxRestoreKill)
	}
}

func TestWithHooksSuppressed_RestoresOnFnError(t *testing.T) {
	tmux := &stubTmux{
		outputs: map[string]string{
			"show-hooks -g after-kill-pane": `after-kill-pane[0] run-shell -b "demux event pane_closed ..."` + "\n",
		},
	}
	sentinel := errors.New("boom")
	err := withHooksSuppressed(tmux, []string{"after-kill-pane"}, func() error { return sentinel })
	if err != sentinel {
		t.Fatalf("want sentinel error, got %v", err)
	}
	// Restore must still run.
	sawRestore := false
	for _, r := range tmux.runs {
		if strings.HasPrefix(strings.Join(r, " "), "set-hook -ga after-kill-pane ") {
			sawRestore = true
		}
	}
	if !sawRestore {
		t.Errorf("expected hook restore even when fn returned error, got runs: %v", tmux.runs)
	}
}

func TestWithHooksSuppressed_NoHooksRegistered_StillCallsFn(t *testing.T) {
	tmux := &stubTmux{
		outputs: map[string]string{
			"show-hooks -g after-kill-pane": "",
		},
	}
	called := false
	if err := withHooksSuppressed(tmux, []string{"after-kill-pane"}, func() error {
		called = true
		return nil
	}); err != nil {
		t.Fatalf("withHooksSuppressed: %v", err)
	}
	if !called {
		t.Errorf("fn must run even when no hooks were registered")
	}
}

func TestParseSuppressedHookCmds_HandlesMultipleEntries(t *testing.T) {
	in := `after-kill-pane[0] run-shell -b "demux event pane_closed"
after-kill-pane[1] run-shell "logger a"
pane-exited[0] run-shell "logger b"
`
	got := parseSuppressedHookCmds(in, "after-kill-pane")
	want := []string{`run-shell -b "demux event pane_closed"`, `run-shell "logger a"`}
	if len(got) != len(want) {
		t.Fatalf("got %d cmds, want %d: %v", len(got), len(want), got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("[%d] got %q want %q", i, got[i], want[i])
		}
	}
}

func indexOf(haystack []string, needle string) int {
	for i, h := range haystack {
		if h == needle {
			return i
		}
	}
	return -1
}
