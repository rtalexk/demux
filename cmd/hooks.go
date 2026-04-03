package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var hooksInitTool string

var hooksCmd = &cobra.Command{
	Use:   "hooks",
	Short: "AI agent hook utilities",
}

var hooksInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Print hook configuration for an AI agent",
	Long: `Prints a configuration snippet for the specified tool.

Supported tools: tmux

For --tool tmux, prints hook lines to add to ~/.tmux.conf.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		def, err := resolveAgent(hooksInitTool)
		if err != nil {
			return err
		}
		fmt.Print(def.snippet)
		return nil
	},
}

type agentDef struct {
	snippet string
}

var agentDefs = map[string]agentDef{
	"tmux": {snippet: tmuxHooksSnippet},
}

func resolveAgent(name string) (agentDef, error) {
	if def, ok := agentDefs[name]; ok {
		return def, nil
	}
	supported := make([]string, 0, len(agentDefs))
	for k := range agentDefs {
		supported = append(supported, k)
	}
	sort.Strings(supported)
	return agentDef{}, fmt.Errorf("unknown tool %q: supported tools: %s", name, strings.Join(supported, ", "))
}

// tmuxPaneTarget returns the current tmux target as "session:windowIndex.paneIndex".
// It uses $TMUX_PANE (set by tmux at process start) so the result always refers
// to the pane where the agent was launched, not the currently focused pane.
func tmuxPaneTarget() (string, error) {
	args := []string{"display-message", "-p", "#S:#I.#P"}
	if pane := os.Getenv("TMUX_PANE"); pane != "" {
		args = []string{"display-message", "-t", pane, "-p", "#S:#I.#P"}
	}
	out, err := exec.Command("tmux", args...).Output()
	if err != nil {
		return "", fmt.Errorf("get tmux pane target: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

const tmuxHooksSnippet = `# demux tmux hooks
# ──────────────────────────────────────────────────────────────────────────────
# Paste the lines below into ~/.tmux.conf, then reload with:
#   tmux source ~/.tmux.conf
#
# How it works:
#   after-select-pane   — fires when you move focus between panes within a window.
#   after-select-window — fires when you switch to a different window.
#   client-session-changed — fires when you switch to a different session.
#   All three are needed: each navigation action fires a different hook.
#   Together they cover every way a pane can receive focus.
#   Each hook targets the now-active pane and clears any alerts on it and its
#   parent window.
# ──────────────────────────────────────────────────────────────────────────────

# Clears Demux alerts when switching between panes within the same window.
set-hook -g after-select-pane   "run-shell 'demux event pane_focus --target #{session_name}:#{window_index}.#{pane_index} 2>/dev/null; true'"

# Clears Demux alerts when switching windows (after-select-pane does not fire for window switches).
set-hook -g after-select-window "run-shell 'demux event pane_focus --target #{session_name}:#{window_index}.#{pane_index} 2>/dev/null; true'"

# Clears Demux alerts when switching sessions (after-select-window does not fire for session switches).
set-hook -g client-session-changed "run-shell 'demux event pane_focus --target #{session_name}:#{window_index}.#{pane_index} 2>/dev/null; true'"

# Clears Demux alerts when switching back from another application.
set-hook -g client-focus-in "run-shell 'demux event pane_focus --target #{session_name}:#{window_index}.#{pane_index} 2>/dev/null; true'"
# ──────────────────────────────────────────────────────────────────────────────
`

func init() {
	hooksInitCmd.Flags().StringVar(&hooksInitTool, "tool", "", "Tool to configure (required): tmux")
	hooksInitCmd.MarkFlagRequired("tool")
	hooksCmd.AddCommand(hooksInitCmd)
	rootCmd.AddCommand(hooksCmd)
}
