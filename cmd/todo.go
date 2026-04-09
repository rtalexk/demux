package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rtalexk/demux/internal/tmux"
	"github.com/spf13/cobra"
)

var todoCmd = &cobra.Command{
	Use:   "todo",
	Short: "Manage session TODO checklists",
}

var todoListOrphanedCmd = &cobra.Command{
	Use:   "list-orphaned",
	Short: "List sessions with checklists that no longer have a live tmux session",
	RunE:  runTodoListOrphaned,
}

var todoClearOrphanedName string

var todoClearOrphanedCmd = &cobra.Command{
	Use:   "clear-orphaned [session]",
	Short: "Delete orphaned checklists (all, or a specific session with --name)",
	RunE:  runTodoClearOrphaned,
}

func init() {
	todoClearOrphanedCmd.Flags().StringVar(&todoClearOrphanedName, "name", "", "Delete only this session's orphaned checklist")
	todoCmd.AddCommand(todoListOrphanedCmd)
	todoCmd.AddCommand(todoClearOrphanedCmd)
	rootCmd.AddCommand(todoCmd)
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

func runTodoListOrphaned(_ *cobra.Command, _ []string) error {
	database, err := openDB()
	if err != nil {
		return err
	}
	defer database.Close()

	active, err := liveTmuxSessions()
	if err != nil {
		return err
	}

	orphans, err := database.TodoListOrphaned(active)
	if err != nil {
		return err
	}
	if len(orphans) == 0 {
		fmt.Println("No orphaned checklists.")
		return nil
	}

	// Print in sorted order for deterministic output.
	sessions := make([]string, 0, len(orphans))
	for sess := range orphans {
		sessions = append(sessions, sess)
	}
	sort.Strings(sessions)

	for _, sess := range sessions {
		items := orphans[sess]
		open := 0
		for _, it := range items {
			if !it.Checked {
				open++
			}
		}
		fmt.Printf("%s  (%d items, %d open)\n", sess, len(items), open)
		for _, it := range items {
			mark := "[ ]"
			if it.Checked {
				mark = "[x]"
			}
			fmt.Printf("  %s %s\n", mark, it.Body)
		}
	}
	return nil
}

func runTodoClearOrphaned(_ *cobra.Command, args []string) error {
	database, err := openDB()
	if err != nil {
		return err
	}
	defer database.Close()

	active, err := liveTmuxSessions()
	if err != nil {
		return err
	}

	orphans, err := database.TodoListOrphaned(active)
	if err != nil {
		return err
	}

	// Specific session via --name or first positional arg.
	target := todoClearOrphanedName
	if target == "" && len(args) > 0 {
		target = args[0]
	}

	if target != "" {
		if _, ok := orphans[target]; !ok {
			return fmt.Errorf("no orphaned checklist for session %q", target)
		}
		if err := database.TodoDeleteSession(target); err != nil {
			return err
		}
		fmt.Printf("Cleared orphaned checklist for %q.\n", target)
		return nil
	}

	if len(orphans) == 0 {
		fmt.Println("No orphaned checklists to clear.")
		return nil
	}

	names := make([]string, 0, len(orphans))
	for s := range orphans {
		names = append(names, s)
	}
	sort.Strings(names)

	fmt.Printf("This will clear orphaned checklists for: %s\nContinue? [y/N] ", strings.Join(names, ", "))
	var reply string
	fmt.Scanln(&reply)
	if strings.ToLower(strings.TrimSpace(reply)) != "y" {
		fmt.Println("Aborted.")
		return nil
	}
	for _, sess := range names {
		if err := database.TodoDeleteSession(sess); err != nil {
			fmt.Printf("error clearing %q: %v\n", sess, err)
		} else {
			fmt.Printf("Cleared %q.\n", sess)
		}
	}
	return nil
}
