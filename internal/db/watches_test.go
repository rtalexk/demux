package db

import (
	"testing"
)

func TestWatchSet_And_WatchList(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	if err := d.WatchSet("myapp"); err != nil {
		t.Fatalf("WatchSet: %v", err)
	}
	list, err := d.WatchList()
	if err != nil {
		t.Fatalf("WatchList: %v", err)
	}
	if len(list) != 1 || list[0] != "myapp" {
		t.Errorf("WatchList() = %v, want [myapp]", list)
	}
}

func TestWatchSet_Idempotent(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	d.WatchSet("myapp")
	if err := d.WatchSet("myapp"); err != nil {
		t.Fatalf("second WatchSet should be a no-op, got: %v", err)
	}
	list, _ := d.WatchList()
	if len(list) != 1 {
		t.Errorf("WatchList() = %v, want exactly 1 entry", list)
	}
}

func TestWatchClear(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	d.WatchSet("myapp")
	if err := d.WatchClear("myapp"); err != nil {
		t.Fatalf("WatchClear: %v", err)
	}
	list, _ := d.WatchList()
	if len(list) != 0 {
		t.Errorf("WatchList() after clear = %v, want []", list)
	}
}

func TestWatchClear_NonExistent(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	if err := d.WatchClear("ghost"); err != nil {
		t.Fatalf("WatchClear of missing session should not error, got: %v", err)
	}
}

func TestWatchList_Empty(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	list, err := d.WatchList()
	if err != nil {
		t.Fatalf("WatchList: %v", err)
	}
	if list == nil {
		t.Error("WatchList() = nil, want empty slice")
	}
}

func TestWatchList_MultipleEntries(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()

	d.WatchSet("alpha")
	d.WatchSet("beta")
	d.WatchSet("gamma")
	list, _ := d.WatchList()
	if len(list) != 3 {
		t.Errorf("WatchList() len = %d, want 3", len(list))
	}
}
