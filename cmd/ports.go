package cmd

import (
	"fmt"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/rtalex/demux/internal/format"
	"github.com/rtalex/demux/internal/proc"
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
		return err
	}

	procs, _ := proc.Snapshot()
	pidToProc := map[int32]proc.Process{}
	for _, p := range procs {
		pidToProc[p.PID] = p
	}

	headers := []string{"PORT", "PID", "PROCESS", "SESSION", "WINDOW", "PANE", "UP"}
	var rows []format.Row

	for _, pi := range ports {
		p := pidToProc[pi.PID]
		rows = append(rows, portRow{
			port:    fmt.Sprintf(":%d", pi.Port),
			pid:     fmt.Sprint(pi.PID),
			process: p.Name,
			session: "—",
			window:  "—",
			pane:    "—",
			up:      formatDuration(p.Uptime),
		})
	}

	isTTYVal := isatty.IsTerminal(os.Stdout.Fd())
	fmt.Println(format.Render(resolveFormat(cmd), headers, rows, isTTYVal))
	return nil
}
