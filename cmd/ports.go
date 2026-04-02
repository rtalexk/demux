package cmd

import (
    "fmt"
    "os"

    "github.com/mattn/go-isatty"
    "github.com/rtalexk/demux/internal/format"
    demuxlog "github.com/rtalexk/demux/internal/log"
    "github.com/rtalexk/demux/internal/proc"
    "github.com/rtalexk/demux/internal/tmux"
    "github.com/spf13/cobra"
)

var portsCmd = &cobra.Command{
    Use:   "ports",
    Short: "List all TCP listening ports",
    RunE:  runPorts,
}

func init() {
    rootCmd.AddCommand(portsCmd)
}

type portRow struct {
    port, pid, process, session, window, pane, up string
}

func (r portRow) Fields() []string {
    return []string{r.port, r.pid, r.process, r.session, r.window, r.pane, r.up}
}

func runPorts(cmd *cobra.Command, _ []string) error {
    ports, err := proc.ListeningPorts()
    if err != nil {
        return fmt.Errorf("list ports: %w", err)
    }

    procs, err := proc.Snapshot()
    if err != nil {
        demuxlog.Warn("proc snapshot failed", "err", err)
    }
    pidToProc := map[int32]proc.Process{}
    for _, p := range procs {
        pidToProc[p.PID] = p
    }

    allPanes, err := tmux.ListPanes()
    if err != nil {
        demuxlog.Warn("tmux panes unavailable", "err", err)
    }
    type paneRef struct{ session, window, pane string }
    cwdToPane := map[string]paneRef{}
    for _, p := range allPanes {
        if _, exists := cwdToPane[p.CWD]; !exists {
            cwdToPane[p.CWD] = paneRef{
                session: p.Session,
                window:  fmt.Sprint(p.WindowIndex),
                pane:    fmt.Sprint(p.PaneIndex),
            }
        }
    }

    headers := []string{"PORT", "PID", "PROCESS", "SESSION", "WINDOW", "PANE", "UP"}
    var rows []format.Row

    for _, pi := range ports {
        p := pidToProc[pi.PID]
        session, window, pane := "—", "—", "—"

        cwd, err := proc.CWD(pi.PID)
        if err == nil && cwd != "" {
            if ref, ok := cwdToPane[cwd]; ok {
                session = ref.session
                window = ref.window
                pane = ref.pane
            }
        }

        rows = append(rows, portRow{
            port:    fmt.Sprintf(":%d", pi.Port),
            pid:     fmt.Sprint(pi.PID),
            process: p.Name,
            session: session,
            window:  window,
            pane:    pane,
            up:      format.Duration(p.Uptime),
        })
    }

    isTTYVal := isatty.IsTerminal(os.Stdout.Fd())
    fmt.Println(format.Render(resolveFormat(cmd), headers, rows, isTTYVal))
    return nil
}
