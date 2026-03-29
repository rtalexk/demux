package tmux_test

import (
    "testing"

    "github.com/rtalex/demux/internal/tmux"
)

func TestParsePanes(t *testing.T) {
    raw := "mysession\t0\t0\t/home/dev/project\t%1\teditor\n" +
        "mysession\t0\t1\t/home/dev/project/ui\t%2\teditor\n" +
        "mysession\t1\t0\t/home/dev/project\t%3\tserver\n"

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
    // 8 fields: session, window_index, pane_index, cwd, pane_id, window_name, pane_pid, session_activity
    raw := "mysession\t0\t0\t/home/dev\t%1\teditor\t1234\t1711652000\n"
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
    // Old 7-field format (no session_activity) should still parse with SessionActivity=0
    raw := "mysession\t0\t0\t/home/dev\t%1\teditor\t1234\n"
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
