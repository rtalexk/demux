package cmd

import (
	"fmt"

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

	var warns, errors int
	for _, a := range alerts {
		switch a.Level {
		case "warn":
			warns++
		case "error":
			errors++
		}
	}

	fmtName := resolveFormat(cmd)
	if fmtName == "table" {
		// status command defaults to tmux format
		fmtName = "tmux"
	}

	switch fmtName {
	case "tmux":
		if warns == 0 && errors == 0 {
			fmt.Print("#[fg=green]●#[default]")
		} else if errors > 0 {
			fmt.Printf("#[fg=yellow]● %d#[default]  #[fg=red,bold]● %d#[default]", warns, errors)
		} else {
			fmt.Printf("#[fg=yellow]● %d#[default]", warns)
		}
	case "json":
		fmt.Printf(`{"warns":%d,"errors":%d}`+"\n", warns, errors)
	default:
		if warns == 0 && errors == 0 {
			fmt.Println("ok")
		} else {
			fmt.Printf("warns=%d errors=%d\n", warns, errors)
		}
	}
	return nil
}
