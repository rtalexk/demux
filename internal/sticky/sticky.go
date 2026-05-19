// Package sticky implements the per-client tmux pane that runs `demux --sticky`
// and physically follows the attached client between tmux sessions.
//
// See docs/superpowers/specs/2026-05-18-sticky-sidebar-design.md for the spec.
package sticky

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Tmux abstracts the tmux invocations sticky needs. The production
// implementation shells out; tests substitute a fake.
type Tmux interface {
	Run(args ...string) error
	Output(args ...string) (string, error)
}

type realTmux struct{}

func (realTmux) Run(args ...string) error {
	return exec.Command("tmux", args...).Run()
}

func (realTmux) Output(args ...string) (string, error) {
	out, err := exec.Command("tmux", args...).Output()
	return string(out), err
}

// Sticky is the operations object. Construct via New() for production or
// inject a fake Tmux for tests.
type Sticky struct {
	T Tmux
}

// New returns a Sticky wired to the real tmux binary.
func New() *Sticky {
	return &Sticky{T: realTmux{}}
}

// envKeyPrefix is the shared prefix for per-client sticky pane env vars.
const envKeyPrefix = "DEMUX_STICKY_PANE_"

// SanitizeTTY converts a tmux client_tty string into a token safe for use in
// an environment variable name: lowercase, with any non-[a-z0-9] character
// replaced by '_'.
func SanitizeTTY(tty string) string {
	var b strings.Builder
	b.Grow(len(tty))
	for _, r := range strings.ToLower(tty) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

// EnvKey returns the tmux server-global env var name that stores the sticky
// sidebar pane ID for the given client TTY.
func EnvKey(tty string) string {
	return envKeyPrefix + SanitizeTTY(tty)
}

// ReadEnv reads a tmux server-global env var.
// Returns (value, true, nil) when set, ("", false, nil) when unset, error on
// any other tmux failure. Unset is detected by an "unknown variable" line in
// tmux's stderr (matches tmux 2.6+ output).
func (s *Sticky) ReadEnv(key string) (string, bool, error) {
	out, err := s.T.Output("show-environment", "-g", key)
	if err != nil {
		if isUnknownVariableErr(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("tmux show-environment %s: %w", key, err)
	}
	line := strings.TrimSpace(out)
	eq := strings.IndexByte(line, '=')
	if eq < 0 {
		if strings.HasPrefix(line, "-") {
			return "", false, nil
		}
		return "", false, fmt.Errorf("tmux show-environment: unexpected output %q", line)
	}
	return line[eq+1:], true, nil
}

// WriteEnv sets a tmux server-global env var.
func (s *Sticky) WriteEnv(key, value string) error {
	if err := s.T.Run("set-environment", "-g", key, value); err != nil {
		return fmt.Errorf("tmux set-environment -g %s: %w", key, err)
	}
	return nil
}

// UnsetEnv clears a tmux server-global env var.
func (s *Sticky) UnsetEnv(key string) error {
	if err := s.T.Run("set-environment", "-gu", key); err != nil {
		return fmt.Errorf("tmux set-environment -gu %s: %w", key, err)
	}
	return nil
}

// isUnknownVariableErr inspects an error from exec.Command.Output() to detect
// tmux's "unknown variable: X" stderr line.
func isUnknownVariableErr(err error) bool {
	if err == nil {
		return false
	}
	if strings.Contains(err.Error(), "unknown variable") {
		return true
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && strings.Contains(string(exitErr.Stderr), "unknown variable") {
		return true
	}
	return false
}
