package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

// A tmux hook that calls a since-removed demux subcommand must fail silently:
// an error on stderr with a non-zero exit, so the hook's `2>/dev/null` swallows
// it. Cobra's default for a non-runnable parent command is to print the help
// text to stdout with a zero exit, which leaks help into the tmux pane.
func TestGroupCommandsRejectUnknownSubcommand(t *testing.T) {
	groups := map[string]*cobra.Command{
		"event":         eventCmd,
		"sidebar":       sidebarCmd,
		"sidebar slots": sidebarSlotsCmd,
	}
	for name, c := range groups {
		if c.RunE == nil {
			t.Errorf("%s: group command must have a RunE that rejects unknown subcommands", name)
			continue
		}
		if err := c.RunE(c, []string{"definitely-not-a-real-subcommand"}); err == nil {
			t.Errorf("%s: unknown subcommand must return an error, got nil", name)
		}
	}
}
