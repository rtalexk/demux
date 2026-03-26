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
