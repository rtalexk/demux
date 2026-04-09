package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rtalexk/demux/internal/db"
	"github.com/rtalexk/demux/internal/tmux"
	"github.com/spf13/cobra"
)

var itemCmd = &cobra.Command{
	Use:   "item",
	Short: "Manage session items (TODOs and Notes)",
}

var itemListOrphanedCmd = &cobra.Command{
	Use:   "list-orphaned",
	Short: "List sessions with items that no longer have a live tmux session",
	RunE:  runItemListOrphaned,
}

var itemClearOrphanedName string

var itemClearOrphanedCmd = &cobra.Command{
	Use:   "clear-orphaned [session]",
	Short: "Delete orphaned items (all, or a specific session with --name)",
	RunE:  runItemClearOrphaned,
}

func init() {
	itemClearOrphanedCmd.Flags().StringVar(&itemClearOrphanedName, "name", "", "Delete only this session's orphaned items")
	itemCmd.AddCommand(itemListOrphanedCmd)
	itemCmd.AddCommand(itemClearOrphanedCmd)
	rootCmd.AddCommand(itemCmd)
}

// liveTmuxSessions returns the names of all currently live tmux sessions.
func liveTmuxSessions() ([]string, error) {
	panes, err := tmux.ListPanes()
	if err != nil {
		return nil, fmt.Errorf("list tmux panes: %w", err)
	}
	grouped := tmux.GroupBySessions(panes)
	names := make([]string, 0, len(grouped))
	for name := range grouped {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func runItemListOrphaned(_ *cobra.Command, _ []string) error {
	database, err := openDB()
	if err != nil {
		return err
	}
	defer database.Close()

	active, err := liveTmuxSessions()
	if err != nil {
		return err
	}

	orphans, err := database.ItemListOrphaned(active)
	if err != nil {
		return err
	}
	if len(orphans) == 0 {
		fmt.Println("No orphaned items.")
		return nil
	}

	sessions := make([]string, 0, len(orphans))
	for sess := range orphans {
		sessions = append(sessions, sess)
	}
	sort.Strings(sessions)

	for _, sess := range sessions {
		items := orphans[sess]
		open := 0
		for _, it := range items {
			if it.Kind == db.KindTodo && !it.Checked {
				open++
			}
		}
		fmt.Printf("%s  (%d items, %d open TODOs)\n", sess, len(items), open)
		for _, it := range items {
			switch it.Kind {
			case db.KindTodo:
				mark := "[ ]"
				if it.Checked {
					mark = "[x]"
				}
				fmt.Printf("  %s %s\n", mark, it.Body)
			case db.KindNote:
				fmt.Printf("  [-] %s\n", it.Body)
			}
		}
	}
	return nil
}

func runItemClearOrphaned(_ *cobra.Command, args []string) error {
	database, err := openDB()
	if err != nil {
		return err
	}
	defer database.Close()

	active, err := liveTmuxSessions()
	if err != nil {
		return err
	}

	orphans, err := database.ItemListOrphaned(active)
	if err != nil {
		return err
	}

	target := itemClearOrphanedName
	if target == "" && len(args) > 0 {
		target = args[0]
	}

	if target != "" {
		if _, ok := orphans[target]; !ok {
			return fmt.Errorf("no orphaned items for session %q", target)
		}
		if err := database.ItemDeleteSession(target); err != nil {
			return err
		}
		fmt.Printf("Cleared orphaned items for %q.\n", target)
		return nil
	}

	if len(orphans) == 0 {
		fmt.Println("No orphaned items to clear.")
		return nil
	}

	names := make([]string, 0, len(orphans))
	for s := range orphans {
		names = append(names, s)
	}
	sort.Strings(names)

	fmt.Printf("This will clear orphaned items for: %s\nContinue? [y/N] ", strings.Join(names, ", "))
	var reply string
	fmt.Scanln(&reply)
	if strings.ToLower(strings.TrimSpace(reply)) != "y" {
		fmt.Println("Aborted.")
		return nil
	}
	for _, sess := range names {
		if err := database.ItemDeleteSession(sess); err != nil {
			fmt.Printf("error clearing %q: %v\n", sess, err)
		} else {
			fmt.Printf("Cleared %q.\n", sess)
		}
	}
	return nil
}
