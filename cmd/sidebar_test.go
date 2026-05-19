package cmd

import (
	"testing"
)

func TestSidebarCmd_Registered(t *testing.T) {
	if cmd, _, _ := rootCmd.Find([]string{"sidebar"}); cmd == nil || cmd.Use != "sidebar" {
		t.Fatal("expected `demux sidebar` command to be registered")
	}
}

func TestSidebarCmd_Subcommands(t *testing.T) {
	for _, sub := range []string{"toggle", "show", "hide", "follow"} {
		cmd, _, _ := rootCmd.Find([]string{"sidebar", sub})
		if cmd == nil || cmd.Name() != sub {
			t.Errorf("expected `demux sidebar %s`, got %v", sub, cmd)
		}
	}
}

func TestSidebarCmd_FollowIsHidden(t *testing.T) {
	cmd, _, _ := rootCmd.Find([]string{"sidebar", "follow"})
	if cmd == nil {
		t.Fatal("follow subcommand missing")
	}
	if !cmd.Hidden {
		t.Errorf("follow should be hidden from --help")
	}
}

func TestSidebarCmd_SlotIsHidden(t *testing.T) {
	cmd, _, _ := rootCmd.Find([]string{"sidebar", "slot"})
	if cmd == nil {
		t.Fatal("slot subcommand missing")
	}
	if !cmd.Hidden {
		t.Errorf("slot should be hidden from --help")
	}
}
