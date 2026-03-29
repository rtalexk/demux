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

func tmuxCounter(style, icon string, count int) string {
    return fmt.Sprintf("%s%s %d", style, icon, count)
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

    var infos, warns, errors int
    for _, a := range alerts {
        switch a.Level {
        case "info":
            infos++
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
        if infos == 0 && warns == 0 && errors == 0 {
            fmt.Print("#[fg=green]#[default]")
        } else {
            sep := ""
            if infos > 0 {
                fmt.Print(tmuxCounter("#[fg=cyan]", "", infos))
                sep = " "
            }
            if warns > 0 {
                fmt.Printf("%s%s", sep, tmuxCounter("#[fg=yellow]", "", warns))
                sep = " "
            }
            if errors > 0 {
                fmt.Printf("%s%s", sep, tmuxCounter("#[fg=red,bold]", "", errors))
            }
            fmt.Print("#[default]")
        }
    case "json":
        fmt.Printf(`{"infos":%d,"warns":%d,"errors":%d}`+"\n", infos, warns, errors)
    default:
        if infos == 0 && warns == 0 && errors == 0 {
            fmt.Println("ok")
        } else {
            fmt.Printf("infos=%d warns=%d errors=%d\n", infos, warns, errors)
        }
    }
    return nil
}
