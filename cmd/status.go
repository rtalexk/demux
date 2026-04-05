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

func countAlertsByLevel(alerts []db.Alert) (infos, warns, errors, defers int) {
	for _, a := range alerts {
		switch a.Level {
		case "info":
			infos++
		case "warn":
			warns++
		case "error":
			errors++
		case "defer":
			defers++
		}
	}
	return
}

func tmuxStatusParts(infos, warns, errors, defers int, cfg config.Config) string {
	if infos == 0 && warns == 0 && errors == 0 && defers == 0 {
		return "#[fg=green]#[default]"
	}
	var parts []string
	if errors > 0 {
		parts = append(parts, tmuxCounter("#[fg=red,bold]", cfg.Theme.IconAlertError, errors))
	}
	if warns > 0 {
		parts = append(parts, tmuxCounter("#[fg=yellow]", cfg.Theme.IconAlertWarn, warns))
	}
	if infos > 0 {
		parts = append(parts, tmuxCounter("#[fg=cyan]", cfg.Theme.IconAlertInfo, infos))
	}
	if defers > 0 {
		parts = append(parts, tmuxCounter("#[fg=#b4befe]", cfg.Theme.IconAlertDefer, defers))
	}
	return strings.Join(parts, " ") + "#[default]"
}

func formatStatusOutput(fmtName string, infos, warns, errors, defers int, cfg config.Config) string {
	switch fmtName {
	case "tmux":
		return tmuxStatusParts(infos, warns, errors, defers, cfg)
	case "json":
		// defers intentionally omitted; JSON schema is stable and order is not meaningful.
		return fmt.Sprintf(`{"infos":%d,"warns":%d,"errors":%d}`, infos, warns, errors)
	default:
		if infos == 0 && warns == 0 && errors == 0 && defers == 0 {
			return "ok"
		}
		var parts []string
		if errors > 0 {
			parts = append(parts, fmt.Sprintf("errors=%d", errors))
		}
		if warns > 0 {
			parts = append(parts, fmt.Sprintf("warns=%d", warns))
		}
		if infos > 0 {
			parts = append(parts, fmt.Sprintf("infos=%d", infos))
		}
		if defers > 0 {
			parts = append(parts, fmt.Sprintf("defers=%d", defers))
		}
		return strings.Join(parts, " ")
	}
}

func runStatus(cmd *cobra.Command, _ []string) error {
	database, err := openDB()
	if err != nil {
		return err
	}
	defer database.Close()

	alerts, err := database.AlertList()
	if err != nil {
		return err
	}

	infos, warns, errors, defers := countAlertsByLevel(alerts)

	cfg := loadConfig()

	fmtName := resolveFormat(cmd)
	if fmtName == "table" {
		fmtName = "tmux"
	}

	out := formatStatusOutput(fmtName, infos, warns, errors, defers, cfg)
	if fmtName == "tmux" {
		fmt.Print(out)
	} else {
		fmt.Println(out)
	}
	return nil
}
