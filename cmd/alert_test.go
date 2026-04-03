package cmd_test

import (
	"testing"

	"github.com/rtalexk/demux/internal/db"
)

func TestAlertCRUD(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	// set
	if err := d.AlertSet("s:1", "waiting", "info"); err != nil {
		t.Fatal(err)
	}

	// list
	alerts, err := d.AlertList()
	if err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 1 || alerts[0].Target != "s:1" {
		t.Fatalf("unexpected alerts: %v", alerts)
	}

	// replace
	d.AlertSet("s:1", "new reason", "warn")
	alerts, _ = d.AlertList()
	if len(alerts) != 1 || alerts[0].Reason != "new reason" {
		t.Errorf("expected replaced alert")
	}

	// remove
	d.AlertRemove("s:1")
	alerts, _ = d.AlertList()
	if len(alerts) != 0 {
		t.Errorf("expected 0 after remove")
	}
}
