package db

import (
	"testing"
)

func TestTodoAdd_And_TodoList(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	if err := d.TodoAdd("myapp", "write tests"); err != nil {
		t.Fatalf("TodoAdd: %v", err)
	}
	items, err := d.TodoList("myapp")
	if err != nil {
		t.Fatalf("TodoList: %v", err)
	}
	if len(items) != 1 || items[0].Body != "write tests" || items[0].Checked {
		t.Errorf("TodoList() = %+v", items)
	}
}

func TestTodoList_Empty(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	items, err := d.TodoList("ghost")
	if err != nil {
		t.Fatalf("TodoList: %v", err)
	}
	if items == nil {
		t.Error("TodoList() = nil, want empty slice")
	}
}

func TestTodoToggle(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	d.TodoAdd("myapp", "do something")
	items, _ := d.TodoList("myapp")
	id := items[0].ID

	if err := d.TodoToggle(id); err != nil {
		t.Fatalf("TodoToggle: %v", err)
	}
	items, _ = d.TodoList("myapp")
	if !items[0].Checked {
		t.Error("expected item to be checked after toggle")
	}
	d.TodoToggle(id)
	items, _ = d.TodoList("myapp")
	if items[0].Checked {
		t.Error("expected item to be unchecked after second toggle")
	}
}

func TestTodoUpdate(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	d.TodoAdd("myapp", "old body")
	items, _ := d.TodoList("myapp")
	if err := d.TodoUpdate(items[0].ID, "new body"); err != nil {
		t.Fatalf("TodoUpdate: %v", err)
	}
	items, _ = d.TodoList("myapp")
	if items[0].Body != "new body" {
		t.Errorf("body = %q, want %q", items[0].Body, "new body")
	}
}

func TestTodoDelete(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	d.TodoAdd("myapp", "item")
	items, _ := d.TodoList("myapp")
	if err := d.TodoDelete(items[0].ID); err != nil {
		t.Fatalf("TodoDelete: %v", err)
	}
	items, _ = d.TodoList("myapp")
	if len(items) != 0 {
		t.Errorf("TodoList() after delete = %v, want []", items)
	}
}

func TestTodoDeleteSession(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	d.TodoAdd("myapp", "a")
	d.TodoAdd("myapp", "b")
	d.TodoAdd("other", "c")
	if err := d.TodoDeleteSession("myapp"); err != nil {
		t.Fatalf("TodoDeleteSession: %v", err)
	}
	items, _ := d.TodoList("myapp")
	if len(items) != 0 {
		t.Errorf("myapp items after delete = %v, want []", items)
	}
	items, _ = d.TodoList("other")
	if len(items) != 1 {
		t.Errorf("other items after delete = %v, want 1", items)
	}
}

func TestTodoListOrphaned(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	d.TodoAdd("alive", "task")
	d.TodoAdd("dead", "orphan task")

	orphans, err := d.TodoListOrphaned([]string{"alive"})
	if err != nil {
		t.Fatalf("TodoListOrphaned: %v", err)
	}
	if len(orphans) != 1 {
		t.Fatalf("orphans count = %d, want 1", len(orphans))
	}
	if _, ok := orphans["dead"]; !ok {
		t.Errorf("expected 'dead' in orphans, got %v", orphans)
	}
}

func TestTodoAdd_Position_Order(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	d.TodoAdd("myapp", "first")
	d.TodoAdd("myapp", "second")
	d.TodoAdd("myapp", "third")
	items, _ := d.TodoList("myapp")
	if len(items) != 3 || items[0].Body != "first" || items[2].Body != "third" {
		t.Errorf("items = %+v", items)
	}
}

func TestTodoSessionsWithOpen(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	open, _ := d.TodoSessionsWithOpen()
	if len(open) != 0 {
		t.Errorf("want empty, got %v", open)
	}

	d.TodoAdd("myapp", "task")
	open, _ = d.TodoSessionsWithOpen()
	if len(open) != 1 || open[0] != "myapp" {
		t.Errorf("want [myapp], got %v", open)
	}

	items, _ := d.TodoList("myapp")
	d.TodoToggle(items[0].ID)
	open, _ = d.TodoSessionsWithOpen()
	if len(open) != 0 {
		t.Errorf("want [] after all checked, got %v", open)
	}
}
