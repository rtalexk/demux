package tmuxhooks

import (
	"fmt"
	"os/exec"
	"strings"
)

// Tmux abstracts the tmux invocations the reconciler needs. The production
// implementation shells out; tests substitute a fake.
type Tmux interface {
	Run(args ...string) error
	Output(args ...string) (string, error)
}

type realTmux struct{}

// Real returns a Tmux that shells out to the tmux binary.
func Real() Tmux { return realTmux{} }

func (realTmux) Run(args ...string) error {
	cmd := exec.Command("tmux", args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
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
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return string(out), fmt.Errorf("%w: %s", err, msg)
		}
	}
	return string(out), err
}
