package cmd

import (
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestSidebarCmd_Registered(t *testing.T) {
	if cmd, _, _ := rootCmd.Find([]string{"sidebar"}); cmd == nil || cmd.Use != "sidebar" {
		t.Fatal("expected `demux sidebar` command to be registered")
	}
}

func TestSidebarCmd_Subcommands(t *testing.T) {
	for _, sub := range []string{"toggle", "show", "hide", "follow"} {
		cmd, _, _ := rootCmd.Find([]string{"sidebar", sub})
		if cmd == nil || cmd.Name() != sub {
			t.Errorf("expected `demux sidebar %s`, got %v", sub, cmd)
		}
	}
}

func TestSidebarCmd_FollowIsHidden(t *testing.T) {
	cmd, _, _ := rootCmd.Find([]string{"sidebar", "follow"})
	if cmd == nil {
		t.Fatal("follow subcommand missing")
	}
	if !cmd.Hidden {
		t.Errorf("follow should be hidden from --help")
	}
}

func TestSidebarCmd_SlotsSubcommands(t *testing.T) {
	for _, sub := range []string{"install", "uninstall", "ensure"} {
		cmd, _, _ := rootCmd.Find([]string{"sidebar", "slots", sub})
		if cmd == nil || cmd.Name() != sub {
			t.Errorf("expected `demux sidebar slots %s`, got %v", sub, cmd)
		}
	}
}

func TestSidebarCmd_SlotsEnsureHidden(t *testing.T) {
	cmd, _, _ := rootCmd.Find([]string{"sidebar", "slots", "ensure"})
	if cmd == nil {
		t.Fatal("ensure subcommand missing")
	}
	if !cmd.Hidden {
		t.Errorf("slots ensure should be hidden from --help")
	}
}

func TestSidebarCmd_SlotIsHidden(t *testing.T) {
	cmd, _, _ := rootCmd.Find([]string{"sidebar", "slot"})
	if cmd == nil {
		t.Fatal("slot subcommand missing")
	}
	if !cmd.Hidden {
		t.Errorf("slot should be hidden from --help")
	}
}

// TestMain re-enters this test binary as the `demux sidebar slot` placeholder
// when DEMUX_TEST_SLOT is set, so the lifecycle tests below can drive a real
// process with real OS signal handlers.
func TestMain(m *testing.M) {
	if os.Getenv("DEMUX_TEST_SLOT") == "1" {
		runSidebarSlotPlaceholder(os.Stdout)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// startSlotHelper spawns this test binary as a slot placeholder and returns
// once the placeholder has installed its signal handlers, detected by its
// banner reaching stdout (the banner is printed after the handlers are set).
// The returned channel receives the process's exit.
func startSlotHelper(t *testing.T) (*exec.Cmd, <-chan error) {
	t.Helper()

	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	c := exec.Command(os.Args[0])
	c.Env = append(os.Environ(), "DEMUX_TEST_SLOT=1")
	c.Stdout = pw
	if err := c.Start(); err != nil {
		t.Fatalf("start slot helper: %v", err)
	}
	pw.Close() // child holds its own copy; drop ours so reads see EOF on exit

	exited := make(chan error, 1)
	go func() { exited <- c.Wait() }()

	ready := make(chan struct{})
	go func() {
		buf := make([]byte, 4096)
		var seen strings.Builder
		for {
			n, err := pr.Read(buf)
			seen.Write(buf[:n])
			if strings.Contains(seen.String(), "Reserved by") {
				close(ready)
				io.Copy(io.Discard, pr) // drain so the child never blocks on write
				return
			}
			if err != nil {
				return
			}
		}
	}()

	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		c.Process.Kill()
		t.Fatal("slot placeholder never painted its banner")
	}

	t.Cleanup(func() {
		c.Process.Kill()
		pr.Close()
	})
	return c, exited
}

func assertExitsOnSignal(t *testing.T, sig syscall.Signal) {
	t.Helper()
	c, exited := startSlotHelper(t)

	if err := c.Process.Signal(sig); err != nil {
		t.Fatalf("send %v: %v", sig, err)
	}
	select {
	case <-exited:
		// Process exited: its controlling pty is released back to the system.
	case <-time.After(5 * time.Second):
		t.Fatalf("slot placeholder ignored %v and did not exit; "+
			"a tmux pane close would orphan it and leak its pty", sig)
	}
}

// A slot placeholder must exit when tmux tears down its pane: tmux closes the
// pane's pty, which delivers SIGHUP to the placeholder.
func TestSidebarSlotPlaceholder_ExitsOnSIGHUP(t *testing.T) {
	assertExitsOnSignal(t, syscall.SIGHUP)
}

// An explicit `kill` / `tmux kill-pane` path delivers SIGTERM; the placeholder
// must honor it too so the pty is not leaked.
func TestSidebarSlotPlaceholder_ExitsOnSIGTERM(t *testing.T) {
	assertExitsOnSignal(t, syscall.SIGTERM)
}

// SIGINT is intentionally ignored: an accidental Ctrl-C while the slot pane is
// focused must not destroy the reserved slot.
func TestSidebarSlotPlaceholder_IgnoresSIGINT(t *testing.T) {
	c, exited := startSlotHelper(t)

	if err := c.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("send SIGINT: %v", err)
	}
	select {
	case <-exited:
		t.Fatal("slot placeholder exited on SIGINT; accidental Ctrl-C must not destroy the slot")
	case <-time.After(750 * time.Millisecond):
		// Still alive: correct.
	}
}
