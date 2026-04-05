package cmd

import (
    "testing"
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
