package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rtalexk/demux/internal/config"
	"github.com/rtalexk/demux/internal/db"
	"github.com/rtalexk/demux/internal/tmux"
)

func TestSessionNameToIDMap(t *testing.T) {
	panes := []tmux.Pane{
		{Session: "work", SessionID: "$1", WindowIndex: 0, PaneIndex: 0, PaneID: "%1"},
		{Session: "work", SessionID: "$1", WindowIndex: 0, PaneIndex: 1, PaneID: "%2"},
		{Session: "other", SessionID: "$2", WindowIndex: 0, PaneIndex: 0, PaneID: "%3"},
	}
	m := tmux.SessionNameToIDMap(panes)
	if got := m["work"]; got != "$1" {
		t.Errorf("work: want $1, got %q", got)
	}
	if got := m["other"]; got != "$2" {
		t.Errorf("other: want $2, got %q", got)
	}
}

func TestSessionNameToIDMap_Empty(t *testing.T) {
	m := tmux.SessionNameToIDMap(nil)
	if len(m) != 0 {
		t.Errorf("want empty map, got %v", m)
	}
}

func TestHealStateSessionIDs_UpdatesSessionID(t *testing.T) {
	states := []db.ToolState{
		{Target: db.Target{Type: db.TargetTypePane, ID: "%1", SessionID: "$0"}},
	}
	paneIDMap := map[string]string{"%1": "work:%1"}
	nameToIDMap := map[string]string{"work": "$1"}

	healStateSessionIDs(states, paneIDMap, nameToIDMap)

	if states[0].Target.SessionID != "$1" {
		t.Errorf("expected SessionID $1, got %q", states[0].Target.SessionID)
	}
}

func TestHealStateSessionIDs_NoopWhenAlreadyCorrect(t *testing.T) {
	states := []db.ToolState{
		{Target: db.Target{Type: db.TargetTypePane, ID: "%1", SessionID: "$1"}},
	}
	paneIDMap := map[string]string{"%1": "work:%1"}
	nameToIDMap := map[string]string{"work": "$1"}

	healStateSessionIDs(states, paneIDMap, nameToIDMap)

	if states[0].Target.SessionID != "$1" {
		t.Errorf("session ID should be unchanged, got %q", states[0].Target.SessionID)
	}
}

func TestHealStateSessionIDs_IgnoresNonPane(t *testing.T) {
	states := []db.ToolState{
		{Target: db.Target{Type: db.TargetTypeWindow, ID: "@1", SessionID: "$0"}},
	}
	paneIDMap := map[string]string{"@1": "work:@1"}
	nameToIDMap := map[string]string{"work": "$1"}

	healStateSessionIDs(states, paneIDMap, nameToIDMap)

	if states[0].Target.SessionID != "$0" {
		t.Errorf("non-pane entry should be unchanged, got %q", states[0].Target.SessionID)
	}
}

func TestHealStateSessionIDs_IgnoresUnknownPane(t *testing.T) {
	states := []db.ToolState{
		{Target: db.Target{Type: db.TargetTypePane, ID: "%99", SessionID: "$0"}},
	}
	paneIDMap := map[string]string{} // %99 not in live map
	nameToIDMap := map[string]string{"work": "$1"}

	healStateSessionIDs(states, paneIDMap, nameToIDMap)

	if states[0].Target.SessionID != "$0" {
		t.Errorf("unknown pane should be unchanged, got %q", states[0].Target.SessionID)
	}
}

func TestUpdateYank_Enter_CopyError_ShowsFailedStatus(t *testing.T) {
	old := clipboardFunc
	clipboardFunc = func(string) error { return errors.New("clipboard unavailable") }
	defer func() { clipboardFunc = old }()

	m := New(config.Default(), nil)
	m.showYank = true
	m.yank.SetFields([]YankField{
		{Key: "s", Label: "session", Value: "work"},
	}, "Copy")

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := result.(Model)
	if !strings.HasPrefix(got.statusMsg, "copy failed:") {
		t.Errorf("want 'copy failed:...' status, got %q", got.statusMsg)
	}
}

func TestHandleItemsMsg_SortsTodosBeforeNotes(t *testing.T) {
	m := New(config.Default(), nil)
	m.itemsMode = true
	m.itemSession = "work"

	// Items in DB insertion order: todo, note, todo
	items := []db.Item{
		{ID: 1, Kind: db.KindTodo, Body: "first todo"},
		{ID: 2, Kind: db.KindNote, Body: "a note"},
		{ID: 3, Kind: db.KindTodo, Body: "second todo"},
	}

	m, _ = m.handleItemsMsg(itemsMsg{session: "work", items: items})

	if len(m.itemList) != 3 {
		t.Fatalf("expected 3 items, got %d", len(m.itemList))
	}
	if m.itemList[0].Body != "first todo" {
		t.Errorf("index 0: want 'first todo', got %q", m.itemList[0].Body)
	}
	if m.itemList[1].Body != "second todo" {
		t.Errorf("index 1: want 'second todo', got %q (second todo must follow first todo)", m.itemList[1].Body)
	}
	if m.itemList[2].Body != "a note" {
		t.Errorf("index 2: want 'a note', got %q (notes must follow todos)", m.itemList[2].Body)
	}
}

func TestHandleItemsMsg_NavigationReachesSecondTodo(t *testing.T) {
	m := New(config.Default(), nil)
	m.itemsMode = true
	m.itemSession = "work"

	// Todo, note interleaved in DB order — second todo must be reachable with one j press.
	items := []db.Item{
		{ID: 1, Kind: db.KindTodo, Body: "first todo"},
		{ID: 2, Kind: db.KindNote, Body: "a note"},
		{ID: 3, Kind: db.KindTodo, Body: "second todo"},
	}
	m, _ = m.handleItemsMsg(itemsMsg{session: "work", items: items})
	m.itemCursor = 0

	got, _ := m.handleItemsKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})

	if got.itemCursor != 1 {
		t.Fatalf("expected cursor=1 after j, got %d", got.itemCursor)
	}
	if got.itemList[got.itemCursor].Kind != db.KindTodo {
		t.Errorf("cursor should land on todo after j, got kind=%v body=%q",
			got.itemList[got.itemCursor].Kind, got.itemList[got.itemCursor].Body)
	}
	if got.itemList[got.itemCursor].Body != "second todo" {
		t.Errorf("want 'second todo' selected, got %q", got.itemList[got.itemCursor].Body)
	}
}
