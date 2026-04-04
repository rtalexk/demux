package tui

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "time"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    xansi "github.com/charmbracelet/x/ansi"
    runewidth "github.com/mattn/go-runewidth"
    "github.com/rtalexk/demux/internal/config"
    "github.com/rtalexk/demux/internal/db"
    "github.com/rtalexk/demux/internal/git"
    demuxlog "github.com/rtalexk/demux/internal/log"
    "github.com/rtalexk/demux/internal/proc"
    "github.com/rtalexk/demux/internal/query"
    "github.com/rtalexk/demux/internal/session"
    "github.com/rtalexk/demux/internal/tmux"
)

type panel int

const (
    panelSidebar panel = iota
    panelProcList
)

const searchBoxH = 3

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Message types
type tickMsg time.Time
type panesMsg struct {
    panes          []tmux.Pane
    currentSession string // populated by CurrentTarget(); used for startup focus in Task 4
}
type alertsMsg struct{ alerts []db.Alert }
type procDataMsg struct {
    procs  []proc.Process
    cwdMap map[int32]string
    gen    int // generation counter — stale results are discarded
}
type gitResultMsg struct {
    key  string // session name, or "session:window" for deviants
    info git.Info
}

type searchDebounceMsg struct{ gen int }
type queryResultMsg struct {
    result query.Result
    gen    int
}

type Model struct {
    cfg    config.Config
    db     *db.DB
    focus  panel
    width  int
    height int

    panes   []tmux.Pane
    alerts  []db.Alert
    gitInfo map[string]git.Info // keyed by session name
    procs   []proc.Process
    cwdMap  map[int32]string // PID -> CWD, pre-fetched async

    sidebar  SidebarModel
    procList ProcListModel
    detail   DetailModel
    yank     YankModel
    help     HelpModel

    showYank bool
    showHelp bool

    pulse        bool
    spinnerFrame int
    statusMsg    string
    statusExp    time.Time
    ready        bool // true after first panesMsg — gates deferred fetches
    procGen      int  // incremented on window change; discards in-flight proc fetches for old window
    popupMode    bool // true when launched with DEMUX_POPUP=1; quits after attach

    currentSession   string
    startupFocusDone bool

    searchInput SearchInputModel
    queryResult query.Result
    searchGen   int

    sessionsConfig session.SessionsConfig
    configDir      string
}

func New(cfg config.Config, database *db.DB) Model {
    initStyles(ThemeFromConfig(cfg.Theme), cfg.Theme.Processes, cfg.IgnoredProcesses)
    m := Model{
        cfg:       cfg,
        db:        database,
        focus:     panelSidebar,
        gitInfo:   make(map[string]git.Info),
        popupMode: os.Getenv("DEMUX_POPUP") == "1",
    }
    m.searchInput = NewSearchInputModel()
    cfgPath, _ := config.DefaultPath()
    m.configDir = filepath.Dir(cfgPath)
    var loadErr error
    m.sessionsConfig, loadErr = session.LoadConfigSessions(m.configDir)
    if loadErr != nil {
        demuxlog.Error("failed to load config sessions", "dir", m.configDir, "err", loadErr)
    }
    if cfg.Sidebar.DefaultFilter != "" {
        m.sidebar.filter = SidebarFilter(cfg.Sidebar.DefaultFilter)
    } else {
        m.sidebar.filter = FilterTmux
    }
    return m
}

func (m Model) Init() tea.Cmd {
    // Only fetch panes on startup — sidebar renders immediately.
    // fetchAlerts, fetchProcs, and the tick are deferred until panesMsg arrives.
    return m.fetchPanes()
}

func (m Model) View() string {
    if m.width == 0 {
        return "loading..."
    }

    if m.cfg.Mode == "compact" {
        return m.compactView()
    }

    dims := m.buildLayoutDims()
    sidebarTitle := m.buildSidebarTitle()
    procTitle := m.buildProcTitle()

    sidebarContent := m.sidebar.Render(dims.sidebarW, dims.contentH-searchBoxH, m.focus == panelSidebar, sidebarTitle, "")
    searchBox := m.searchInput.View(dims.sidebarW)
    leftCol := lipgloss.JoinVertical(lipgloss.Left, searchBox, sidebarContent)

    procList := m.procList.Render(dims.procW, dims.procH, m.focus == panelProcList, procTitle)
    detail := m.detail.Render(dims.procW, dims.detailH)

    right := lipgloss.JoinVertical(lipgloss.Left, procList, detail)
    body := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, right)

    var full string
    if statusBar := m.buildStatusBar(m.width); statusBar != "" {
        full = lipgloss.JoinVertical(lipgloss.Left, body, statusBar)
    } else {
        full = body
    }

    return m.applyOverlay(full)
}

func (m Model) compactView() string {
    contentH := m.height
    if m.cfg.StatusBar.Show {
        contentH-- // reserve 1 row for status bar
    }

    sidebarTitle := m.buildSidebarTitle()

    // compact mode: sidebar is the only panel and always has focus
    sidebarContent := m.sidebar.Render(m.width, contentH-searchBoxH, true, sidebarTitle, "")
    searchBox := m.searchInput.View(m.width)
    leftCol := lipgloss.JoinVertical(lipgloss.Left, searchBox, sidebarContent)

    var full string
    if statusBar := m.buildStatusBar(m.width); statusBar != "" {
        full = lipgloss.JoinVertical(lipgloss.Left, leftCol, statusBar)
    } else {
        full = leftCol
    }

    return m.applyOverlay(full)
}

// layoutDims holds pre-computed layout dimensions for the normal (non-compact) view.
type layoutDims struct {
    sidebarW int
    procW    int
    contentH int
    procH    int
    detailH  int
}

// buildLayoutDims computes column widths and row heights for the normal view.
func (m Model) buildLayoutDims() layoutDims {
    sidebarW := m.cfg.Sidebar.Width
    if sidebarW <= 0 {
        sidebarW = 30
    }
    procW := m.width - sidebarW
    if procW < 10 {
        procW = 10
    }

    contentH := m.height
    if m.cfg.StatusBar.Show {
        contentH--
    }

    innerW := procW - 2
    detailContent := m.detail.ContentLines(innerW)
    detailH := detailContent + 2
    minDetailH := 4
    maxDetailH := contentH - 4
    if detailH < minDetailH {
        detailH = minDetailH
    }
    if detailH > maxDetailH {
        detailH = maxDetailH
    }

    return layoutDims{
        sidebarW: sidebarW,
        procW:    procW,
        contentH: contentH,
        procH:    contentH - detailH,
        detailH:  detailH,
    }
}

// buildSidebarTitle builds the bordered title string for the sidebar panel.
func (m Model) buildSidebarTitle() string {
    sessionCount := m.sidebar.SessionCount()
    sessionCountStr := statValueStyle.Render(fmt.Sprintf("(%d)", sessionCount))
    filterMark := ""
    if f := m.sidebar.ActiveFilter(); f != FilterTmux {
        filterMark = " [" + string(f) + "]"
    }
    return fmt.Sprintf(" [h] Sessions %s%s ", sessionCountStr, filterMark)
}

// buildProcTitle builds the bordered title string for the process list panel.
func (m Model) buildProcTitle() string {
    bc := m.plainBreadcrumb()
    procTitleSuffix := " "
    if runes := []rune(bc); len(runes) > 0 && isIconRune(runes[len(runes)-1]) {
        procTitleSuffix = "  "
    }
    return " [l] " + bc + procTitleSuffix
}

// applyOverlay wraps base with the help or yank overlay when active.
func (m Model) applyOverlay(base string) string {
    if m.showHelp {
        return overlayCenter(m.help.Render(m.height), base, m.width, m.height)
    }
    if m.showYank {
        return overlayCenter(m.yank.Render(), base, m.width, m.height)
    }
    return base
}

// buildStatusBar assembles the styled status bar string for the given width.
// Returns "" if cfg.StatusBar.Show is false.
func (m Model) buildStatusBar(width int) string {
    if !m.cfg.StatusBar.Show {
        return ""
    }

    var statusBar string
    if m.statusMsg != "" && time.Now().Before(m.statusExp) {
        statusBar = m.statusMsg
    } else if m.cfg.Mode == "compact" {
        statusBar = "  j/k:nav  Enter:open  !:alerts  ?:help  q:quit"
    } else if m.focus == panelSidebar {
        statusBar = "  Tab:cycle  j/k:nav  Enter:select  !:alerts  ?:help  q:quit"
    } else {
        statusBar = "  Tab:cycle  j/k:nav  J/K:jump  x:kill  r:restart  l:log  q:quit"
    }

    spinnerStr := ""
    if m.cfg.Git.ShowSpinner {
        for _, info := range m.gitInfo {
            if info.Loading {
                frame := spinnerFrames[m.spinnerFrame%len(spinnerFrames)]
                spinnerStr = " " + spinnerStyle.Render(frame) + " "
                break
            }
        }
    }

    if spinnerStr != "" {
        leftWidth := width - lipgloss.Width(spinnerStr)
        return lipgloss.JoinHorizontal(lipgloss.Top,
            lipgloss.NewStyle().Width(leftWidth).MaxHeight(1).Render(statusBar),
            spinnerStr,
        )
    }
    return lipgloss.NewStyle().
        Width(width).
        MaxHeight(1).
        Render(statusBar)
}

// overlayCenter places fg centered on top of bg (bgW×bgH visible columns).
// Uses ANSI-aware slicing so the background content remains visible.
func overlayCenter(fg, bg string, bgW, bgH int) string {
    fgLines := strings.Split(fg, "\n")
    bgLines := strings.Split(bg, "\n")

    fgH := len(fgLines)
    fgW := 0
    for _, l := range fgLines {
        if w := lipgloss.Width(l); w > fgW {
            fgW = w
        }
    }

    startY := (bgH - fgH) / 2
    startX := (bgW - fgW) / 2
    if startX < 0 {
        startX = 0
    }
    if startY < 0 {
        startY = 0
    }

    result := make([]string, len(bgLines))
    for i, bgLine := range bgLines {
        fgIdx := i - startY
        if fgIdx < 0 || fgIdx >= fgH {
            result[i] = bgLine
            continue
        }
        left := xansi.Truncate(bgLine, startX, "")
        right := xansi.TruncateLeft(bgLine, startX+fgW, "")
        result[i] = left + fgLines[fgIdx] + right
    }
    return strings.Join(result, "\n")
}

// isIconRune reports whether r is likely a terminal icon that renders as 2
// columns: emoji (runewidth > 1), Nerd Font / Private Use Area glyphs
// (U+E000–U+F8FF, U+F0000+), and common symbol blocks that many terminals
// render wide.
func isIconRune(r rune) bool {
    if runewidth.RuneWidth(r) > 1 {
        return true
    }
    // Private Use Area — Nerd Font icons live here and render as 2-wide
    // in terminals even though Unicode assigns them width 1.
    if r >= 0xE000 && r <= 0xF8FF {
        return true
    }
    if r >= 0xF0000 {
        return true
    }
    return false
}

func (m Model) plainBreadcrumb() string {
    node := m.sidebar.Selected()
    if node == nil {
        return ""
    }
    return node.Session
}

// Run launches the Bubbletea program.
func Run(cfg config.Config, database *db.DB) error {
    m := New(cfg, database)
    p := tea.NewProgram(m, tea.WithAltScreen())
    _, err := p.Run()
    return err
}
