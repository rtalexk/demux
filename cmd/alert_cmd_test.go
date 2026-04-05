package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/rtalexk/demux/internal/db"
)

func TestAlertSetSticky_InvalidLevel(t *testing.T) {
	// Reset flags to known state.
	alertSetTarget = "main"
	alertLevel = "info"
	alertSticky = true
	alertReason = "test"

	err := alertSetCmd.RunE(alertSetCmd, []string{})
	if err == nil {
		t.Fatal("expected error for --sticky with non-defer level")
	}
	want := "--sticky is only valid with --level defer"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestAlertRemove_StickyWarning(t *testing.T) {
	// Use a temp file so the DB persists after the command closes it.
	tmp := t.TempDir() + "/test.db"

	setup, err := db.Open(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if err := setup.AlertSet("main", "Come back", db.LevelDefer, true); err != nil {
		setup.Close()
		t.Fatal(err)
	}
	setup.Close()

	// Override openDB to open the temp file each call.
	orig := openDB
	openDB = func() (*db.DB, error) { return db.Open(tmp) }
	defer func() { openDB = orig }()

	alertRemoveTarget = "main"
	alertRemoveForce = false

	// Capture stderr.
	r, w, _ := os.Pipe()
	origStderr := os.Stderr
	os.Stderr = w

	err = alertRemoveCmd.RunE(alertRemoveCmd, []string{})

	w.Close()
	os.Stderr = origStderr
	var buf bytes.Buffer
	io.Copy(&buf, r)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	wantMsg := fmt.Sprintf("warning: alert on %q is sticky; use --force to remove it\n", "main")
	if buf.String() != wantMsg {
		t.Errorf("stderr got %q, want %q", buf.String(), wantMsg)
	}

	// Alert should still exist (verify by reopening the DB).
	verify, _ := db.Open(tmp)
	defer verify.Close()
	a, _ := verify.AlertByTarget("main")
	if a == nil {
		t.Error("expected alert to still exist after warning (no --force)")
	}
}

func TestAlertRemove_ForceRemovesSticky(t *testing.T) {
	// Use a temp file so the DB persists after the command closes it.
	tmp := t.TempDir() + "/test.db"

	setup, err := db.Open(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if err := setup.AlertSet("main", "Come back", db.LevelDefer, true); err != nil {
		setup.Close()
		t.Fatal(err)
	}
	setup.Close()

	// Override openDB to open the temp file each call.
	orig := openDB
	openDB = func() (*db.DB, error) { return db.Open(tmp) }
	defer func() { openDB = orig }()

	alertRemoveTarget = "main"
	alertRemoveForce = true

	err = alertRemoveCmd.RunE(alertRemoveCmd, []string{})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Alert should be gone (verify by reopening the DB).
	verify, _ := db.Open(tmp)
	defer verify.Close()
	a, _ := verify.AlertByTarget("main")
	if a != nil {
		t.Error("expected alert to be removed after --force")
	}
}
