package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/rtalexk/demux/internal/db"
	"github.com/rtalexk/demux/internal/tmuxhooks"
	"github.com/spf13/cobra"
)

var (
	hooksInstallTool      string
	hooksInstallPrintOnly bool
)

var hooksCmd = &cobra.Command{
	Use:   "hooks",
	Short: "Manage demux's tmux hooks",
}

// promptYesNo reads a line from r and returns true only for an affirmative
// answer (y/yes, case-insensitive). Anything else, including EOF, is false.
func promptYesNo(r io.Reader, w io.Writer, question string) bool {
	fmt.Fprintf(w, "%s [y/N] ", question)
	line, _ := bufio.NewReader(r).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}

// reviewLegacyHooks reports any legacy (pre-managed-block) demux hook lines in
// content, prompts the user to remove them, and returns content with the
// confirmed removals applied. realPath is used only in the report. content is
// returned unchanged when there are no legacy lines or the user declines.
func reviewLegacyHooks(in io.Reader, out io.Writer, content, realPath string) string {
	legacy := tmuxhooks.DetectLegacyHooks(content)
	if len(legacy) == 0 {
		return content
	}
	fmt.Fprintf(out, "Found %d legacy demux hook line(s) in %s:\n", len(legacy), realPath)
	for _, l := range legacy {
		fmt.Fprintf(out, "  L%d: %s\n", l.Number, strings.TrimSpace(l.Text))
	}
	if !promptYesNo(in, out, "Remove them and replace with the managed block?") {
		fmt.Fprintln(out, "Kept legacy lines; they will be de-duplicated at runtime but should be removed.")
		return content
	}
	nums := make([]int, len(legacy))
	for i, l := range legacy {
		nums[i] = l.Number
	}
	fmt.Fprintf(out, "Removed %d legacy line(s).\n", len(legacy))
	return tmuxhooks.RemoveLines(content, nums)
}

// registerLiveHooks registers the bootstrap hook and demux's full hook set into
// the running tmux server, then records the hook-set version. Failures are
// reported as warnings on stderr, not returned: the managed block is already
// written, so the next client attach reconciles even if no server is reachable.
func registerLiveHooks(cmd *cobra.Command) {
	out, errOut := cmd.OutOrStdout(), cmd.ErrOrStderr()
	t := tmuxhooks.Real()
	if err := t.Run("set-hook", "-g", tmuxhooks.BootstrapEvent, tmuxhooks.BootstrapCommand); err != nil {
		fmt.Fprintf(errOut, "warning: live bootstrap registration failed (not in tmux?): %v\n", err)
		return
	}
	if err := tmuxhooks.Reconcile(t); err != nil {
		fmt.Fprintf(errOut, "warning: live hook reconcile failed: %v\n", err)
		return
	}
	if err := tmuxhooks.MarkVersion(t); err != nil {
		fmt.Fprintf(errOut, "warning: recording hook version failed: %v\n", err)
	}
	fmt.Fprintln(out, "Registered demux hooks in the live tmux server.")
}

var hooksInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install demux's tmux hooks into ~/.tmux.conf and the live server",
	Long: `Installs demux's tmux integration.

Writes a one-line managed block to ~/.tmux.conf that bootstraps demux's
self-registering hooks, and registers them in the running tmux server
immediately. Re-running it is safe and idempotent.

With --print-only, prints the bootstrap line and makes no changes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if hooksInstallTool != "tmux" {
			return fmt.Errorf("unknown tool %q: supported tools: tmux", hooksInstallTool)
		}
		out := cmd.OutOrStdout()

		if hooksInstallPrintOnly {
			fmt.Fprintln(out, tmuxhooks.BootstrapHook)
			return nil
		}

		path, err := tmuxhooks.ConfPath()
		if err != nil {
			return err
		}
		content, realPath, existed, err := tmuxhooks.LoadConf(path)
		if err != nil {
			return err
		}

		content = reviewLegacyHooks(cmd.InOrStdin(), out, content, realPath)
		content = tmuxhooks.WithManagedBlock(content)
		if err := tmuxhooks.SaveConf(realPath, content); err != nil {
			return err
		}
		if existed {
			fmt.Fprintf(out, "Updated %s (backup: %s.demux-bak)\n", realPath, realPath)
		} else {
			fmt.Fprintf(out, "Created %s\n", realPath)
		}

		registerLiveHooks(cmd)
		return nil
	},
}

// hooksInitCmd is a hidden deprecated alias for `hooks install --print-only`.
var hooksInitCmd = &cobra.Command{
	Use:    "init",
	Short:  "Deprecated: use `demux hooks install --print-only`",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.OutOrStdout(), tmuxhooks.BootstrapHook)
		return nil
	},
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

func init() {
	hooksInstallCmd.Flags().StringVar(&hooksInstallTool, "tool", "tmux", "Tool to configure (supported: tmux)")
	hooksInstallCmd.Flags().BoolVar(&hooksInstallPrintOnly, "print-only", false, "Print the bootstrap line without changing any files")
	hooksInitCmd.Flags().String("tool", "tmux", "Deprecated and ignored; kept for backward compatibility")
	hooksCmd.AddCommand(hooksInstallCmd, hooksInitCmd)
	rootCmd.AddCommand(hooksCmd)
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
