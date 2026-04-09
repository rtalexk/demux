package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rtalexk/demux/internal/db"
	demuxlog "github.com/rtalexk/demux/internal/log"
)

const editorFileHeader = "# Checklist for session: %s\n# Lines: [ ] unchecked, [x] checked. Other lines are ignored.\n\n"

// checklistOpenEditor opens $EDITOR (fallback: vi) with the current checklist
// items pre-filled. On editor close the TUI receives a todoEditorDoneMsg.
func (m Model) checklistOpenEditor() (Model, tea.Cmd) {
	items := m.checklistItems
	session := m.checklistSession

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	f, err := os.CreateTemp("", "demux-todo-*.txt")
	if err != nil {
		demuxlog.Error("create temp file for editor", "err", err)
		m.statusMsg = "failed to open editor: " + err.Error()
		m.statusExp = time.Now().Add(4 * time.Second)
		return m, nil
	}

	fmt.Fprintf(f, editorFileHeader, session)
	for _, item := range items {
		mark := " "
		if item.Checked {
			mark = "x"
		}
		fmt.Fprintf(f, "[%s] %s\n", mark, item.Body)
	}
	f.Close()

	tempFile := f.Name()
	demuxlog.Info("opening editor for checklist", "session", session, "editor", editor, "file", tempFile)

	return m, tea.ExecProcess(exec.Command(editor, tempFile), func(err error) tea.Msg {
		return todoEditorDoneMsg{session: session, tempFile: tempFile, err: err}
	})
}

// syncTodosFromFile reads the editor temp file, parses [ ]/[x] lines,
// replaces the session's checklist in the DB, and removes the temp file.
func syncTodosFromFile(d *db.DB, session, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read editor file: %w", err)
	}
	defer os.Remove(path)

	if err := d.TodoDeleteSession(session); err != nil {
		return fmt.Errorf("clear todos: %w", err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		// Must match "[ ] body" or "[x] body" (or "[X] body")
		if len(line) < 4 || line[0] != '[' || line[2] != ']' || line[3] != ' ' {
			continue
		}
		checked := line[1] == 'x' || line[1] == 'X'
		body := strings.TrimSpace(line[4:])
		if body == "" {
			continue
		}
		if err := d.TodoAdd(session, body); err != nil {
			return fmt.Errorf("add todo: %w", err)
		}
		if checked {
			items, err := d.TodoList(session)
			if err != nil {
				return err
			}
			if len(items) > 0 {
				if err := d.TodoToggle(items[len(items)-1].ID); err != nil {
					return fmt.Errorf("toggle todo: %w", err)
				}
			}
		}
	}
	return nil
}

