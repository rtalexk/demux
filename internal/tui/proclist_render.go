package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/rtalexk/demux/internal/format"
	"github.com/rtalexk/demux/internal/proc"
	"github.com/rtalexk/demux/internal/query"
)

// renderedLine pairs a node index with its rendered text, used to build the
// full line list before viewport selection.
type renderedLine struct {
	nodeIdx int
	text    string
}

// computeViewport selects the visible slice of allLines given cursor position,
// scroll offset, and available row count. Returns visible texts and scroll hints.
// Pure function — safe to call from read-only View/Render methods.
func computeViewport(lines []renderedLine, cursor, offset, maxRows int) (visible []string, hasAbove, hasBelow bool) {
	if len(lines) == 0 {
		return nil, false, false
	}
	hasAbove = offset > 0
	contentRows := maxRows
	if hasAbove {
		contentRows--
	}
	// First pass: check if content overflows to determine hasBelow.
	rowCount := 0
	for _, rl := range lines {
		if rl.nodeIdx < offset {
			continue
		}
		entryRows := strings.Count(rl.text, "\n") + 1
		if rowCount+entryRows > contentRows {
			hasBelow = true
			break
		}
		rowCount += entryRows
	}
	// If hasBelow, shrink contentRows by one more for the ▼ hint.
	if hasBelow {
		contentRows = maxRows
		if hasAbove {
			contentRows--
		}
		contentRows-- // for ▼ hint
	}
	// Second pass: build visible with the final contentRows.
	rowCount = 0
	for _, rl := range lines {
		if rl.nodeIdx < offset {
			continue
		}
		entryRows := strings.Count(rl.text, "\n") + 1
		if rowCount+entryRows > contentRows {
			break
		}
		visible = append(visible, rl.text)
		rowCount += entryRows
	}
	return visible, hasAbove, hasBelow
}

func (p ProcListModel) Render(width, height int, focused bool, title string) string {
	border := procBorderInactive
	if focused {
		border = procBorderActive
	}
	innerW := width - 2

	// Right border title: show the primary CWD (session or window path).
	rightTitle := ""
	if p.primaryCWD != "" {
		rightTitle = " " + format.ShortenPath(p.primaryCWD, p.cfg.PathAliases) + " "
	}

	if p.sessionAlert != nil {
		title += paneSepStyle.Render("──") + " " + alertIcon(p.sessionAlert.Level) + " " + alertBadge(p.sessionAlert.Level, p.sessionAlert.Reason)
	}

	if len(p.nodes) == 0 {
		hint := "Select a window with Enter"
		inner := noSelectionStyle.Render(hint)
		return injectBorderTitles(border.Width(width-2).Height(height-2).Render(inner), title, rightTitle)
	}

	// build the full rendered line list, tracking node index
	searchActive := p.searchQuery.Term != "" &&
		p.searchQuery.Scope != query.ScopeSession &&
		len(p.queryResult.Sessions) > 0
	dimStyle := lipgloss.NewStyle().Foreground(activeTheme.ColorFgDim)

	// Pre-compute which panes and windows have at least one matching process so
	// that ancestor header rows are not dimmed when a descendant matches.
	paneHasMatch := map[string]bool{}
	windowHasMatch := map[int]bool{}
	if searchActive {
		for _, node := range p.nodes {
			if node.IsPaneHeader || node.IsWindowHeader || node.IsIdle {
				continue
			}
			if p.procMatchPos(p.curSession, node.Proc.PID) != nil {
				paneHasMatch[node.Pane.PaneID] = true
				windowHasMatch[node.Pane.WindowIndex] = true
			}
		}
	}

	var allLines []renderedLine
	for i, node := range p.nodes {
		selected := focused && i == p.cursor
		var line string
		if node.IsWindowHeader {
			line = p.renderWindowHeader(node, selected, innerW)
			if searchActive && !selected {
				pos := p.windowMatchPos(p.curSession, node.Pane.WindowIndex)
				if pos != nil {
					highlighted := highlightMatchPos(node.Pane.WindowName, pos)
					line = strings.Replace(line, node.Pane.WindowName, highlighted, 1)
				} else if !windowHasMatch[node.Pane.WindowIndex] {
					line = dimStyle.Render(stripANSI(line))
				}
			}
		} else if node.IsPaneHeader {
			paneInnerW := innerW
			if p.inSessionMode {
				paneInnerW -= 4
				if paneInnerW < 0 {
					paneInnerW = 0
				}
			}
			hasIdle := i+1 < len(p.nodes) && p.nodes[i+1].IsIdle
			rendered := p.renderPaneHeader(node, selected, paneInnerW, hasIdle)
			if hasIdle && !selected {
				rendered += "  " + paneIdleStyle.Render("idle")
			}
			if p.inSessionMode {
				rendered = "    " + rendered
			}
			line = rendered
			if searchActive && !selected && !paneHasMatch[node.Pane.PaneID] {
				line = dimStyle.Render(stripANSI(line))
			}
		} else if node.IsIdle {
			continue
		} else {
			procInnerW := innerW
			if p.inSessionMode {
				procInnerW = innerW - 4
				if procInnerW < 0 {
					procInnerW = 0
				}
			}
			rendered := p.renderProc(node, selected, procInnerW)
			if p.inSessionMode {
				parts := strings.SplitN(rendered, "\n", 2)
				if len(parts) == 2 {
					rendered = "    " + parts[0] + "\n" + "    " + parts[1]
				} else {
					rendered = "    " + rendered
				}
			}
			line = rendered
			if searchActive && !selected {
				pos := p.procMatchPos(p.curSession, node.Proc.PID)
				if pos != nil {
					highlighted := highlightMatchPos(node.Proc.FriendlyName(), pos)
					line = strings.Replace(line, node.Proc.FriendlyName(), highlighted, 1)
				} else {
					line = dimStyle.Render(stripANSI(line))
				}
			}
		}
		allLines = append(allLines, renderedLine{nodeIdx: i, text: line})
	}

	maxRows := height - 2
	if maxRows < 1 {
		maxRows = 1
	}

	// Safety clamps (read-only): handle cases where the viewport shrank since
	// the last clampOffset call (e.g. detail pane expanding after selection change).
	offset := p.offset
	if p.cursor < offset {
		offset = p.cursor
	}
	for offset < p.cursor && !procCursorVisible(p.nodes, p.cursor, offset, maxRows) {
		offset++
	}

	visible, hasAbove, hasBelow := computeViewport(allLines, p.cursor, offset, maxRows)

	var resultLines []string
	if hasAbove {
		resultLines = append(resultLines, hintStyle.Render("▲ more"))
	}
	resultLines = append(resultLines, visible...)
	if hasBelow {
		resultLines = append(resultLines, hintStyle.Render("▼ more"))
	}

	inner := strings.Join(resultLines, "\n")
	return injectBorderTitles(border.Width(width-2).Height(height-2).Render(inner), title, rightTitle)
}

func (p ProcListModel) renderPaneHeader(node ProcListNode, selected bool, innerW int, hasIdle bool) string {
	label := fmt.Sprintf("pane %d", node.Pane.PaneIndex)

	alertSuffix := ""
	if node.Alert != nil {
		alertSuffix = "  " + paneSepStyle.Render("────") + "  " + alertIcon(node.Alert.Level) + " " + alertBadge(node.Alert.Level, node.Alert.Reason)
	}

	pathStr := ""
	if node.Pane.CWD != "" && node.Pane.CWD != p.primaryCWD {
		pathStr = format.ShortenPath(node.Pane.CWD, p.cfg.PathAliases)
	}
	gitSuffix := ""
	if node.GitDeviant {
		if node.GitInfo.Loading {
			gitSuffix = "  ↪ …"
		} else {
			gitSuffix = "  ↪ " + stripANSI(compactGitIndicators(node.GitInfo))
		}
	}

	if selected {
		left := label + stripANSI(alertSuffix)
		rightPart := pathStr + gitSuffix
		if rightPart != "" && p.cfg.ProcessList.PathRightAlign && innerW > 0 {
			rightW := len([]rune(rightPart))
			padCount := innerW - len([]rune(left)) - rightW
			if padCount < 1 {
				padCount = 1
			}
			return selectedBG.Render(left + strings.Repeat(" ", padCount) + rightPart)
		}
		if rightPart != "" {
			content := left + "  " + rightPart
			padCount := innerW - len([]rune(content))
			if padCount < 0 {
				padCount = 0
			}
			return selectedBG.Render(content + strings.Repeat(" ", padCount))
		}
		if hasIdle {
			// Render "label  idle" inline, then fill the remaining width with
			// the selected background so the row doesn't wrap.
			const idleVisualW = 6 // len("  idle")
			padCount := innerW - len([]rune(left)) - idleVisualW
			if padCount < 0 {
				padCount = 0
			}
			idleRendered := paneIdleStyle.Background(activeTheme.ColorSelected).Render("idle")
			return selectedBG.Render(left) +
				selectedBG.Render("  ") +
				idleRendered +
				selectedBG.Render(strings.Repeat(" ", padCount))
		}
		padCount := innerW - len([]rune(left))
		if padCount < 0 {
			padCount = 0
		}
		return selectedBG.Render(left + strings.Repeat(" ", padCount))
	}

	rightPart := pathStr + gitSuffix
	if rightPart == "" || !p.cfg.ProcessList.PathRightAlign || innerW <= 0 {
		out := paneHeaderStyle.Render(label) + alertSuffix
		if pathStr != "" {
			out += "  " + panePathStyle.Render(pathStr)
		}
		if node.GitDeviant {
			if node.GitInfo.Loading {
				out += "  " + panePathStyle.Render("↪ …")
			} else {
				out += "  " + panePathStyle.Render("↪") + " " + compactGitIndicators(node.GitInfo)
			}
		}
		return out
	}

	labelW := len([]rune(label + stripANSI(alertSuffix)))
	rightW := len([]rune(rightPart))
	fillCount := innerW - labelW - 2 - 2 - rightW
	if fillCount < 1 {
		fillCount = 1
	}
	out := paneHeaderStyle.Render(label) +
		alertSuffix +
		"  " +
		paneSepStyle.Render(strings.Repeat("─", fillCount)) +
		"  " +
		panePathStyle.Render(pathStr)
	if node.GitDeviant {
		if node.GitInfo.Loading {
			out += "  " + panePathStyle.Render("↪ …")
		} else {
			out += "  " + panePathStyle.Render("↪") + " " + compactGitIndicators(node.GitInfo)
		}
	}
	return out
}

func (p ProcListModel) renderWindowHeader(node ProcListNode, selected bool, innerW int) string {
	label := fmt.Sprintf("Win %d", node.Pane.WindowIndex)
	if node.Pane.WindowName != "" {
		label = fmt.Sprintf("Win %d: %s", node.Pane.WindowIndex, node.Pane.WindowName)
	}

	alertSuffix := ""
	if node.Alert != nil {
		alertSuffix = "  " + paneSepStyle.Render("────") + "  " + alertIcon(node.Alert.Level) + " " + alertBadge(node.Alert.Level, node.Alert.Reason)
	}

	pathStr := ""
	if node.Pane.CWD != "" && node.Pane.CWD != p.primaryCWD {
		pathStr = format.ShortenPath(node.Pane.CWD, p.cfg.PathAliases)
	}

	if selected {
		left := label + stripANSI(alertSuffix)
		if pathStr != "" && innerW > 0 {
			rightW := len([]rune(pathStr))
			padCount := innerW - len([]rune(left)) - rightW
			if padCount < 1 {
				padCount = 1
			}
			return selectedBG.Render(left + strings.Repeat(" ", padCount) + pathStr)
		}
		if pathStr != "" {
			content := left + "  " + pathStr
			padCount := innerW - len([]rune(content))
			if padCount < 0 {
				padCount = 0
			}
			return selectedBG.Render(content + strings.Repeat(" ", padCount))
		}
		padCount := innerW - len([]rune(left))
		if padCount < 0 {
			padCount = 0
		}
		return selectedBG.Render(left + strings.Repeat(" ", padCount))
	}

	if pathStr == "" || innerW <= 0 {
		out := windowHeaderStyle.Render(label) + alertSuffix
		if pathStr != "" {
			out += "  " + panePathStyle.Render(pathStr)
		}
		return out
	}

	labelW := len([]rune(label + stripANSI(alertSuffix)))
	rightW := len([]rune(pathStr))
	fillCount := innerW - labelW - 2 - 2 - rightW
	if fillCount < 1 {
		fillCount = 1
	}
	return windowHeaderStyle.Render(label) +
		alertSuffix +
		"  " +
		paneSepStyle.Render(strings.Repeat("─", fillCount)) +
		"  " +
		panePathStyle.Render(pathStr)
}

// procNameStyle returns the appropriate lipgloss style for a process name
// based on its type and tree depth.
func procNameStyle(pr proc.Process, depth int) lipgloss.Style {
	if depth >= 2 {
		return lipgloss.NewStyle().Foreground(activeTheme.ColorProcChild)
	}
	name := strings.ToLower(pr.FriendlyName())
	switch {
	case containsStr(activeProcEditors, name):
		return lipgloss.NewStyle().Foreground(activeTheme.ColorProcEditor)
	case containsStr(activeProcAgents, name) || strings.HasPrefix(name, "claude-"):
		return lipgloss.NewStyle().Foreground(activeTheme.ColorProcClaude)
	case containsStr(activeProcServers, name):
		return lipgloss.NewStyle().Foreground(activeTheme.ColorProcServer)
	default:
		return lipgloss.NewStyle().Foreground(activeTheme.ColorFgPrimary)
	}
}

func (p ProcListModel) renderProc(node ProcListNode, selected bool, innerW int) string {
	pr := node.Proc
	indent := node.TreePrefix

	// collapse indicator prefix for depth-1 nodes with children
	collapsePrefix := ""
	if node.Depth == 1 && node.HasChildren {
		if node.Collapsed {
			collapsePrefix = "▶ "
		} else {
			collapsePrefix = "▼ "
		}
	}

	// line 1: [indicator]name  pid:N  :port
	var line1 string
	if selected {
		plain := indent + collapsePrefix + pr.FriendlyName()
		if pr.PID > 0 {
			plain += fmt.Sprintf("  pid:%d", pr.PID)
		}
		if node.Port > 0 {
			plain += fmt.Sprintf("  :%d", node.Port)
		}
		padCount := innerW - len([]rune(plain))
		if padCount < 0 {
			padCount = 0
		}
		line1 = selectedBG.Render(plain + strings.Repeat(" ", padCount))
	} else {
		line1 = treeConnectorStyle.Render(indent) + procNameStyle(pr, node.Depth).Render(collapsePrefix+pr.FriendlyName())
		if pr.PID > 0 {
			line1 += "  " + statLabelStyle.Render(fmt.Sprintf("pid:%d", pr.PID))
		}
		if node.Port > 0 {
			line1 += "  " + statValueStyle.Render(fmt.Sprintf(":%d", node.Port))
		}
	}

	// line 2: cpu/mem stats; show aggregated totals in parens when collapsed with children
	statsIndent := treeConnectorStyle.Render(node.StatPrefix) + "  "
	l := statLabelStyle.Render
	v := statValueStyle.Render

	cpuStr := v(fmt.Sprintf("%.1f%%", pr.CPU))
	memStr := v(fmt.Sprintf("%.1fMB", float64(pr.MemRSS)/1024/1024))
	if node.Depth == 1 && node.HasChildren && node.Collapsed {
		cpuStr += v(fmt.Sprintf(" (%.1f%%)", node.AggCPU))
		memStr += v(fmt.Sprintf(" (%.1fMB)", float64(node.AggMemRSS)/1024/1024))
	}

	line2 := statsIndent +
		l("cpu:") + cpuStr + "  " +
		l("mem:") + memStr + "  " +
		l("up:") + v(formatProcDuration(pr.Uptime))

	return line1 + "\n" + line2
}

func formatProcDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	switch {
	case h >= 24:
		return fmt.Sprintf("%dd%dh", h/24, h%24)
	case h > 0:
		return fmt.Sprintf("%dh%dm", h, m)
	case m > 0:
		return fmt.Sprintf("%dm", m)
	default:
		return fmt.Sprintf("%ds", s)
	}
}

func stripANSI(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && s[i] != 'm' {
				i++
			}
			i++ // skip 'm'
			continue
		}
		result.WriteByte(s[i])
		i++
	}
	return result.String()
}

// injectBorderTitle splices title into the top border line of a lipgloss-rendered
// box. The title is placed immediately after the top-left corner, with the
// remaining width filled by the border's horizontal fill character.
// ANSI color codes wrapping the original top line are preserved.
func injectBorderTitle(rendered, title string) string {
	if title == "" {
		return rendered
	}
	nl := strings.IndexByte(rendered, '\n')
	if nl < 0 {
		return rendered
	}
	topLine := rendered[:nl]
	rest := rendered[nl:] // includes the leading \n

	plain := stripANSI(topLine)
	runes := []rune(plain)
	if len(runes) < 4 {
		return rendered
	}

	// runes[0]=╭, runes[1..len-2]=─ fill, runes[len-1]=╮
	cornerLeft := string(runes[0])
	cornerRight := string(runes[len(runes)-1])
	fill := string(runes[1])
	totalInner := len(runes) - 2

	titleRunes := []rune(title)
	titleVisible := []rune(stripANSI(title))
	if len(titleVisible) > totalInner-1 {
		titleRunes = titleRunes[:totalInner-1]
		titleVisible = titleVisible[:totalInner-1]
	}
	fillCount := totalInner - len(titleVisible)

	// Extract ANSI prefix (border color) and suffix (reset) from the original top line.
	cornerLeftIdx := strings.Index(topLine, cornerLeft)
	cornerRightIdx := strings.LastIndex(topLine, cornerRight)
	ansiPrefix := ""
	ansiSuffix := ""
	if cornerLeftIdx > 0 {
		ansiPrefix = topLine[:cornerLeftIdx]
	}
	if cornerRightIdx >= 0 {
		after := cornerRightIdx + len(cornerRight)
		if after <= len(topLine) {
			ansiSuffix = topLine[after:]
		}
	}

	// Color the border chars but not the title text, so the title uses the
	// terminal's default foreground rather than the border color.
	newTop := ansiPrefix + cornerLeft + ansiSuffix +
		title +
		ansiPrefix + strings.Repeat(fill, fillCount) + cornerRight + ansiSuffix

	return newTop + rest
}

// injectBorderTitles is like injectBorderTitle but also places rightTitle
// flush against the right corner of the top border line.
func injectBorderTitles(rendered, leftTitle, rightTitle string) string {
	if rightTitle == "" {
		return injectBorderTitle(rendered, leftTitle)
	}
	nl := strings.IndexByte(rendered, '\n')
	if nl < 0 {
		return rendered
	}
	topLine := rendered[:nl]
	rest := rendered[nl:]

	plain := stripANSI(topLine)
	runes := []rune(plain)
	if len(runes) < 4 {
		return rendered
	}

	cornerLeft := string(runes[0])
	cornerRight := string(runes[len(runes)-1])
	fill := string(runes[1])
	totalInner := len(runes) - 2

	leftVisible := []rune(stripANSI(leftTitle))
	rightVisible := []rune(stripANSI(rightTitle))
	totalUsed := len(leftVisible) + len(rightVisible)
	if totalUsed >= totalInner {
		return injectBorderTitle(rendered, leftTitle)
	}
	fillCount := totalInner - totalUsed

	cornerLeftIdx := strings.Index(topLine, cornerLeft)
	cornerRightIdx := strings.LastIndex(topLine, cornerRight)
	ansiPrefix := ""
	ansiSuffix := ""
	if cornerLeftIdx > 0 {
		ansiPrefix = topLine[:cornerLeftIdx]
	}
	if cornerRightIdx >= 0 {
		after := cornerRightIdx + len(cornerRight)
		if after <= len(topLine) {
			ansiSuffix = topLine[after:]
		}
	}

	newTop := ansiPrefix + cornerLeft + ansiSuffix +
		leftTitle +
		ansiPrefix + strings.Repeat(fill, fillCount) + ansiSuffix +
		rightTitle +
		ansiPrefix + cornerRight + ansiSuffix

	return newTop + rest
}

// highlightMatchPos returns the name with matched character positions rendered
// in the accent color. If pos is empty the name is returned unchanged.
func highlightMatchPos(name string, pos []int) string {
	if len(pos) == 0 {
		return name
	}
	posSet := make(map[int]bool, len(pos))
	for _, p := range pos {
		posSet[p] = true
	}
	accentStyle := lipgloss.NewStyle().Foreground(activeTheme.ColorFgSearchHighlight)
	var b strings.Builder
	for i, ch := range name {
		if posSet[i] {
			b.WriteString(accentStyle.Render(string(ch)))
		} else {
			b.WriteRune(ch)
		}
	}
	return b.String()
}

// injectBottomBorderLabel splices label into the bottom border line of a
// lipgloss-rendered box, centered. ANSI color codes are preserved.
func injectBottomBorderLabel(rendered, label string) string {
	if label == "" {
		return rendered
	}
	// Determine the bottom border line.
	// Lipgloss may or may not append a trailing newline; handle both.
	var bottomLine, prefix, trailing string
	lastNL := strings.LastIndexByte(rendered, '\n')
	if lastNL < 0 {
		// No newlines at all: entire string is the bottom border.
		bottomLine = rendered
		prefix = ""
		trailing = ""
	} else if lastNL == len(rendered)-1 {
		// String ends with \n: bottom border is between prevNL and lastNL.
		prevNL := strings.LastIndexByte(rendered[:lastNL], '\n')
		trailing = rendered[lastNL:] // the trailing \n
		if prevNL < 0 {
			prefix = ""
			bottomLine = rendered[:lastNL]
		} else {
			prefix = rendered[:prevNL+1]
			bottomLine = rendered[prevNL+1 : lastNL]
		}
	} else {
		// String does NOT end with \n: bottom border is rendered[lastNL+1:].
		prefix = rendered[:lastNL+1]
		bottomLine = rendered[lastNL+1:]
		trailing = ""
	}

	plain := stripANSI(bottomLine)
	runes := []rune(plain)
	if len(runes) < 4 {
		return rendered
	}

	totalInner := len(runes) - 2
	labelVisible := []rune(stripANSI(label))
	if len(labelVisible) > totalInner {
		return rendered
	}

	fill := string(runes[1])
	cornerLeft := string(runes[0])
	cornerRight := string(runes[len(runes)-1])

	leftPad := (totalInner - len(labelVisible)) / 2
	rightPad := totalInner - len(labelVisible) - leftPad

	// Preserve ANSI prefix/suffix from original bottom line.
	cornerLeftIdx := strings.Index(bottomLine, cornerLeft)
	cornerRightIdx := strings.LastIndex(bottomLine, cornerRight)
	ansiPrefix, ansiSuffix := "", ""
	if cornerLeftIdx > 0 {
		ansiPrefix = bottomLine[:cornerLeftIdx]
	}
	if cornerRightIdx >= 0 {
		after := cornerRightIdx + len(cornerRight)
		if after <= len(bottomLine) {
			ansiSuffix = bottomLine[after:]
		}
	}

	newBottom := ansiPrefix + cornerLeft + ansiSuffix +
		strings.Repeat(fill, leftPad) +
		label +
		strings.Repeat(fill, rightPad) +
		ansiPrefix + cornerRight + ansiSuffix

	return prefix + newBottom + trailing
}
