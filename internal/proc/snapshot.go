package proc

import (
    "fmt"
    "time"

    gops "github.com/shirou/gopsutil/v3/process"
)

type Process struct {
    PID     int32
    PPID    int32
    Name    string
    Cmdline string
    CPU     float64
    MemRSS  uint64
    Uptime  time.Duration
}

func Snapshot() ([]Process, error) {
    pids, err := gops.Pids()
    if err != nil {
        return nil, fmt.Errorf("pids: %w", err)
    }

    procs := make([]Process, 0, len(pids))
    for _, pid := range pids {
        p, err := gops.NewProcess(pid)
        if err != nil {
            continue
        }
        name, _ := p.Name()
        ppid, _ := p.Ppid()
        cmdline, _ := p.Cmdline()
        cpu, _ := p.CPUPercent()
        mem, _ := p.MemoryInfo()
        created, _ := p.CreateTime()

        var rss uint64
        if mem != nil {
            rss = mem.RSS
        }
        var uptime time.Duration
        if created > 0 {
            uptime = time.Since(time.UnixMilli(created))
        }

        procs = append(procs, Process{
            PID:     pid,
            PPID:    ppid,
            Name:    name,
            Cmdline: cmdline,
            CPU:     cpu,
            MemRSS:  rss,
            Uptime:  uptime,
        })
    }
    return procs, nil
}

func BuildTree(procs []Process) map[int32][]Process {
    tree := make(map[int32][]Process, len(procs))
    for _, p := range procs {
        tree[p.PPID] = append(tree[p.PPID], p)
    }
    return tree
}
