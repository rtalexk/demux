package sticky

import (
	"errors"
	"strings"
	"testing"
)

// 2-pane horizontal layout: nvim (left, 80 cols) + claude (right, 80 cols).
// After the sidebar split tmux's default victim is the rightmost pane
// (claude). We expect a resize-pane -t claude -x 80 so the width comes from
// the leftmost-adjacent column (nvim) instead, keeping claude stable across
// repeated open/close cycles.
func TestSplitSidebarPane_RebalancesAdjacentAbsorbsWidth(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -t $1:@7 -F #{pane_id} #{pane_left} #{pane_width}"] = fakeReply{
		out: "%nvim 0 80\n%claude 80 80\n",
	}
	f.outputs["split-window -f -h -b -d -l 35 -t $1:@7 -P -F #{pane_id} demux --sticky"] = fakeReply{out: "%sidebar\n"}

	s := &Sticky{T: f}
	pane, err := s.splitSidebarPane("$1:@7", 35, "demux --sticky")
	if err != nil {
		t.Fatalf("splitSidebarPane: %v", err)
	}
	if pane != "%sidebar" {
		t.Errorf("want pane %%sidebar, got %q", pane)
	}

	wantResize := "resize-pane -t %claude -x 80"
	var sawResize bool
	var sawNvimResize bool
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if j == wantResize {
			sawResize = true
		}
		if j == "resize-pane -t %nvim -x 80" {
			sawNvimResize = true
		}
	}
	if !sawResize {
		t.Errorf("expected %q so the rightmost pane keeps its width and the leftmost absorbs the sidebar's footprint, got runs: %v", wantResize, f.runs)
	}
	if sawNvimResize {
		t.Errorf("must not resize the leftmost original pane back to its width - it has to absorb the sidebar's footprint; got: %v", f.runs)
	}
}

// 3-pane horizontal layout (nvim, code, claude). Resizes must run
// rightmost-first so the deficit propagates leftward and lands on nvim.
func TestSplitSidebarPane_ThreePanes_RestoresRightmostFirst(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -t @9 -F #{pane_id} #{pane_left} #{pane_width}"] = fakeReply{
		out: "%nvim 0 80\n%code 80 80\n%claude 160 80\n",
	}
	f.outputs["split-window -f -h -b -d -l 35 -t @9 -P -F #{pane_id} demux sidebar slot"] = fakeReply{out: "%sidebar\n"}

	s := &Sticky{T: f}
	if _, err := s.splitSidebarPane("@9", 35, SlotPlaceholderCmd); err != nil {
		t.Fatalf("splitSidebarPane: %v", err)
	}

	var resizeOrder []string
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if strings.HasPrefix(j, "resize-pane -t %") {
			resizeOrder = append(resizeOrder, j)
		}
	}
	want := []string{
		"resize-pane -t %claude -x 80",
		"resize-pane -t %code -x 80",
	}
	if len(resizeOrder) != len(want) {
		t.Fatalf("want %d resize calls, got %d: %v", len(want), len(resizeOrder), resizeOrder)
	}
	for i := range want {
		if resizeOrder[i] != want[i] {
			t.Errorf("resize order[%d]: want %q, got %q (full: %v)", i, want[i], resizeOrder[i], resizeOrder)
		}
	}
}

// Vertically-stacked panes in the leftmost column (nvim top, log bottom,
// claude right). Both leftmost-column panes share pane_left=0 and must be
// skipped from the rebalance so the column absorbs the sidebar's width.
func TestSplitSidebarPane_VerticalStackInLeftColumn_SkipsAllLeftmost(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -t @3 -F #{pane_id} #{pane_left} #{pane_width}"] = fakeReply{
		out: "%nvim 0 80\n%log 0 80\n%claude 80 80\n",
	}
	f.outputs["split-window -f -h -b -d -l 35 -t @3 -P -F #{pane_id} demux --sticky"] = fakeReply{out: "%sidebar\n"}

	s := &Sticky{T: f}
	if _, err := s.splitSidebarPane("@3", 35, "demux --sticky"); err != nil {
		t.Fatalf("splitSidebarPane: %v", err)
	}

	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if j == "resize-pane -t %nvim -x 80" || j == "resize-pane -t %log -x 80" {
			t.Errorf("must not resize panes in the leftmost column - the column has to absorb the sidebar's width; got: %v", r)
		}
	}
	var sawClaude bool
	for _, r := range f.runs {
		if strings.Join(r, " ") == "resize-pane -t %claude -x 80" {
			sawClaude = true
		}
	}
	if !sawClaude {
		t.Errorf("expected claude to be restored to its original width; got: %v", f.runs)
	}
}

// Geometry-read failures must not block the split. The split itself still
// runs and returns the new pane id; rebalance is silently skipped.
func TestSplitSidebarPane_GeometryReadFails_StillSplits(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -t $1:@7 -F #{pane_id} #{pane_left} #{pane_width}"] = fakeReply{err: errors.New("tmux exploded")}
	f.outputs["split-window -f -h -b -d -l 35 -t $1:@7 -P -F #{pane_id} demux --sticky"] = fakeReply{out: "%88\n"}

	s := &Sticky{T: f}
	pane, err := s.splitSidebarPane("$1:@7", 35, "demux --sticky")
	if err != nil {
		t.Fatalf("splitSidebarPane: %v", err)
	}
	if pane != "%88" {
		t.Errorf("want %%88, got %q", pane)
	}
	for _, r := range f.runs {
		if strings.HasPrefix(strings.Join(r, " "), "resize-pane") {
			t.Errorf("no rebalance possible without geometry, but got resize call: %v", r)
		}
	}
}

// Single-pane window has no neighbours to rebalance against. Split should run
// and emit zero resize-pane calls (the lone pane is the leftmost and is
// always skipped).
func TestSplitSidebarPane_SinglePane_NoResize(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -t $1:@7 -F #{pane_id} #{pane_left} #{pane_width}"] = fakeReply{out: "%only 0 160\n"}
	f.outputs["split-window -f -h -b -d -l 35 -t $1:@7 -P -F #{pane_id} demux --sticky"] = fakeReply{out: "%new\n"}

	s := &Sticky{T: f}
	if _, err := s.splitSidebarPane("$1:@7", 35, "demux --sticky"); err != nil {
		t.Fatalf("splitSidebarPane: %v", err)
	}
	for _, r := range f.runs {
		if strings.HasPrefix(strings.Join(r, " "), "resize-pane") {
			t.Errorf("single-pane window has nothing to rebalance, got: %v", r)
		}
	}
}
