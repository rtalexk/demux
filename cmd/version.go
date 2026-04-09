package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

var Version = "dev"

func printVersion(w io.Writer) {
	fmt.Fprintln(w, Version)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version",
	Run: func(cmd *cobra.Command, args []string) {
		printVersion(cmd.OutOrStdout())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
