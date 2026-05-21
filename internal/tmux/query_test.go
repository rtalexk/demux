package tmux_test

import (
	"testing"

	"github.com/rtalexk/demux/internal/tmux"
)

func TestParsePanes(t *testing.T) {
	// 10 fields: session_name, session_id, window_index, window_id, pane_index, cwd, pane_id, window_name, pane_pid, session_activity
	raw := "mysession\t$1\t0\t@5\t0\t/home/dev/project\t%1\teditor\t1234\t1711652000\n" +
		"mysession\t$1\t0\t@5\t1\t/home/dev/project/ui\t%2\teditor\t1235\t1711652000\n" +
		"mysession\t$1\t1\t@6\t0\t/home/dev/project\t%3\tserver\t1236\t1711652000\n"

	panes, err := tmux.ParsePanes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(panes) != 3 {
		t.Fatalf("expected 3, got %d", len(panes))
	}
	if panes[0].Session != "mysession" {
		t.Errorf("unexpected session: %s", panes[0].Session)
	}
	if panes[0].SessionID != "$1" {
		t.Errorf("unexpected session id: %s", panes[0].SessionID)
	}
	if panes[0].WindowID != "@5" {
		t.Errorf("unexpected window id: %s", panes[0].WindowID)
	}
	if panes[1].PaneIndex != 1 {
		t.Errorf("unexpected pane index: %d", panes[1].PaneIndex)
	}
	if panes[1].CWD != "/home/dev/project/ui" {
		t.Errorf("unexpected cwd: %s", panes[1].CWD)
	}
	if panes[0].PaneID != "%1" {
		t.Errorf("unexpected pane id: %s", panes[0].PaneID)
	}
	if panes[0].WindowName != "editor" {
		t.Errorf("unexpected window name: %s", panes[0].WindowName)
	}
}

func TestGroupBySessions(t *testing.T) {
	panes := []tmux.Pane{
		{Session: "s1", WindowIndex: 0, PaneIndex: 0, CWD: "/a"},
		{Session: "s1", WindowIndex: 0, PaneIndex: 1, CWD: "/b"},
		{Session: "s1", WindowIndex: 1, PaneIndex: 0, CWD: "/c"},
		{Session: "s2", WindowIndex: 0, PaneIndex: 0, CWD: "/d"},
	}
	grouped := tmux.GroupBySessions(panes)

	if len(grouped) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(grouped))
	}
	if len(grouped["s1"][0]) != 2 {
		t.Errorf("expected 2 panes in s1:window0, got %d", len(grouped["s1"][0]))
	}
	if len(grouped["s1"][1]) != 1 {
		t.Errorf("expected 1 pane in s1:window1, got %d", len(grouped["s1"][1]))
	}
	if len(grouped["s2"][0]) != 1 {
		t.Errorf("expected 1 pane in s2:window0, got %d", len(grouped["s2"][0]))
	}
}

func TestParsePanesEmpty(t *testing.T) {
	panes, err := tmux.ParsePanes("")
	if err != nil {
		t.Fatal(err)
	}
	if len(panes) != 0 {
		t.Errorf("expected 0 panes, got %d", len(panes))
	}
}

func TestParsePanes_WithSessionActivity(t *testing.T) {
	// 10 fields: session_name, session_id, window_index, window_id, pane_index, cwd, pane_id, window_name, pane_pid, session_activity
	raw := "mysession\t$1\t0\t@5\t0\t/home/dev\t%1\teditor\t1234\t1711652000\n"
	panes, err := tmux.ParsePanes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(panes) != 1 {
		t.Fatalf("expected 1 pane, got %d", len(panes))
	}
	if panes[0].SessionActivity != 1711652000 {
		t.Errorf("expected SessionActivity=1711652000, got %d", panes[0].SessionActivity)
	}
}

func TestParsePanes_WithoutSessionActivity_BackwardCompat(t *testing.T) {
	// 9-field format (no session_activity) should still parse with SessionActivity=0
	raw := "mysession\t$1\t0\t@5\t0\t/home/dev\t%1\teditor\t1234\n"
	panes, err := tmux.ParsePanes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(panes) != 1 {
		t.Fatalf("expected 1 pane, got %d", len(panes))
	}
	if panes[0].SessionActivity != 0 {
		t.Errorf("expected SessionActivity=0 for old format, got %d", panes[0].SessionActivity)
	}
}

func TestParsePanes_WithSlotMarker(t *testing.T) {
	// 11-field format: ...session_activity, demux_slot. Slot panes carry "1",
	// every other pane carries an empty trailing field.
	raw := "s\t$1\t0\t@5\t0\t/home/dev\t%1\tdemux-slot\t1234\t1711652000\t1\n" +
		"s\t$1\t0\t@5\t1\t/home/dev/app\t%2\teditor\t1235\t1711652000\t\n"
	panes, err := tmux.ParsePanes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(panes) != 2 {
		t.Fatalf("expected 2 panes, got %d", len(panes))
	}
	if !panes[0].IsSlot {
		t.Error("expected pane 0 (demux_slot=1) to have IsSlot=true")
	}
	if panes[1].IsSlot {
		t.Error("expected pane 1 (demux_slot empty) to have IsSlot=false")
	}
}

func TestParsePanes_WithoutSlotMarker_BackwardCompat(t *testing.T) {
	// 10-field format (no demux_slot) should still parse with IsSlot=false
	raw := "mysession\t$1\t0\t@5\t0\t/home/dev\t%1\teditor\t1234\t1711652000\n"
	panes, err := tmux.ParsePanes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(panes) != 1 {
		t.Fatalf("expected 1 pane, got %d", len(panes))
	}
	if panes[0].IsSlot {
		t.Error("expected IsSlot=false for old format")
	}
}

func TestSessionActivityMap_MaxPerSession(t *testing.T) {
	panes := []tmux.Pane{
		{Session: "s1", SessionActivity: 1000},
		{Session: "s1", SessionActivity: 3000}, // max for s1
		{Session: "s2", SessionActivity: 2000},
	}
	m := tmux.SessionActivityMap(panes)
	if m["s1"].Unix() != 3000 {
		t.Errorf("expected s1=3000, got %d", m["s1"].Unix())
	}
	if m["s2"].Unix() != 2000 {
		t.Errorf("expected s2=2000, got %d", m["s2"].Unix())
	}
}

func TestSessionActivityMap_Empty(t *testing.T) {
	m := tmux.SessionActivityMap(nil)
	if len(m) != 0 {
		t.Errorf("expected empty map, got %v", m)
	}
}

func TestSessionActivityMap_ZeroTimestampSkipped(t *testing.T) {
	panes := []tmux.Pane{
		{Session: "s1", SessionActivity: 0},
	}
	m := tmux.SessionActivityMap(panes)
	if _, ok := m["s1"]; ok {
		t.Error("expected s1 to be absent (zero timestamp skipped)")
	}
}

func TestParseCurrentTarget(t *testing.T) {
	session, window, err := tmux.ParseCurrentTarget("myproject\t3\n")
	if err != nil {
		t.Fatal(err)
	}
	if session != "myproject" {
		t.Errorf("expected session \"myproject\", got %q", session)
	}
	if window != 3 {
		t.Errorf("expected window 3, got %d", window)
	}
}

func TestParseCurrentTarget_Empty(t *testing.T) {
	_, _, err := tmux.ParseCurrentTarget("")
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestParseCurrentTarget_NoTab(t *testing.T) {
	_, _, err := tmux.ParseCurrentTarget("myproject")
	if err == nil {
		t.Error("expected error for input with no tab")
	}
}

func TestPaneIDToTargetMap(t *testing.T) {
	panes := []tmux.Pane{
		{Session: "work", WindowIndex: 0, PaneIndex: 0, PaneID: "%1"},
		{Session: "work", WindowIndex: 1, PaneIndex: 0, PaneID: "%2"},
		{Session: "other", WindowIndex: 0, PaneIndex: 0, PaneID: "%3"},
		{Session: "noid", WindowIndex: 0, PaneIndex: 0, PaneID: ""},
	}

	m := tmux.PaneIDToTargetMap(panes)

	if m["%1"] != "work:0.0" {
		t.Errorf("%%1: want work:0.0, got %q", m["%1"])
	}
	if m["%2"] != "work:1.0" {
		t.Errorf("%%2: want work:1.0, got %q", m["%2"])
	}
	if m["%3"] != "other:0.0" {
		t.Errorf("%%3: want other:0.0, got %q", m["%3"])
	}
	if _, ok := m[""]; ok {
		t.Error("pane with empty PaneID should not appear in map")
	}
}

func TestWindowIDToTargetMap(t *testing.T) {
	panes := []tmux.Pane{
		{Session: "work", WindowIndex: 0, WindowID: "@5", PaneIndex: 0, PaneID: "%1"},
		{Session: "work", WindowIndex: 1, WindowID: "@6", PaneIndex: 0, PaneID: "%2"},
		{Session: "other", WindowIndex: 0, WindowID: "@7", PaneIndex: 0, PaneID: "%3"},
	}

	m := tmux.WindowIDToTargetMap(panes)

	if m["@5"] != "work:0" {
		t.Errorf("@5: want work:0, got %q", m["@5"])
	}
	if m["@6"] != "work:1" {
		t.Errorf("@6: want work:1, got %q", m["@6"])
	}
	if m["@7"] != "other:0" {
		t.Errorf("@7: want other:0, got %q", m["@7"])
	}
}

func TestSessionIDToNameMap(t *testing.T) {
	panes := []tmux.Pane{
		{Session: "work", SessionID: "$1", PaneID: "%1"},
		{Session: "other", SessionID: "$2", PaneID: "%2"},
		{Session: "work", SessionID: "$1", PaneID: "%3"},
	}

	m := tmux.SessionIDToNameMap(panes)

	if m["$1"] != "work" {
		t.Errorf("$1: want work, got %q", m["$1"])
	}
	if m["$2"] != "other" {
		t.Errorf("$2: want other, got %q", m["$2"])
	}
}
