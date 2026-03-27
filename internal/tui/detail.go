package tui

import (
    "fmt"
    "strings"

    "github.com/rtalex/demux/internal/config"
    "github.com/rtalex/demux/internal/db"
    "github.com/rtalex/demux/internal/git"
    "github.com/rtalex/demux/internal/proc"
    "github.com/rtalex/demux/internal/tmux"
)


type DetailSelection int

const (
    DetailNone DetailSelection = iota
    DetailSession
    DetailWindow
    DetailProc
)

type DetailModel struct {
    selType DetailSelection
    cfg     config.Config

    // session
    session    string
    sessionCWD string
    gitInfo    git.Info
    prInfo     string
    winCount   int
    procCount  int
    alertCount int

    // window
    windowIndex int
    windowPanes []tmux.Pane
    windowGit   git.Info
    windowAlert *db.Alert

    // proc
    proc     proc.Process
    procGit  git.Info
    procPort string
    procCWD  string
}

// ContentLines returns the number of visual rows the detail content will occupy
// at the given inner width (panel width minus border). Long values wrap at valueW.
func (d DetailModel) ContentLines(innerWidth int) int {
    var lines []string
    switch d.selType {
    case DetailSession:
        lines = d.renderSession()
    case DetailWindow:
        lines = d.renderWindow()
    case DetailProc:
        lines = d.renderProc(innerWidth)
    default:
        return 1
    }
    valueW := innerWidth - 10 // label is 10 wide
    if valueW < 1 {
        valueW = 1
    }
    total := 0
    for _, l := range lines {
        if l == "" {
            total++
            continue
        }
        plain := stripANSI(l)
        // plain width = label(10) + value; count wrapped rows
        rows := (len([]rune(plain)) + innerWidth - 1) / innerWidth
        if rows < 1 {
            rows = 1
        }
        _ = valueW
        total += rows
    }
    return total
}

func (d DetailModel) Render(width, height int) string {
    innerW := width - 2
    var lines []string
    switch d.selType {
    case DetailSession:
        lines = d.renderSession()
    case DetailWindow:
        lines = d.renderWindow()
    case DetailProc:
        lines = d.renderProc(innerW)
    default:
        lines = []string{
            noSelectionStyle.Render("No selection"),
        }
    }
    maxLines := height - 2
    if maxLines < 0 {
        maxLines = 0
    }
    if len(lines) > maxLines {
        lines = lines[:maxLines]
    }
    inner := strings.Join(lines, "\n")
    return detailBorder.Width(innerW).Height(height - 2).Render(inner)
}

func row(label, value string) string {
    return detailLabelStyle.Render(label) + detailValueStyle.Render(value)
}

func (d DetailModel) renderSession() []string {
    lines := []string{
        row("path", d.sessionCWD),
    }
    if d.gitInfo.Worktree != "" {
        lines = append(lines, row("worktree", d.gitInfo.Worktree))
    }
    if d.gitInfo.Branch != "" {
        branch := d.gitInfo.Branch
        if ind := gitIndicatorsLong(d.gitInfo); ind != "" {
            branch += "  " + ind
        }
        lines = append(lines, row("branch", branch))
    }
    if d.prInfo != "" {
        lines = append(lines, row("pr", d.prInfo))
    }
    lines = append(lines,
        "",
        row("windows", fmt.Sprint(d.winCount)),
        row("procs", fmt.Sprint(d.procCount)),
        row("alerts", fmt.Sprint(d.alertCount)),
    )
    return lines
}

func (d DetailModel) renderWindow() []string {
    var lines []string
    for _, p := range d.windowPanes {
        lines = append(lines, fmt.Sprintf("  pane %d  %s", p.PaneIndex, p.CWD))
    }
    if d.windowGit.Branch != "" {
        lines = append(lines, "")
        if d.windowGit.Worktree != "" {
            lines = append(lines, row("worktree", d.windowGit.Worktree))
        }
        branch := d.windowGit.Branch
        if ind := gitIndicatorsLong(d.windowGit); ind != "" {
            branch += "  " + ind
        }
        lines = append(lines, row("branch", branch))
    }
    if d.windowAlert != nil {
        lines = append(lines, "")
        lines = append(lines, row("alert", d.windowAlert.Level+": "+d.windowAlert.Reason))
    }
    return lines
}

func (d DetailModel) renderProc(innerWidth int) []string {
    valueW := innerWidth - 10 // label column is 10 wide
    if valueW < 8 {
        valueW = 8
    }
    cmd := d.proc.Cmdline
    cmdRunes := []rune(cmd)
    if len(cmdRunes) > valueW {
        cmdRunes = append(cmdRunes[:valueW-1], '…')
        cmd = string(cmdRunes)
    }
    lines := []string{
        row("name", d.proc.FriendlyName()),
        row("pid", fmt.Sprint(d.proc.PID)),
        row("cmd", cmd),
        row("uptime", formatProcDuration(d.proc.Uptime)),
        row("memory", fmt.Sprintf("%.1fMB", float64(d.proc.MemRSS)/1024/1024)),
    }
    if d.procPort != "" {
        lines = append(lines, row("port", d.procPort))
    }
    if d.procCWD != "" {
        lines = append(lines, row("cwd", d.procCWD))
    }
    if d.procGit.Branch != "" {
        lines = append(lines, "")
        if d.procGit.Worktree != "" {
            lines = append(lines, row("worktree", d.procGit.Worktree))
        }
        branch := d.procGit.Branch
        if ind := gitIndicatorsLong(d.procGit); ind != "" {
            branch += "  " + ind
        }
        lines = append(lines, row("branch", branch))
    }
    return lines
}

func gitIndicatorsLong(info git.Info) string {
    var parts []string
    if info.Ahead > 0 {
        parts = append(parts, gitAheadStyle.Render(fmt.Sprintf("↑%d", info.Ahead)))
    }
    if info.Behind > 0 {
        parts = append(parts, gitBehindStyle.Render(fmt.Sprintf("↓%d", info.Behind)))
    }
    if info.Dirty {
        parts = append(parts, gitDirtyStyle.Render("*"))
    }
    return strings.Join(parts, " ")
}
