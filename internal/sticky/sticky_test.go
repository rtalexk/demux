package sticky

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// fakeTmux records calls and returns scripted outputs/errors.
type fakeTmux struct {
	outputs map[string]fakeReply
	runs    [][]string
	runErrs map[string]error
}

type fakeReply struct {
	out string
	err error
}

func newFakeTmux() *fakeTmux {
	return &fakeTmux{
		outputs: map[string]fakeReply{},
		runErrs: map[string]error{},
	}
}

func (f *fakeTmux) key(args []string) string { return strings.Join(args, " ") }

func (f *fakeTmux) Run(args ...string) error {
	f.runs = append(f.runs, append([]string(nil), args...))
	if err, ok := f.runErrs[f.key(args)]; ok {
		return err
	}
	return nil
}

func (f *fakeTmux) Output(args ...string) (string, error) {
	r, ok := f.outputs[f.key(args)]
	if !ok {
		return "", fmt.Errorf("fakeTmux: no scripted output for: %s", f.key(args))
	}
	return r.out, r.err
}

// stderrExitError mimics what exec.Command.Output() returns when tmux fails:
// an *exec.ExitError whose Stderr contains the message.
func stderrExitError(msg string) error {
	return &exec.ExitError{Stderr: []byte(msg)}
}

func TestSanitizeTTY(t *testing.T) {
	cases := map[string]string{
		"/dev/ttys001":  "_dev_ttys001",
		"/dev/pts/3":    "_dev_pts_3",
		"":              "",
		"ABC123":        "abc123",
		"/dev/ttyS01-x": "_dev_ttys01_x",
	}
	for in, want := range cases {
		got := SanitizeTTY(in)
		if got != want {
			t.Errorf("SanitizeTTY(%q): want %q, got %q", in, want, got)
		}
	}
}

func TestEnvKey(t *testing.T) {
	got := EnvKey("/dev/ttys001")
	want := "DEMUX_STICKY_PANE__dev_ttys001"
	if got != want {
		t.Errorf("EnvKey: want %q, got %q", want, got)
	}
}

func TestReadEnv_Set(t *testing.T) {
	f := newFakeTmux()
	f.outputs["show-environment -g DEMUX_STICKY_PANE__dev_ttys001"] = fakeReply{
		out: "DEMUX_STICKY_PANE__dev_ttys001=%42\n",
	}
	s := &Sticky{T: f}
	val, set, err := s.ReadEnv("DEMUX_STICKY_PANE__dev_ttys001")
	if err != nil {
		t.Fatalf("ReadEnv: %v", err)
	}
	if !set || val != "%42" {
		t.Errorf("want (%q, true), got (%q, %v)", "%42", val, set)
	}
}

func TestReadEnv_UnsetViaStderr(t *testing.T) {
	f := newFakeTmux()
	f.outputs["show-environment -g DEMUX_STICKY_PANE_x"] = fakeReply{
		err: stderrExitError("unknown variable: DEMUX_STICKY_PANE_x\n"),
	}
	s := &Sticky{T: f}
	val, set, err := s.ReadEnv("DEMUX_STICKY_PANE_x")
	if err != nil {
		t.Fatalf("ReadEnv: %v", err)
	}
	if set || val != "" {
		t.Errorf("want unset, got (%q, %v)", val, set)
	}
}

func TestReadEnv_OtherErrorSurfaces(t *testing.T) {
	f := newFakeTmux()
	f.outputs["show-environment -g DEMUX_STICKY_PANE_y"] = fakeReply{
		err: errors.New("tmux: server not running"),
	}
	s := &Sticky{T: f}
	if _, _, err := s.ReadEnv("DEMUX_STICKY_PANE_y"); err == nil {
		t.Error("expected error to surface, got nil")
	}
}

func TestWriteEnv(t *testing.T) {
	f := newFakeTmux()
	s := &Sticky{T: f}
	if err := s.WriteEnv("DEMUX_STICKY_PANE_x", "%5"); err != nil {
		t.Fatalf("WriteEnv: %v", err)
	}
	want := []string{"set-environment", "-g", "DEMUX_STICKY_PANE_x", "%5"}
	if len(f.runs) != 1 || strings.Join(f.runs[0], " ") != strings.Join(want, " ") {
		t.Errorf("WriteEnv args: want %v, got %v", want, f.runs)
	}
}

func TestPaneAlive_Found(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -aF #{pane_id}"] = fakeReply{out: "%1\n%42\n%3\n"}
	s := &Sticky{T: f}
	alive, err := s.PaneAlive("%42")
	if err != nil || !alive {
		t.Errorf("want alive=true, got alive=%v err=%v", alive, err)
	}
}

func TestPaneAlive_NotFound(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -aF #{pane_id}"] = fakeReply{out: "%1\n%3\n"}
	s := &Sticky{T: f}
	alive, err := s.PaneAlive("%42")
	if err != nil || alive {
		t.Errorf("want alive=false, got alive=%v err=%v", alive, err)
	}
}

func TestPaneAlive_TmuxFails(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -aF #{pane_id}"] = fakeReply{err: errors.New("server not running")}
	s := &Sticky{T: f}
	if _, err := s.PaneAlive("%42"); err == nil {
		t.Error("expected error to surface")
	}
}

func TestCurrentClientTTY(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{out: "/dev/ttys001\n"}
	s := &Sticky{T: f}
	tty, err := s.CurrentClientTTY()
	if err != nil {
		t.Fatalf("CurrentClientTTY: %v", err)
	}
	if tty != "/dev/ttys001" {
		t.Errorf("tty: want /dev/ttys001, got %q", tty)
	}
}

func TestCurrentClientTTY_NotInTmux(t *testing.T) {
	f := newFakeTmux()
	f.outputs["display-message -p #{client_tty}"] = fakeReply{err: stderrExitError("no server running")}
	s := &Sticky{T: f}
	if _, err := s.CurrentClientTTY(); err == nil {
		t.Error("expected error when tmux unavailable")
	}
}

func TestCheckTmuxVersion(t *testing.T) {
	cases := map[string]bool{
		"tmux 3.3a\n":     true,
		"tmux 3.4\n":      true,
		"tmux 2.6\n":      true,
		"tmux 2.5\n":      false,
		"tmux 1.8\n":      false,
		"tmux next-3.5\n": true,
		"":                true,
	}
	for in, expectOK := range cases {
		err := checkTmuxVersion(in)
		if (err == nil) != expectOK {
			t.Errorf("checkTmuxVersion(%q): expectOK=%v, got err=%v", in, expectOK, err)
		}
	}
}

func TestVersionOK_UsesTmuxV(t *testing.T) {
	f := newFakeTmux()
	f.outputs["-V"] = fakeReply{out: "tmux 3.3a\n"}
	s := &Sticky{T: f}
	if err := s.VersionOK(); err != nil {
		t.Errorf("VersionOK: %v", err)
	}
}

func TestUnsetEnv(t *testing.T) {
	f := newFakeTmux()
	s := &Sticky{T: f}
	if err := s.UnsetEnv("DEMUX_STICKY_PANE_x"); err != nil {
		t.Fatalf("UnsetEnv: %v", err)
	}
	want := []string{"set-environment", "-gu", "DEMUX_STICKY_PANE_x"}
	if len(f.runs) != 1 || strings.Join(f.runs[0], " ") != strings.Join(want, " ") {
		t.Errorf("UnsetEnv args: want %v, got %v", want, f.runs)
	}
}
