package db

import (
	"testing"
)

func TestItemAdd_And_ItemList(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	if err := d.ItemAdd("myapp", "todo", "write tests"); err != nil {
		t.Fatalf("ItemAdd: %v", err)
	}
	items, err := d.ItemList("myapp")
	if err != nil {
		t.Fatalf("ItemList: %v", err)
	}
	if len(items) != 1 || items[0].Body != "write tests" || items[0].Kind != "todo" || items[0].Checked {
		t.Errorf("ItemList() = %+v", items)
	}
}

func TestItemList_Empty(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	items, err := d.ItemList("ghost")
	if err != nil {
		t.Fatalf("ItemList: %v", err)
	}
	if items == nil {
		t.Error("ItemList() = nil, want empty slice")
	}
}

func TestItemToggle_TodoOnly(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	d.ItemAdd("myapp", "todo", "do something")
	items, _ := d.ItemList("myapp")
	id := items[0].ID

	if err := d.ItemToggle(id); err != nil {
		t.Fatalf("ItemToggle: %v", err)
	}
	items, _ = d.ItemList("myapp")
	if !items[0].Checked {
		t.Error("expected todo to be checked after toggle")
	}
	d.ItemToggle(id)
	items, _ = d.ItemList("myapp")
	if items[0].Checked {
		t.Error("expected todo to be unchecked after second toggle")
	}
}

func TestItemToggle_NoteIsNoop(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	d.ItemAdd("myapp", "note", "just a note")
	items, _ := d.ItemList("myapp")
	id := items[0].ID

	if err := d.ItemToggle(id); err != nil {
		t.Fatalf("ItemToggle on note: %v", err)
	}
	items, _ = d.ItemList("myapp")
	if items[0].Checked {
		t.Error("expected note to remain unchecked after toggle (no-op)")
	}
}

func TestItemUpdate(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	d.ItemAdd("myapp", "todo", "old body")
	items, _ := d.ItemList("myapp")
	if err := d.ItemUpdate(items[0].ID, "new body"); err != nil {
		t.Fatalf("ItemUpdate: %v", err)
	}
	items, _ = d.ItemList("myapp")
	if items[0].Body != "new body" {
		t.Errorf("body = %q, want %q", items[0].Body, "new body")
	}
}

func TestItemDelete(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	d.ItemAdd("myapp", "todo", "item")
	items, _ := d.ItemList("myapp")
	if err := d.ItemDelete(items[0].ID); err != nil {
		t.Fatalf("ItemDelete: %v", err)
	}
	items, _ = d.ItemList("myapp")
	if len(items) != 0 {
		t.Errorf("ItemList() after delete = %v, want []", items)
	}
}

func TestItemDeleteSession(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	d.ItemAdd("myapp", "todo", "a")
	d.ItemAdd("myapp", "note", "b")
	d.ItemAdd("other", "todo", "c")
	if err := d.ItemDeleteSession("myapp"); err != nil {
		t.Fatalf("ItemDeleteSession: %v", err)
	}
	items, _ := d.ItemList("myapp")
	if len(items) != 0 {
		t.Errorf("myapp items after delete = %v, want []", items)
	}
	items, _ = d.ItemList("other")
	if len(items) != 1 {
		t.Errorf("other items after delete = %v, want 1", items)
	}
}

func TestItemListOrphaned(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	d.ItemAdd("alive", "todo", "task")
	d.ItemAdd("dead", "note", "orphan note")

	orphans, err := d.ItemListOrphaned([]string{"alive"})
	if err != nil {
		t.Fatalf("ItemListOrphaned: %v", err)
	}
	if len(orphans) != 1 {
		t.Fatalf("orphans count = %d, want 1", len(orphans))
	}
	if _, ok := orphans["dead"]; !ok {
		t.Errorf("expected 'dead' in orphans, got %v", orphans)
	}
}

func TestItemList_OrderByCreatedAt(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	d.ItemAdd("myapp", "todo", "first")
	d.ItemAdd("myapp", "note", "second")
	d.ItemAdd("myapp", "todo", "third")
	items, _ := d.ItemList("myapp")
	if len(items) != 3 || items[0].Body != "first" || items[2].Body != "third" {
		t.Errorf("items = %+v", items)
	}
}

func TestItemSessionsWithOpen(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	open, _ := d.ItemSessionsWithOpen()
	if len(open) != 0 {
		t.Errorf("want empty, got %v", open)
	}

	d.ItemAdd("myapp", "todo", "task")
	open, _ = d.ItemSessionsWithOpen()
	if len(open) != 1 || open[0] != "myapp" {
		t.Errorf("want [myapp], got %v", open)
	}

	// Notes should not affect SessionsWithOpen (only unchecked todos count).
	d.ItemAdd("myapp", "note", "just a note")
	items, _ := d.ItemList("myapp")
	for _, it := range items {
		if it.Kind == "todo" {
			d.ItemToggle(it.ID)
		}
	}
	open, _ = d.ItemSessionsWithOpen()
	if len(open) != 0 {
		t.Errorf("want [] after all todos checked, got %v", open)
	}
}
