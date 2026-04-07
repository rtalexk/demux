package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/rtalexk/demux/internal/db"
	"github.com/rtalexk/demux/internal/format"
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
	stateSetCmd.Flags().StringVar(&stateTarget, "target", "", "Target: session:window.pane (required)")
	stateSetCmd.Flags().StringVar(&stateValue, "state", "", "State: working|waiting|done|error|flagged (required)")
	stateSetCmd.Flags().StringVar(&stateTool, "tool", "", "Tool name")
	stateSetCmd.Flags().StringVar(&stateMessage, "message", "", "Human-readable detail")
	stateSetCmd.Flags().StringVar(&stateSource, "source", "tool", "Source: tool|user")
	stateSetCmd.Flags().BoolVar(&stateForce, "force", false, "Override write-lock (allow overwriting another tool's active state)")
	stateSetCmd.Flags().StringVar(&stateIfState, "if-state", "", "Only write if current state matches this value")
	stateSetCmd.MarkFlagRequired("target")
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
	stateClearCmd.Flags().StringVar(&clearTarget, "target", "", "Target to clear (required)")
	stateClearCmd.Flags().BoolVar(&clearYes, "yes", false, "Confirm clearing a flagged target")
	stateClearCmd.MarkFlagRequired("target")

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

	if err := d.StateSet(stateTarget, stateTool, val, stateMessage, src, stateForce, ifState); err != nil {
		if errors.Is(err, db.ErrStateLocked) {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return fmt.Errorf("state set: %w", err)
	}
	return nil
}

func applyStateClear(d *db.DB) error {
	st, err := d.StateByTarget(clearTarget)
	if err != nil {
		return fmt.Errorf("state lookup: %w", err)
	}
	if st != nil && st.Value == db.StateFlagged && !clearYes {
		return fmt.Errorf("target is flagged; use --yes to clear it")
	}
	return d.StateClear(clearTarget)
}

type stateRow struct {
	target  string
	tool    string
	value   string
	message string
	source  string
	updated string
}

func (r stateRow) Fields() []string {
	return []string{r.target, r.tool, r.value, r.message, r.source, r.updated}
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

	rows := make([]format.Row, len(states))
	for i, st := range states {
		tool := st.Tool
		if tool == "" {
			tool = "-"
		}
		rows[i] = stateRow{
			target:  st.Target,
			tool:    tool,
			value:   st.Value.String(),
			message: st.Message,
			source:  st.Source.String(),
			updated: format.Age(st.UpdatedAt),
		}
	}

	headers := []string{"TARGET", "TOOL", "STATE", "MESSAGE", "SOURCE", "UPDATED"}
	fmt.Println(format.Render(resolveFormat(cmd), headers, rows, isTTY()))
	return nil
}
