package cmd

import (
	"fmt"
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

func countStatesByValue(states []db.ToolState) (waiting, errs, flagged, done, idle int) {
	for _, s := range states {
		switch s.Value {
		case db.StateWaiting:
			waiting++
		case db.StateError:
			errs++
		case db.StateFlagged:
			flagged++
		case db.StateDone:
			done++
		case db.StateIdle:
			idle++
		}
	}
	return
}

func tmuxStatusParts(waiting, errs, flagged, done, idle int, cfg config.Config) string {
	th := cfg.Theme
	if waiting == 0 && errs == 0 && flagged == 0 && done == 0 && idle == 0 {
		return fmt.Sprintf("#[fg=%s]%s#[default]", th.ColorStateDone, th.IconStateDone)
	}
	var parts []string
	if errs > 0 {
		parts = append(parts, tmuxCounter(th.ColorStateError+",bold", th.IconStateError, errs))
	}
	if flagged > 0 {
		parts = append(parts, tmuxCounter(th.ColorStateFlagged, th.IconStateFlagged, flagged))
	}
	if waiting > 0 {
		parts = append(parts, tmuxCounter(th.ColorStateWaiting, th.IconStateWaiting, waiting))
	}
	if done > 0 {
		parts = append(parts, tmuxCounter(th.ColorStateDone, th.IconStateDone, done))
	}
	if idle > 0 {
		parts = append(parts, tmuxCounter(th.ColorStateIdle, th.IconStateIdle, idle))
	}
	return strings.Join(parts, " ") + "#[default]"
}

func textStatusParts(waiting, errs, flagged, done, idle int) string {
	if waiting == 0 && errs == 0 && flagged == 0 && done == 0 && idle == 0 {
		return "ok"
	}
	var parts []string
	if errs > 0 {
		parts = append(parts, fmt.Sprintf("errors=%d", errs))
	}
	if flagged > 0 {
		parts = append(parts, fmt.Sprintf("flagged=%d", flagged))
	}
	if waiting > 0 {
		parts = append(parts, fmt.Sprintf("waiting=%d", waiting))
	}
	if done > 0 {
		parts = append(parts, fmt.Sprintf("done=%d", done))
	}
	if idle > 0 {
		parts = append(parts, fmt.Sprintf("idle=%d", idle))
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
	waiting, errs, flagged, done, idle := countStatesByValue(states)

	cfg := loadConfig()

	fmtName := resolveFormat(cmd)
	if fmtName == "table" {
		fmtName = "tmux"
	}

	switch fmtName {
	case "tmux":
		fmt.Print(tmuxStatusParts(waiting, errs, flagged, done, idle, cfg))
	default:
		fmt.Println(textStatusParts(waiting, errs, flagged, done, idle))
	}
	return nil
}
