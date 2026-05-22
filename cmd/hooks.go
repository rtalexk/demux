package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/rtalexk/demux/internal/db"
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

// tmuxCurrentIDs returns the stable pane, window, and session IDs for the
// tmux pane that launched this process.
func tmuxCurrentIDs() (paneID, windowID, sessionID string, err error) {
	paneID = os.Getenv("TMUX_PANE") // $TMUX_PANE is set by tmux as %N
	if paneID == "" {
		return "", "", "", fmt.Errorf("TMUX_PANE not set")
	}
	windowID, sessionID, err = tmuxParentIDs(paneID)
	return paneID, windowID, sessionID, err
}

// resolveParentIDs fills in parent window/session IDs on t by querying tmux.
// Best-effort: returns t unchanged when tmux is unavailable or target is a session.
func resolveParentIDs(t db.Target) db.Target {
	switch t.Type {
	case db.TargetTypePane:
		if windowID, sessionID, err := tmuxParentIDs(t.ID); err == nil {
			t.WindowID = windowID
			t.SessionID = sessionID
		}
	case db.TargetTypeWindow:
		if _, sessionID, err := tmuxParentIDs(t.ID); err == nil {
			t.SessionID = sessionID
		}
	}
	return t
}

// tmuxParentIDs queries tmux for the window_id and session_id of the given
// stable target ID (%N pane or @N window). It uses the provided ID directly
// so callers that know their target ID do not need to go through $TMUX_PANE.
func tmuxParentIDs(targetID string) (windowID, sessionID string, err error) {
	out, execErr := exec.Command("tmux", "display-message", "-t", targetID, "-p", "#{window_id}\t#{session_id}").Output()
	if execErr != nil {
		return "", "", fmt.Errorf("get tmux parent IDs for %s: %w", targetID, execErr)
	}
	parts := strings.Split(strings.TrimSpace(string(out)), "\t")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("unexpected tmux output for %s", targetID)
	}
	return parts[0], parts[1], nil
}

// tmuxResolveTarget resolves any tmux target string (including human-readable
// "session:window.pane" format) to stable pane, window, and session IDs.
// Used to handle pre-v8 hook snippets that passed --target instead of --pane-id.
func tmuxResolveTarget(target string) (paneID, windowID, sessionID string, err error) {
	out, execErr := exec.Command("tmux", "display-message", "-t", target, "-p", "#{pane_id}\t#{window_id}\t#{session_id}").Output()
	if execErr != nil {
		return "", "", "", fmt.Errorf("resolve tmux target %q: %w", target, execErr)
	}
	parts := strings.Split(strings.TrimSpace(string(out)), "\t")
	if len(parts) < 3 {
		return "", "", "", fmt.Errorf("unexpected tmux output for %q", target)
	}
	return parts[0], parts[1], parts[2], nil
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
#   after-kill-pane     — fires when a pane is closed; removes its state record.
#
# Flag syntax note: flags use --flag=value (not --flag value). tmux expands
# #{session_id} to the raw tmux ID (e.g. $8). The shell then expands $8 as a
# positional parameter, which is empty. With --flag value, an empty expansion
# leaves the flag with no argument and the command fails. With --flag=value, an
# empty expansion produces --flag= (empty string), which is valid.
#
# Idempotency note: the hooks below use -g (replace), not -ga (append), so
# re-sourcing this file (e.g. via a stray after-new-session re-source hook)
# replaces each hook in place instead of stacking duplicate copies. The two
# follow hooks on shared events (client-session-changed, after-select-window)
# keep -ga only because a -g line for the same event resets the array just
# above them.
# ──────────────────────────────────────────────────────────────────────────────

# Clears Demux done states when switching between panes within the same window.
set-hook -g after-select-pane   "run-shell 'demux event pane_focus --pane-id=#{pane_id} --window-id=#{window_id} --session-id=#{session_id} 2>/dev/null; true'"

# Clears Demux done states when switching windows (after-select-pane does not fire for window switches).
set-hook -g after-select-window "run-shell 'demux event pane_focus --pane-id=#{pane_id} --window-id=#{window_id} --session-id=#{session_id} 2>/dev/null; true'"

# Clears Demux done states when switching sessions (after-select-window does not fire for session switches).
set-hook -g client-session-changed "run-shell 'demux event pane_focus --pane-id=#{pane_id} --window-id=#{window_id} --session-id=#{session_id} 2>/dev/null; true'"

# Clears Demux done states when switching back from another application.
set-hook -g client-focus-in "run-shell 'demux event pane_focus --pane-id=#{pane_id} --window-id=#{window_id} --session-id=#{session_id} 2>/dev/null; true'"

# Removes Demux state when a pane is closed (prevents stale state accumulation).
# #{hook_pane} is the killed pane's ID; #{pane_id} is the new active pane after
# the kill - using #{pane_id} here would clear the wrong pane's state.
# #{hook_window} lets demux detect a window left with only a sidebar slot pane
# (e.g. the user closed the last non-slot pane) and clean it up.
# run-shell -b backgrounds the handler so closing many panes at once (e.g.
# 'demux sidebar hide' tearing down every sticky slot pane) never blocks tmux
# input; concurrent handlers serialize on a file lock instead.
set-hook -g after-kill-pane "run-shell -b 'demux event pane_closed --pane=#{hook_pane} --window=#{hook_window} 2>/dev/null; true'"

# Sticky sidebar - proactively ejects the sidebar when the process running in a
# pane exits and the sidebar would be the only pane left in that window. Fires
# BEFORE after-kill-pane (while the exiting pane is still present), which makes
# the sidebar return to the previous window without any stranded-sidebar flicker.
# Complements after-kill-pane for cases where that hook is unreliable (e.g.
# natural process exits in some tmux versions).
# run-shell -b backgrounds the handler (see after-kill-pane above) so it never
# blocks tmux input; it shares the pane_closed handler's file lock.
set-hook -g pane-exited "run-shell -b 'demux event pane_exiting --pane=#{hook_pane} --window=#{hook_window} 2>/dev/null; true'"

# Sticky sidebar - moves the demux sidebar pane to the newly active session.
set-hook -ga client-session-changed "run-shell 'demux sidebar follow 2>/dev/null; true'"

# Sticky sidebar - moves the demux sidebar pane to the newly active window
# (within the same session). join-pane uses -d so user focus is preserved.
set-hook -ga after-select-window "run-shell 'demux sidebar follow 2>/dev/null; true'"

# Sticky sidebar - moves the demux sidebar pane into a newly created window and
# ensures that window gets a slot pane (slots mode; no-op when slots = false or
# no slots are installed). tmux does NOT fire after-select-window for
# new-window, so this separate hook is required.
# run-shell -b backgrounds the handler so creating a window never blocks input
# while the sidebar relocates. follow + slots ensure run sequentially in one
# job so they never race each other.
set-hook -g after-new-window "run-shell -b 'demux sidebar follow 2>/dev/null; demux sidebar slots ensure 2>/dev/null; true'"

# Sticky sidebar auto-show on client attach. Remove this line if you prefer
# to toggle the sticky sidebar manually with 'demux sidebar toggle'.
set-hook -g client-attached "run-shell -b 'demux sidebar show 2>/dev/null; true'"
# ──────────────────────────────────────────────────────────────────────────────
`

func init() {
	hooksInitCmd.Flags().StringVar(&hooksInitTool, "tool", "", "Tool to configure (required): tmux")
	hooksInitCmd.MarkFlagRequired("tool")
	hooksCmd.AddCommand(hooksInitCmd)
	rootCmd.AddCommand(hooksCmd)
}
