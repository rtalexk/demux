package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var formatFlag string

var rootCmd = &cobra.Command{
	Use:   "demux",
	Short: "Monitor tmux sessions, processes, and alerts",
	RunE: func(cmd *cobra.Command, args []string) error {
		// TUI launch — placeholder until Task 15
		fmt.Println("TUI not yet implemented")
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&formatFlag, "format", "", "Output format: text|table|json")
}
