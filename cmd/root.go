package cmd

import (
    "fmt"
    "os"

    "github.com/mattn/go-isatty"
    "github.com/rtalexk/demux/internal/config"
    "github.com/rtalexk/demux/internal/db"
    demuxlog "github.com/rtalexk/demux/internal/log"
    "github.com/rtalexk/demux/internal/tui"
    "github.com/spf13/cobra"
)

var formatFlag string
var compactFlag bool
var searchFlag bool
var logLevelFlag string

var rootCmd = &cobra.Command{
    Use:          "demux",
    Short:        "Monitor tmux sessions, processes, and alerts",
    SilenceUsage: true,
    PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
        cfg := loadConfig()
        levelStr := cfg.Log.Level
        if logLevelFlag != "" {
            levelStr = logLevelFlag
        }
        level, err := demuxlog.ParseLevel(levelStr)
        if err != nil {
            // Invalid level from config or flag — warn but continue with default.
            fmt.Fprintf(os.Stderr, "demux: %v\n", err)
            level = demuxlog.LevelWarn
        }
        logPath, err := demuxlog.DefaultPath()
        if err != nil {
            // Cannot determine home dir — disable logging silently.
            return nil
        }
        closer, err := demuxlog.Open(logPath, level)
        if err != nil {
            fmt.Fprintf(os.Stderr, "demux: failed to open log file: %v\n", err)
            return nil
        }
        // Register closer so the file is flushed/closed on exit.
        // Cobra doesn't expose a post-run hook on root that fires for all children,
        // so we use a finalizer via the cobra RunE pattern.
        cmd.PersistentPostRunE = func(cmd *cobra.Command, args []string) error {
            return closer.Close()
        }
        return nil
    },
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg := loadConfig()
        if compactFlag {
            cfg.Mode = "compact"
        }
        if searchFlag {
            cfg.Sidebar.FocusSearchOnOpen = true
        }
        dbPath, err := db.DefaultPath()
        if err != nil {
            return err
        }
        database, err := db.Open(dbPath)
        if err != nil {
            return fmt.Errorf("open db: %w", err)
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
    rootCmd.PersistentFlags().StringVar(&logLevelFlag, "log-level", "", "Log level: off|error|warn|info|debug (overrides config)")
    rootCmd.PersistentFlags().BoolVar(&compactFlag, "compact", false, "Launch in compact mode (sidebar + search only)")
    rootCmd.PersistentFlags().BoolVar(&searchFlag, "search", false, "Start with focus in the search input")
}

func loadConfig() config.Config {
    path, err := config.DefaultPath()
    if err != nil {
        demuxlog.Warn("cannot determine config path", "err", err)
        return config.Default()
    }
    cfg, err := config.Load(path)
    if err != nil {
        demuxlog.Warn("failed to load config, using defaults", "path", path, "err", err)
        return config.Default()
    }
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

