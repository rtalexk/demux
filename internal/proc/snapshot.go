package proc

import (
    "fmt"
    "path/filepath"
    "regexp"
    "strings"
    "time"

    gops "github.com/shirou/gopsutil/v3/process"
)

var versionRe = regexp.MustCompile(`^\d+\.\d+`)

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

// FriendlyName returns a human-readable process name. The OS-level Name is
// often a truncated or version-string form (e.g. "2.1.83" for Claude Code).
// When Name looks like a version number we parse argv[0] from Cmdline instead.
func (p Process) FriendlyName() string {
    if p.Name != "" && !versionRe.MatchString(p.Name) {
        return p.Name
    }
    // argv[0] is the first whitespace-delimited token of cmdline
    argv0 := strings.Fields(p.Cmdline)
    if len(argv0) == 0 {
        return p.Name
    }
    base := filepath.Base(argv0[0])
    if base == "" || base == "." {
        return p.Name
    }
    return base
}

func BuildTree(procs []Process) map[int32][]Process {
    tree := make(map[int32][]Process, len(procs))
    for _, p := range procs {
        tree[p.PPID] = append(tree[p.PPID], p)
    }
    return tree
}
