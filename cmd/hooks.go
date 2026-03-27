package cmd

import (
    "encoding/json"
    "fmt"
    "io"
    "os"
    "os/exec"
    "sort"
    "strings"

    "github.com/spf13/cobra"
)

var hooksCmd = &cobra.Command{
    Use:   "hooks",
    Short: "Claude Code hook utilities",
}

var hooksInitCmd = &cobra.Command{
    Use:   "init",
    Short: "Print Claude Code hook configuration for demux",
    Long: `Prints a JSON snippet to add to ~/.claude/settings.json.

Stop fires when a Claude Code session ends — demux shows an info badge on
that tmux window.

Notification fires when Claude needs your attention: permission prompts,
idle waiting for input, elicitation dialogs. demux shows a warn badge with
the actual message from Claude so you know what it's asking.`,
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Print(claudeHooksSnippet)
        return nil
    },
}

// notifyEvent mirrors the JSON that Claude Code sends to the Notification hook on stdin.
type notifyEvent struct {
    Message          string `json:"message"`
    Title            string `json:"title"`
    NotificationType string `json:"notification_type"`
}

// stopEvent mirrors the JSON that Claude Code sends to the Stop hook on stdin.
type stopEvent struct {
    StopHookActive     bool   `json:"stop_hook_active"`
    LastAssistantMessage string `json:"last_assistant_message"`
}

type agentDef struct {
	stopMsg        string
	notifyFallback string
	snippet        string
}

var agentDefs = map[string]agentDef{
	"claude": {
		stopMsg:        "Claude finished",
		notifyFallback: "Claude notification",
		snippet:        claudeHooksSnippet,
	},
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
	return agentDef{}, fmt.Errorf("unknown agent %q: supported agents: %s", name, strings.Join(supported, ", "))
}

var hooksNotifyCmd = &cobra.Command{
    Use:   "notify",
    Short: "Handle a Claude Code Notification hook event (reads event JSON from stdin)",
    Long: `Reads the Notification hook event JSON from stdin, extracts the message,
and sets a warn-level demux alert on the current tmux window.

Intended to be called from ~/.claude/settings.json:
  "command": "demux hooks notify"`,
    RunE: func(cmd *cobra.Command, args []string) error {
        if os.Getenv("TMUX") == "" {
            return nil
        }

        var event notifyEvent
        if data, err := io.ReadAll(os.Stdin); err == nil {
            json.Unmarshal(data, &event) //nolint:errcheck — fallback handles failure
        }

        reason := event.Message
        if reason == "" {
            reason = "Claude notification"
        }

        target, err := tmuxTarget()
        if err != nil {
            return err
        }

        d, err := openDB()
        if err != nil {
            return err
        }
        defer d.Close()
        return d.AlertSet(target, reason, "warn", false)
    },
}

var hooksStopCmd = &cobra.Command{
    Use:   "stop",
    Short: "Handle a Claude Code Stop hook event (reads event JSON from stdin)",
    Long: `Reads the Stop hook event JSON from stdin and sets an info-level demux
alert on the current tmux window.

Intended to be called from ~/.claude/settings.json:
  "command": "demux hooks stop"`,
    RunE: func(cmd *cobra.Command, args []string) error {
        if os.Getenv("TMUX") == "" {
            return nil
        }

        var event stopEvent
        if data, err := io.ReadAll(os.Stdin); err == nil {
            json.Unmarshal(data, &event) //nolint:errcheck — fallback handles failure
        }

        // stop_hook_active=true means this stop was triggered by a stop hook;
        // skip to avoid creating stale alerts after the hook chain.
        if event.StopHookActive {
            return nil
        }

        target, err := tmuxTarget()
        if err != nil {
            return err
        }

        d, err := openDB()
        if err != nil {
            return err
        }
        defer d.Close()
        return d.AlertSet(target, "Claude finished", "info", false)
    },
}

// tmuxTarget returns the current tmux target as "session:windowIndex".
func tmuxTarget() (string, error) {
    out, err := exec.Command("tmux", "display-message", "-p", "#S:#I").Output()
    if err != nil {
        return "", fmt.Errorf("get tmux target: %w", err)
    }
    return strings.TrimSpace(string(out)), nil
}

const claudeHooksSnippet = `# Claude Code hooks for demux
# ──────────────────────────────────────────────────────────────────────────────
# Paste the JSON block below into ~/.claude/settings.json.
#
# If the file already has a "hooks" key, merge these entries into it.
# If the file does not exist yet, wrap the block in { } to make it valid JSON.
#
# How it works:
#   Stop         — fires when a Claude Code session ends.
#                  demux sets an info alert on the window (green badge).
#
#   Notification — fires when Claude needs your attention: permission prompts,
#                  idle waiting for input, elicitation dialogs.
#                  demux sets a warn alert with the actual message from Claude
#                  so you can see at a glance what it's asking (yellow badge).
#
# Both commands are silent when run outside tmux ($TMUX is unset).
# ──────────────────────────────────────────────────────────────────────────────

  "hooks": {
    "Stop": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "demux hooks stop"
          }
        ]
      }
    ],
    "Notification": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "demux hooks notify"
          }
        ]
      }
    ]
  }

# ──────────────────────────────────────────────────────────────────────────────
# Minimal ~/.claude/settings.json (starting from scratch):
#
#   {
#     "hooks": {
#       "Stop": [
#         { "matcher": "", "hooks": [{ "type": "command", "command": "demux hooks stop" }] }
#       ],
#       "Notification": [
#         { "matcher": "", "hooks": [{ "type": "command", "command": "demux hooks notify" }] }
#       ]
#     }
#   }
#
# To clear an alert after you've seen it:
#   demux alert remove --target SESSION:WINDOW
# ──────────────────────────────────────────────────────────────────────────────
`

func init() {
    hooksCmd.AddCommand(hooksInitCmd, hooksNotifyCmd, hooksStopCmd)
    rootCmd.AddCommand(hooksCmd)
}
