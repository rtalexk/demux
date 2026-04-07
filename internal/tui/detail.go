package tui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/rtalexk/demux/internal/config"
	"github.com/rtalexk/demux/internal/db"
	"github.com/rtalexk/demux/internal/format"
	"github.com/rtalexk/demux/internal/git"
	"github.com/rtalexk/demux/internal/proc"
	"github.com/rtalexk/demux/internal/tmux"
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
	session        string
	sessionCWD     string
	configPath     string // populated for config-only (non-live) sessions
	configWorktree string // worktree name for config-only sessions with Worktree=true
	isConfigOnly   bool   // true when session exists in config but is not currently live
	gitInfo        git.Info
	prInfo         string
	winCount       int
	paneCount      int
	procCount      int

	// session states (non-idle states for the focused session)
	sessionStates []db.ToolState

	// window
	windowIndex int
	windowPanes []tmux.Pane
	windowGit   git.Info

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

func worktreeValue(info git.Info) string {
	if info.RepoRoot == "" {
		return info.Worktree
	}
	root := info.RepoRoot
	// Bare-repo worktree convention: demux/.bare + demux/main/ → show parent "demux"
	if filepath.Base(root) == info.Worktree {
		root = filepath.Dir(root)
	}
	return info.Worktree + " (" + filepath.Base(root) + ")"
}

func row(label, value string) string {
	return detailLabelStyle.Render(label) + detailValueStyle.Render(value)
}

func inlineStat(label, value string) string {
	return detailLabelStyle.Width(0).Render(label+":") + " " + detailValueStyle.Render(value)
}

// renderStateSection builds detail lines for displayable states of the focused session.
// Returns nil when there are no states to show.
func (d DetailModel) renderStateSection() []string {
	var active []db.ToolState
	for _, st := range d.sessionStates {
		effective := st
		effective.Value = ageDrivenValue(st, d.cfg.Tui.DoneIdleAfterSecs)
		if effective.Value.IsDisplayable() {
			active = append(active, effective)
		}
	}
	if len(active) == 0 {
		return nil
	}
	// Sort by target for stable ordering.
	sort.Slice(active, func(i, j int) bool { return active[i].Target < active[j].Target })
	lines := []string{""}
	for _, st := range active {
		icon := stateIcon(st.Value)
		target := st.Target
		// Strip session prefix: "mysess:1.0" → "win1·p0"
		if prefix := d.session + ":"; strings.HasPrefix(target, prefix) {
			rest := target[len(prefix):]
			parts := strings.SplitN(rest, ".", 2)
			if len(parts) == 2 {
				target = "win" + parts[0] + "·p" + parts[1]
			}
		}
		tool := st.Tool
		if tool == "" {
			tool = "-"
		}
		stateStr := st.Value.String()
		line := icon + "  " + tool + "  " + target + "  " + stateStr
		lines = append(lines, line)
		if st.Message != "" {
			lines = append(lines, "   "+st.Message)
		}
	}
	return lines
}

func (d DetailModel) renderSession() []string {
	if d.isConfigOnly {
		var lines []string
		if d.configPath != "" {
			lines = append(lines, row("path", format.ShortenPath(d.configPath, d.cfg.PathAliases)))
		}
		if d.configWorktree != "" {
			lines = append(lines, row("worktree", d.configWorktree))
		}
		return lines
	}
	lines := []string{
		row("path", format.ShortenPath(d.sessionCWD, d.cfg.PathAliases)),
	}
	if d.gitInfo.Worktree != "" {
		lines = append(lines, row("worktree", worktreeValue(d.gitInfo)))
	} else if d.gitInfo.IsWorktreeRoot {
		bareStr := lipgloss.NewStyle().Italic(true).Render("_bare_")
		lines = append(lines, row("worktree", bareStr+" ("+filepath.Base(d.gitInfo.Dir)+")"))
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
		inlineStat("windows", fmt.Sprint(d.winCount))+"   "+
			inlineStat("panes", fmt.Sprint(d.paneCount))+"   "+
			inlineStat("procs", fmt.Sprint(d.procCount)),
	)
	if stateLines := d.renderStateSection(); stateLines != nil {
		lines = append(lines, stateLines...)
	}
	return lines
}

func (d DetailModel) renderWindow() []string {
	lines := []string{
		row("path", format.ShortenPath(d.sessionCWD, d.cfg.PathAliases)),
	}
	if d.gitInfo.Worktree != "" {
		lines = append(lines, row("worktree", worktreeValue(d.gitInfo)))
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
		inlineStat("panes", fmt.Sprint(len(d.windowPanes)))+"   "+
			inlineStat("procs", fmt.Sprint(d.procCount)),
	)
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
		lines = append(lines, row("cwd", format.ShortenPath(d.procCWD, d.cfg.PathAliases)))
	}
	if d.procGit.Branch != "" {
		lines = append(lines, "")
		if d.procGit.Worktree != "" {
			lines = append(lines, row("worktree", worktreeValue(d.procGit)))
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
