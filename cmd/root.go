package cmd

import (
    "fmt"
    "os"
    "time"

    "github.com/mattn/go-isatty"
    "github.com/rtalex/demux/internal/config"
    "github.com/rtalex/demux/internal/db"
    "github.com/rtalex/demux/internal/tui"
    "github.com/spf13/cobra"
)

var formatFlag string

var rootCmd = &cobra.Command{
    Use:   "demux",
    Short: "Monitor tmux sessions, processes, and alerts",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg := loadConfig()
        dbPath, err := db.DefaultPath()
        if err != nil {
            return err
        }
        database, err := db.Open(dbPath)
        if err != nil {
            return err
        }
        defer database.Close()
        return tui.Run(cfg, database)
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

func loadConfig() config.Config {
    path, err := config.DefaultPath()
    if err != nil {
        // Home directory unavailable; use built-in defaults.
        return config.Default()
    }
    cfg, _ := config.Load(path)
    return cfg
}

func openDB() (*db.DB, error) {
    path, err := db.DefaultPath()
    if err != nil {
        return nil, err
    }
    return db.Open(path)
}

func resolveFormat(cmd *cobra.Command) string {
    if formatFlag != "" {
        return formatFlag
    }
    return "table"
}

func isTTY() bool {
    return isatty.IsTerminal(os.Stdout.Fd())
}

func formatAge(t time.Time) string {
    d := time.Since(t)
    switch {
    case d < time.Minute:
        return fmt.Sprintf("%ds ago", int(d.Seconds()))
    case d < time.Hour:
        return fmt.Sprintf("%dm ago", int(d.Minutes()))
    case d < 24*time.Hour:
        return fmt.Sprintf("%dh ago", int(d.Hours()))
    default:
        return fmt.Sprintf("%dd ago", int(d.Hours()/24))
    }
}
