// Package sticky implements the per-client tmux pane that runs `demux --sticky`
// and physically follows the attached client between tmux sessions.
//
// See docs/superpowers/specs/2026-05-18-sticky-sidebar-design.md for the spec.
package sticky

import (
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

// keep imports used
var _ = fmt.Sprintf
