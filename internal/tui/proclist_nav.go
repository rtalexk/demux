package tui

import "fmt"

// cursorNodeKey returns a stable string identity for the node at the cursor so
// that applyPendingSeek can restore focus after a rebuild changes indices.
//
// Key format:
//   - window header → "win:<windowIndex>"
//   - pane header   → "pane:<windowIndex>:<paneIndex>"
//   - idle marker   → "idle:<windowIndex>:<paneIndex>"  (uses containing pane)
//   - process       → "pid:<PID>"  (for depth>1 uses the depth-1 ancestor's PID)
func (p *ProcListModel) cursorNodeKey() string {
	if p.cursor < 0 || p.cursor >= len(p.nodes) {
		return ""
	}
	n := p.nodes[p.cursor]
	switch {
	case n.IsWindowHeader:
		return fmt.Sprintf("win:%d", n.Pane.WindowIndex)
	case n.IsPaneHeader:
		return fmt.Sprintf("pane:%d:%d", n.Pane.WindowIndex, n.Pane.PaneIndex)
	case n.IsIdle:
		for i := p.cursor - 1; i >= 0; i-- {
			if p.nodes[i].IsPaneHeader {
				ph := p.nodes[i]
				return fmt.Sprintf("idle:%d:%d", ph.Pane.WindowIndex, ph.Pane.PaneIndex)
			}
		}
		return ""
	case n.Depth > 1:
		for i := p.cursor - 1; i >= 0; i-- {
			if p.nodes[i].Depth == 1 {
				return fmt.Sprintf("pid:%d", p.nodes[i].Proc.PID)
			}
		}
		return ""
	default:
		return fmt.Sprintf("pid:%d", n.Proc.PID)
	}
}

// applyPendingSeek moves the cursor to the node matching pendingSeekKey, then
// clears the field. Called at the end of each SetWindowData / SetSessionData.
func (p *ProcListModel) applyPendingSeek() {
	if p.pendingSeekKey == "" {
		return
	}
	key := p.pendingSeekKey
	p.pendingSeekKey = ""
	// Track the most recent pane header for idle-node matching.
	var curPaneWin, curPanePane int
	for i, n := range p.nodes {
		if n.IsPaneHeader {
			curPaneWin = n.Pane.WindowIndex
			curPanePane = n.Pane.PaneIndex
		}
		var nkey string
		switch {
		case n.IsWindowHeader:
			nkey = fmt.Sprintf("win:%d", n.Pane.WindowIndex)
		case n.IsPaneHeader:
			nkey = fmt.Sprintf("pane:%d:%d", n.Pane.WindowIndex, n.Pane.PaneIndex)
		case n.IsIdle:
			nkey = fmt.Sprintf("idle:%d:%d", curPaneWin, curPanePane)
		case n.Proc.PID != 0:
			nkey = fmt.Sprintf("pid:%d", n.Proc.PID)
		}
		if nkey != "" && nkey == key {
			p.cursor = i
			return
		}
	}
}

// clampOffset adjusts p.offset so the cursor node is always within the
// visible row window. maxRows is the total inner height of the proc panel
// (border already subtracted).
func (p *ProcListModel) clampOffset(maxRows int) {
	if len(p.nodes) == 0 {
		p.offset = 0
		p.cursor = 0
		return
	}
	if p.cursor >= len(p.nodes) {
		p.cursor = len(p.nodes) - 1
	}
	if p.cursor < p.offset {
		p.offset = p.cursor
	}
	for p.offset < p.cursor {
		if procCursorVisible(p.nodes, p.cursor, p.offset, maxRows) {
			break
		}
		p.offset++
	}
	if p.offset < 0 {
		p.offset = 0
	}
}

// procCursorVisible mirrors the Render hint logic to determine whether the
// cursor node would be visible with the given offset and maxRows.
//   - ▲ hint is shown when offset > 0 (costs 1 row)
//   - ▼ hint is shown when content overflows after accounting for ▲ (costs 1 row)
func procCursorVisible(nodes []ProcListNode, cursor, offset, maxRows int) bool {
	hasAbove := offset > 0
	contentRows := maxRows
	if hasAbove {
		contentRows--
	}

	// First pass: scan ALL nodes from offset (not stopping at cursor) so we
	// can detect whether nodes after the cursor would cause ▼ to appear.
	rows := 0
	cursorRows := -1 // rows consumed up to and including the cursor node
	hasBelow := false
	for i := offset; i < len(nodes); i++ {
		nr := nodeRows(nodes[i])
		if rows+nr > contentRows {
			hasBelow = true
			break
		}
		rows += nr
		if i == cursor {
			cursorRows = rows
		}
	}
	if cursorRows < 0 {
		return false // cursor not reached within contentRows
	}
	if !hasBelow {
		return true // no ▼ hint; cursor fits
	}
	// ▼ hint will be shown — re-check with one fewer row.
	rows = 0
	for i := offset; i < len(nodes); i++ {
		nr := nodeRows(nodes[i])
		if rows+nr > contentRows-1 {
			return false
		}
		rows += nr
		if i == cursor {
			return true
		}
	}
	return false
}

// MoveUp moves the cursor one item up, skipping idle placeholders.
func (p *ProcListModel) MoveUp() {
	for i := p.cursor - 1; i >= 0; i-- {
		if isSelectable(p.nodes[i]) {
			p.cursor = i
			return
		}
	}
}

// MoveDown moves the cursor one item down, skipping idle placeholders.
func (p *ProcListModel) MoveDown() {
	for i := p.cursor + 1; i < len(p.nodes); i++ {
		if isSelectable(p.nodes[i]) {
			p.cursor = i
			return
		}
	}
}

// TabNext moves to the next sibling at the same depth level, wrapping around
// within the current depth's peer set.
func (p *ProcListModel) TabNext() {
	if len(p.nodes) == 0 {
		return
	}
	depth := nodeDepth(p.nodes[p.cursor])

	// collect all sibling indices at the same depth within the same scope
	peers := p.peersAtDepth(p.cursor, depth)
	if len(peers) == 0 {
		return
	}

	// find current position among peers and advance (with wrap)
	for i, idx := range peers {
		if idx == p.cursor {
			p.cursor = peers[(i+1)%len(peers)]
			return
		}
	}
}

// peersAtDepth returns the indices of all nodes that are siblings of the node
// at pos within the same scope (pane for depth 1; parent process block for depth 2;
// all pane headers for depth 0).
func (p *ProcListModel) peersAtDepth(pos, depth int) []int {
	if depth == 0 {
		var peers []int
		for i, n := range p.nodes {
			if n.IsPaneHeader || n.IsWindowHeader {
				peers = append(peers, i)
			}
		}
		return peers
	}

	// find the scope boundaries for depth 1 or 2
	scopeStart, scopeEnd := 0, len(p.nodes)-1

	if depth == 1 {
		// scope is within the enclosing pane header section
		for i := pos - 1; i >= 0; i-- {
			if p.nodes[i].IsPaneHeader {
				scopeStart = i + 1
				break
			}
		}
		for i := pos + 1; i < len(p.nodes); i++ {
			if p.nodes[i].IsPaneHeader {
				scopeEnd = i - 1
				break
			}
		}
	} else {
		// depth == 2: scope is within the enclosing depth-1 parent process block
		for i := pos - 1; i >= 0; i-- {
			if p.nodes[i].IsPaneHeader || nodeDepth(p.nodes[i]) == 1 {
				scopeStart = i + 1
				break
			}
		}
		for i := pos + 1; i < len(p.nodes); i++ {
			if p.nodes[i].IsPaneHeader || nodeDepth(p.nodes[i]) == 1 {
				scopeEnd = i - 1
				break
			}
		}
	}

	var peers []int
	for i := scopeStart; i <= scopeEnd; i++ {
		if nodeDepth(p.nodes[i]) == depth && isSelectable(p.nodes[i]) {
			peers = append(peers, i)
		}
	}
	return peers
}

func (p *ProcListModel) JumpToNextPane() {
	for i := p.cursor + 1; i < len(p.nodes); i++ {
		if p.nodes[i].IsPaneHeader || p.nodes[i].IsWindowHeader {
			p.cursor = i
			return
		}
	}
}

func (p *ProcListModel) JumpToPrevPane() {
	for i := p.cursor - 1; i >= 0; i-- {
		if p.nodes[i].IsPaneHeader || p.nodes[i].IsWindowHeader {
			p.cursor = i
			return
		}
	}
}

func (p *ProcListModel) GotoTop() {
	p.cursor = 0
	p.offset = 0
}

func (p *ProcListModel) GotoBottom() {
	for i := len(p.nodes) - 1; i >= 0; i-- {
		if isSelectable(p.nodes[i]) {
			p.cursor = i
			return
		}
	}
}

// ToggleCollapse flips the collapsed state of the cursor node if it is a
// depth-1 process with children. Returns true if a toggle occurred.
// The caller must re-call SetWindowData to rebuild nodes after a toggle.
func (p *ProcListModel) ToggleCollapse() bool {
	if p.cursor < 0 || p.cursor >= len(p.nodes) {
		return false
	}
	n := p.nodes[p.cursor]
	if n.IsPaneHeader || n.IsIdle || n.Depth != 1 || !n.HasChildren {
		return false
	}
	if p.collapsedPIDs == nil {
		p.collapsedPIDs = make(map[int32]bool)
	}
	p.collapsedPIDs[n.Proc.PID] = !p.collapsedPIDs[n.Proc.PID]
	return true
}

// Expand expands the focused depth-1 process node if it has children and is collapsed.
// Returns true if a change occurred; the caller must re-call SetWindowData.
func (p *ProcListModel) Expand() bool {
	if p.cursor < 0 || p.cursor >= len(p.nodes) {
		return false
	}
	n := p.nodes[p.cursor]
	if n.IsPaneHeader || n.IsIdle || n.Depth != 1 || !n.HasChildren {
		return false
	}
	if p.collapsedPIDs == nil {
		p.collapsedPIDs = make(map[int32]bool)
	}
	if !p.collapsedPIDs[n.Proc.PID] {
		return false
	}
	p.collapsedPIDs[n.Proc.PID] = false
	return true
}

// Collapse collapses the focused node. For depth-1 nodes with children it collapses
// directly; for nodes at any deeper depth it walks up to the ancestor depth-1 node, moves
// the cursor there, and collapses it. Returns true if a change occurred; the caller must re-call SetWindowData.
func (p *ProcListModel) Collapse() bool {
	if p.cursor < 0 || p.cursor >= len(p.nodes) {
		return false
	}
	n := p.nodes[p.cursor]
	if n.IsPaneHeader || n.IsIdle {
		return false
	}
	if p.collapsedPIDs == nil {
		p.collapsedPIDs = make(map[int32]bool)
	}
	if n.Depth > 1 {
		// Walk up to the parent depth-1 node.
		for i := p.cursor - 1; i >= 0; i-- {
			if p.nodes[i].IsPaneHeader {
				break
			}
			if p.nodes[i].Depth == 1 {
				parent := p.nodes[i]
				if !parent.HasChildren || p.collapsedPIDs[parent.Proc.PID] {
					return false
				}
				p.cursor = i
				p.collapsedPIDs[parent.Proc.PID] = true
				return true
			}
		}
		return false
	}
	if n.Depth != 1 || !n.HasChildren {
		return false
	}
	if p.collapsedPIDs[n.Proc.PID] {
		return false
	}
	p.collapsedPIDs[n.Proc.PID] = true
	return true
}

// ExpandAll expands all depth-1 process nodes that have children.
// Returns true if any change occurred; the caller must re-call SetWindowData.
// pendingSeekKey is set so that applyPendingSeek can restore focus after
// the rebuild changes indices.
func (p *ProcListModel) ExpandAll() bool {
	if p.collapsedPIDs == nil {
		p.collapsedPIDs = make(map[int32]bool)
	}
	p.pendingSeekKey = p.cursorNodeKey()
	changed := false
	for _, n := range p.nodes {
		if n.IsPaneHeader || n.IsIdle || n.Depth != 1 || !n.HasChildren {
			continue
		}
		if p.collapsedPIDs[n.Proc.PID] {
			p.collapsedPIDs[n.Proc.PID] = false
			changed = true
		}
	}
	return changed
}

// CollapseAll collapses all depth-1 process nodes that have children.
// Returns true if any change occurred; the caller must re-call SetWindowData.
// pendingSeekKey is set so that applyPendingSeek (called by SetWindowData) can
// restore focus to the same logical node after the rebuild changes indices.
func (p *ProcListModel) CollapseAll() bool {
	if p.collapsedPIDs == nil {
		p.collapsedPIDs = make(map[int32]bool)
	}
	p.pendingSeekKey = p.cursorNodeKey()
	changed := false
	for _, n := range p.nodes {
		if n.IsPaneHeader || n.IsIdle || n.Depth != 1 || !n.HasChildren {
			continue
		}
		if !p.collapsedPIDs[n.Proc.PID] {
			p.collapsedPIDs[n.Proc.PID] = true
			changed = true
		}
	}
	return changed
}
