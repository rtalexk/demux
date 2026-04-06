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

func tmuxCounter(style, icon string, count int) string {
	return fmt.Sprintf("%s%s %d", style, icon, count)
}

func countStatesByValue(states []db.ToolState) (waiting, errs int) {
	for _, s := range states {
		switch s.Value {
		case db.StateWaiting:
			waiting++
		case db.StateError:
			errs++
		}
	}
	return
}

func tmuxStatusParts(waiting, errs int, cfg config.Config) string {
	if waiting == 0 && errs == 0 {
		return "#[fg=#a6e3a1]✔#[default]"
	}
	var parts []string
	if errs > 0 {
		parts = append(parts, tmuxCounter("#[fg=#f38ba8,bold]", "✗", errs))
	}
	if waiting > 0 {
		parts = append(parts, tmuxCounter("#[fg=#f9e2af]", "●", waiting))
	}
	return strings.Join(parts, " ") + "#[default]"
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
	waiting, errs := countStatesByValue(states)

	cfg := loadConfig()

	fmtName := resolveFormat(cmd)
	if fmtName == "table" {
		fmtName = "tmux"
	}

	var out string
	switch fmtName {
	case "tmux":
		out = tmuxStatusParts(waiting, errs, cfg)
		fmt.Print(out)
	default:
		if waiting == 0 && errs == 0 {
			out = "ok"
		} else {
			var parts []string
			if errs > 0 {
				parts = append(parts, fmt.Sprintf("errors=%d", errs))
			}
			if waiting > 0 {
				parts = append(parts, fmt.Sprintf("waiting=%d", waiting))
			}
			out = strings.Join(parts, " ")
		}
		fmt.Println(out)
	}
	return nil
}
