package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rtalexk/demux/internal/config"
	"github.com/rtalexk/demux/internal/db"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Output compact summary for tmux status bar",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func tmuxCounter(color, icon string, count int) string {
	return fmt.Sprintf("#[fg=%s]%s %d", color, icon, count)
}

type stateCounter struct {
	Tool   string
	Value  db.StateValue
	Source db.StateSource
	Count  int
}

func statusCounters(states []db.ToolState) []stateCounter {
	counts := make(map[stateCounter]int)
	for _, st := range states {
		switch st.Value {
		case db.StateError, db.StateFlagged, db.StateWaiting, db.StateDone, db.StateIdle:
			key := stateCounter{Tool: st.Tool, Value: st.Value}
			if st.Source == db.SourceUser && st.Value == db.StateFlagged {
				key.Tool = "user"
				key.Source = db.SourceUser
			}
			counts[key]++
		}
	}

	counters := make([]stateCounter, 0, len(counts))
	for counter, count := range counts {
		counter.Count = count
		counters = append(counters, counter)
	}
	sort.Slice(counters, func(i, j int) bool {
		if counters[i].Value != counters[j].Value {
			return statusStateOrder(counters[i].Value) < statusStateOrder(counters[j].Value)
		}
		if counters[i].Tool != counters[j].Tool {
			return counters[i].Tool < counters[j].Tool
		}
		return counters[i].Source < counters[j].Source
	})
	return counters
}

func statusStateOrder(value db.StateValue) int {
	switch value {
	case db.StateError:
		return 0
	case db.StateFlagged:
		return 1
	case db.StateWaiting:
		return 2
	case db.StateDone:
		return 3
	case db.StateIdle:
		return 4
	default:
		return 5
	}
}

func statusToolMarker(st db.ToolState, cfg config.Config) string {
	if st.Tool == "" || (st.Source == db.SourceUser && st.Value == db.StateFlagged) {
		return ""
	}

	if tool, ok := cfg.Tool(st.Tool); ok {
		color := tool.Color
		if color == "" {
			color = cfg.Theme.ColorFgMuted
		}
		return fmt.Sprintf("#[fg=%s]%s", color, tool.Icon)
	}

	runes := []rune(st.Tool)
	label := st.Tool
	if len(runes) > 7 {
		label = string(runes[:6]) + "…"
	}
	return fmt.Sprintf("#[fg=%s]? %s", cfg.Theme.ColorFgMuted, label)
}

func tmuxStateCounter(counter stateCounter, cfg config.Config) string {
	th := cfg.Theme
	var color, icon string
	switch counter.Value {
	case db.StateError:
		color, icon = th.ColorStateError+",bold", th.IconStateError
	case db.StateFlagged:
		color, icon = th.ColorStateFlagged, th.IconStateFlagged
	case db.StateWaiting:
		color, icon = th.ColorStateWaiting, th.IconStateWaiting
	case db.StateDone:
		color, icon = th.ColorStateDone, th.IconStateDone
	case db.StateIdle:
		color, icon = th.ColorStateIdle, th.IconStateIdle
	}

	marker := statusToolMarker(db.ToolState{Tool: counter.Tool, Value: counter.Value, Source: counter.Source}, cfg)
	if marker == "" {
		return tmuxCounter(color, icon, counter.Count)
	}
	return marker + " " + tmuxCounter(color, icon, counter.Count)
}

func tmuxStatusParts(states []db.ToolState, cfg config.Config) string {
	counters := statusCounters(states)
	if len(counters) == 0 {
		return fmt.Sprintf("#[fg=%s]%s#[default]", cfg.Theme.ColorStateDone, cfg.Theme.IconStatusClean)
	}
	parts := make([]string, 0, len(counters))
	for _, counter := range counters {
		parts = append(parts, tmuxStateCounter(counter, cfg))
	}
	return strings.Join(parts, " ") + "#[default]"
}

func textStatusParts(states []db.ToolState, _ config.Config) string {
	counters := statusCounters(states)
	if len(counters) == 0 {
		return "ok"
	}
	parts := make([]string, 0, len(counters))
	for _, counter := range counters {
		tool := counter.Tool
		if counter.Source == db.SourceUser && counter.Value == db.StateFlagged {
			tool = "user"
		}
		parts = append(parts, fmt.Sprintf("%s.%s=%d", tool, counter.Value, counter.Count))
	}
	return strings.Join(parts, " ")
}

func runStatus(cmd *cobra.Command, _ []string) error {
	database, err := openDB()
	if err != nil {
		return err
	}
	defer database.Close()

	states, err := database.StateList(0, "")
	if err != nil {
		return fmt.Errorf("list states: %w", err)
	}
	cfg := loadConfig()

	fmtName := resolveFormat(cmd)
	if fmtName == "table" {
		fmtName = "tmux"
	}

	switch fmtName {
	case "tmux":
		fmt.Print(tmuxStatusParts(states, cfg))
	default:
		fmt.Println(textStatusParts(states, cfg))
	}
	return nil
}
