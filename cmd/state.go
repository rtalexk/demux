package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/rtalexk/demux/internal/db"
	"github.com/rtalexk/demux/internal/format"
	demuxlog "github.com/rtalexk/demux/internal/log"
	"github.com/rtalexk/demux/internal/tmux"
	"github.com/spf13/cobra"
)

var (
	stateTarget  string
	stateValue   string
	stateTool    string
	stateMessage string
	stateSource  string
	stateForce   bool
	stateIfState string

	clearTarget string
	clearYes    bool
)

var stateListCmd *cobra.Command

func init() {
	stateCmd := &cobra.Command{
		Use:   "state",
		Short: "Manage tool states",
		RunE: func(cmd *cobra.Command, args []string) error {
			return stateListCmd.RunE(cmd, args)
		},
	}

	// state set
	stateSetCmd := &cobra.Command{
		Use:   "set",
		Short: "Set the state of a target",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := openDB()
			if err != nil {
				return err
			}
			defer d.Close()
			return applyStateSet(d)
		},
	}
	stateSetCmd.Flags().StringVar(&stateTarget, "target-id", "", "Target ID: pane (%%N), window (@N), or session ($N) (required)")
	stateSetCmd.Flags().StringVar(&stateValue, "state", "", "State: working|waiting|done|error|flagged (required)")
	stateSetCmd.Flags().StringVar(&stateTool, "tool", "", "Tool name")
	stateSetCmd.Flags().StringVar(&stateMessage, "message", "", "Human-readable detail")
	stateSetCmd.Flags().StringVar(&stateSource, "source", "tool", "Source: tool|user")
	stateSetCmd.Flags().BoolVar(&stateForce, "force", false, "Override write-lock (allow overwriting another tool's active state)")
	stateSetCmd.Flags().StringVar(&stateIfState, "if-state", "", "Only write if current state matches this value")
	stateSetCmd.MarkFlagRequired("target-id")
	stateSetCmd.MarkFlagRequired("state")

	// state clear
	stateClearCmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear the state of a target (return to idle)",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := openDB()
			if err != nil {
				return err
			}
			defer d.Close()
			return applyStateClear(d)
		},
	}
	stateClearCmd.Flags().StringVar(&clearTarget, "target-id", "", "Target ID (required)")
	stateClearCmd.Flags().BoolVar(&clearYes, "yes", false, "Confirm clearing a flagged target")
	stateClearCmd.MarkFlagRequired("target-id")

	// state ls
	stateListCmd = &cobra.Command{
		Use:   "ls",
		Short: "List all active (non-idle) states",
		RunE:  runStateList,
	}
	stateListCmd.Flags().String("state", "", "Filter by state")
	stateListCmd.Flags().String("tool", "", "Filter by tool")
	stateListCmd.Flags().String("format", "table", "Output format: text|table|json")

	stateCmd.AddCommand(stateSetCmd, stateClearCmd, stateListCmd)
	rootCmd.AddCommand(stateCmd)
}

func applyStateSet(d *db.DB) error {
	// Parse the target ID and infer type.
	target, err := db.ParseTargetID(stateTarget)
	if err != nil {
		return fmt.Errorf("invalid --target-id: %w", err)
	}

	// For window and pane targets, resolve the missing session/window IDs from tmux.
	if target.Type == "pane" || target.Type == "window" {
		paneID, windowID, sessionID, tmuxErr := tmuxCurrentIDs()
		if tmuxErr == nil {
			if target.Type == "pane" {
				target.PaneID = paneID
				target.WindowID = windowID
				target.SessionID = sessionID
			} else {
				target.WindowID = target.ID // already @N
				target.SessionID = sessionID
			}
		}
		// Best-effort: if tmux fails, continue with partial Target
	} else {
		// Session target: session_id is the ID itself
		target.SessionID = target.ID
	}

	src := db.SourceTool
	if stateSource == "user" {
		src = db.SourceUser
	} else if stateSource != "tool" {
		return fmt.Errorf("invalid --source %q: must be tool|user", stateSource)
	}

	val, err := db.ParseStateValue(stateValue)
	if err != nil {
		return err
	}

	if val == db.StateFlagged && src == db.SourceTool {
		return fmt.Errorf("--state flagged requires --source user")
	}

	var ifState *db.StateValue
	if stateIfState != "" {
		sv, err := db.ParseStateValue(stateIfState)
		if err != nil {
			return fmt.Errorf("--if-state: %w", err)
		}
		ifState = &sv
	}

	if err := d.StateSet(target, stateTool, val, stateMessage, src, stateForce, ifState); err != nil {
		if errors.Is(err, db.ErrStateLocked) {
			demuxlog.Warn("state set rejected: lock", "target", target, "tool", stateTool, "state", stateValue, "err", err)
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		demuxlog.Error("state set failed", "target", target, "tool", stateTool, "state", stateValue, "err", err)
		return fmt.Errorf("state set: %w", err)
	}
	demuxlog.Debug("state set", "target", target, "tool", stateTool, "state", stateValue)
	return nil
}

func applyStateClear(d *db.DB) error {
	target, err := db.ParseTargetID(clearTarget)
	if err != nil {
		return fmt.Errorf("invalid --target-id: %w", err)
	}

	st, err := d.StateByID(target)
	if err != nil {
		demuxlog.Error("state clear lookup failed", "target", target, "err", err)
		return fmt.Errorf("state lookup: %w", err)
	}
	if st != nil && st.Value == db.StateFlagged && !clearYes {
		demuxlog.Warn("state clear rejected: flagged", "target", target)
		return fmt.Errorf("target is flagged; use --yes to clear it")
	}
	if err := d.StateClear(target); err != nil {
		demuxlog.Error("state clear failed", "target", target, "err", err)
		return err
	}
	demuxlog.Debug("state cleared", "target", target)
	return nil
}

type stateRow struct {
	target  string
	paneID  string
	tool    string
	value   string
	message string
	source  string
	updated string
}

func (r stateRow) Fields() []string {
	return []string{r.target, r.paneID, r.tool, r.value, r.message, r.source, r.updated}
}

// formatTarget resolves the display string for a Target from the live tmux maps.
// Falls back to the raw stable ID if not found in maps.
func formatTarget(t db.Target, paneIDMap, windowIDMap, sessionIDMap map[string]string) string {
	switch t.Type {
	case "pane":
		if resolved, ok := paneIDMap[t.ID]; ok {
			return resolved
		}
	case "window":
		if resolved, ok := windowIDMap[t.ID]; ok {
			return resolved
		}
	case "session":
		if resolved, ok := sessionIDMap[t.ID]; ok {
			return resolved
		}
	}
	return t.ID
}

func runStateList(cmd *cobra.Command, args []string) error {
	d, err := openDB()
	if err != nil {
		return err
	}
	defer d.Close()

	var filterVal db.StateValue
	if v, _ := cmd.Flags().GetString("state"); v != "" {
		sv, err := db.ParseStateValue(v)
		if err != nil {
			return err
		}
		filterVal = sv
	}
	filterTool, _ := cmd.Flags().GetString("tool")

	states, err := d.StateList(filterVal, filterTool)
	if err != nil {
		return fmt.Errorf("state list: %w", err)
	}

	// Build display maps from live tmux (best-effort).
	paneIDMap := map[string]string{}
	windowIDMap := map[string]string{}
	sessionIDMap := map[string]string{}
	if panes, pErr := tmux.ListPanes(); pErr == nil {
		paneIDMap = tmux.PaneIDToTargetMap(panes)
		windowIDMap = tmux.WindowIDToTargetMap(panes)
		sessionIDMap = tmux.SessionIDToNameMap(panes)
	}

	rows := make([]format.Row, len(states))
	for i, st := range states {
		tool := st.Tool
		if tool == "" {
			tool = "-"
		}
		rows[i] = stateRow{
			target:  formatTarget(st.Target, paneIDMap, windowIDMap, sessionIDMap),
			paneID:  st.Target.PaneID,
			tool:    tool,
			value:   st.Value.String(),
			message: st.Message,
			source:  st.Source.String(),
			updated: format.Age(st.UpdatedAt),
		}
	}

	headers := []string{"TARGET", "PANE_ID", "TOOL", "STATE", "MESSAGE", "SOURCE", "UPDATED"}
	fmt.Println(format.Render(resolveFormat(cmd), headers, rows, isTTY()))
	return nil
}
