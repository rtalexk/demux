// Package sticky implements the per-client tmux pane that runs `demux --sticky`
// and physically follows the attached client between tmux sessions.
//
// See docs/superpowers/specs/2026-05-18-sticky-sidebar-design.md for the spec.
package sticky

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
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
	cmd := exec.Command("tmux", args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}

func (realTmux) Output(args ...string) (string, error) {
	cmd := exec.Command("tmux", args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return string(out), fmt.Errorf("%w: %s", err, msg)
		}
	}
	return string(out), err
}

// Sticky is the operations object. Construct via New() for production or
// inject a fake Tmux for tests.
type Sticky struct {
	T Tmux
	// Slots toggles per-window slot mode: when true, Show installs a slot
	// pane in every window and switches between them via swap-pane (instead
	// of join-pane). The active sidebar pane lives in whichever window's
	// slot is currently selected; the others run SlotPlaceholderCmd.
	Slots bool
	// FocusOnOpen, when true, moves focus into the sidebar pane right after
	// Show creates (split mode) or activates (slots mode) it.
	FocusOnOpen bool
	// FocusBeforeToggleClose, when true, makes Toggle focus an already-open
	// but unfocused sidebar instead of hiding it; a second Toggle from
	// inside the sidebar then hides it.
	FocusBeforeToggleClose bool
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

// stickyPaneEnv is one DEMUX_STICKY_PANE_* binding parsed from
// `tmux show-environment -g`: the env var name and the pane id it points at.
type stickyPaneEnv struct {
	key  string
	pane string
}

// listStickyPaneEnv runs `tmux show-environment -g` and returns every
// DEMUX_STICKY_PANE_* binding found. Lines that lack a '=' (e.g. tmux's "-KEY"
// unset marker) are skipped.
func listStickyPaneEnv(t Tmux) ([]stickyPaneEnv, error) {
	out, err := t.Output("show-environment", "-g")
	if err != nil {
		return nil, err
	}
	var envs []stickyPaneEnv
	for _, line := range nonEmptyLines(out) {
		if !strings.HasPrefix(line, envKeyPrefix) {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		envs = append(envs, stickyPaneEnv{key: line[:eq], pane: line[eq+1:]})
	}
	return envs, nil
}

// VersionOK returns nil when the available tmux binary is >= 2.6, which is
// required for the -f flag on split-window and join-pane.
func (s *Sticky) VersionOK() error {
	out, err := s.T.Output("-V")
	if err != nil {
		return fmt.Errorf("tmux -V: %w", err)
	}
	return checkTmuxVersion(out)
}

// checkTmuxVersion parses output like "tmux 3.3a\n" and returns an error when
// the version is older than 2.6. Unparseable inputs (next-* builds, empty
// output) succeed.
func checkTmuxVersion(out string) error {
	s := strings.TrimSpace(out)
	s = strings.TrimPrefix(s, "tmux ")
	if s == "" || strings.HasPrefix(s, "next-") {
		return nil
	}
	end := len(s)
	for end > 0 {
		r := s[end-1]
		if (r >= '0' && r <= '9') || r == '.' {
			break
		}
		end--
	}
	s = s[:end]
	parts := strings.SplitN(s, ".", 3)
	if len(parts) < 2 {
		return nil
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return nil
	}
	if major > 2 || (major == 2 && minor >= 6) {
		return nil
	}
	return fmt.Errorf("tmux %d.%d is too old; sticky sidebar requires tmux >= 2.6", major, minor)
}

// PaneAlive returns true when paneID appears in `tmux list-panes -a`.
func (s *Sticky) PaneAlive(paneID string) (bool, error) {
	out, err := s.T.Output("list-panes", "-aF", "#{pane_id}")
	if err != nil {
		return false, fmt.Errorf("tmux list-panes: %w", err)
	}
	for _, line := range nonEmptyLines(out) {
		if line == paneID {
			return true, nil
		}
	}
	return false, nil
}

// CurrentClientTTY returns the tmux #{client_tty} for the invoking client.
func (s *Sticky) CurrentClientTTY() (string, error) {
	out, err := s.T.Output("display-message", "-p", "#{client_tty}")
	if err != nil {
		return "", fmt.Errorf("tmux display-message #{client_tty}: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// currentTarget returns the window the invoking tmux client is focused on.
// target is the raw "session_id:window_id" string (usable directly as a tmux
// -t argument); session and window are its two halves.
func (s *Sticky) currentTarget() (target, session, window string, err error) {
	out, err := s.T.Output("display-message", "-p", "#{session_id}:#{window_id}")
	if err != nil {
		return "", "", "", fmt.Errorf("tmux display-message current target: %w", err)
	}
	target = strings.TrimSpace(out)
	parts := strings.SplitN(target, ":", 2)
	if len(parts) < 2 {
		return "", "", "", fmt.Errorf("unexpected current target %q", target)
	}
	return target, parts[0], parts[1], nil
}

// nonEmptyLines splits raw tmux output on newlines and returns each line
// trimmed of surrounding whitespace, dropping any that are empty.
func nonEmptyLines(out string) []string {
	var lines []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
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
