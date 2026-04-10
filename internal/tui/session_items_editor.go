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

const (
	editorFileHeader   = "# Items for session: %s\n# Edit TODOs and Notes below. Empty lines and comments are ignored.\n\n"
	editorSectionTODOs = "# TODOs"
	editorSectionNotes = "# Notes"
)

func (m Model) openItemEditor() (Model, tea.Cmd) {
	items := m.itemList
	session := m.itemSession

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	f, err := os.CreateTemp("", "demux-items-*.txt")
	if err != nil {
		demuxlog.Error("create temp file for editor", "err", err)
		m.statusMsg = "failed to open editor: " + err.Error()
		m.statusExp = time.Now().Add(4 * time.Second)
		return m, nil
	}

	fmt.Fprintf(f, editorFileHeader, session)

	fmt.Fprintln(f, editorSectionTODOs)
	for _, item := range items {
		if item.Kind != db.KindTodo {
			continue
		}
		mark := " "
		if item.Checked {
			mark = "x"
		}
		fmt.Fprintf(f, "[%s] %s\n", mark, item.Body)
	}

	fmt.Fprintln(f, "")
	fmt.Fprintln(f, editorSectionNotes)
	for _, item := range items {
		if item.Kind != db.KindNote {
			continue
		}
		fmt.Fprintln(f, item.Body)
	}
	f.Close()

	tempFile := f.Name()
	demuxlog.Info("opening editor for items", "session", session, "editor", editor, "file", tempFile)

	return m, tea.ExecProcess(exec.Command(editor, tempFile), func(err error) tea.Msg {
		return itemEditorDoneMsg{session: session, tempFile: tempFile, err: err}
	})
}

// syncItemsFromFile reads the editor temp file, parses # TODOs and # Notes sections,
// replaces the session's items in the DB, and removes the temp file.
func syncItemsFromFile(d *db.DB, session, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read editor file: %w", err)
	}
	defer os.Remove(path)

	if err := d.ItemDeleteSession(session); err != nil {
		return fmt.Errorf("clear items: %w", err)
	}

	section := ""

	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)

		switch trimmed {
		case editorSectionTODOs:
			section = db.KindTodo
			continue
		case editorSectionNotes:
			section = db.KindNote
			continue
		}

		// Skip comment lines and blank lines.
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		switch section {
		case db.KindTodo:
			// Expect "[ ] body" or "[x] body".
			if len(trimmed) < 4 || trimmed[0] != '[' || trimmed[2] != ']' || trimmed[3] != ' ' {
				continue
			}
			checked := trimmed[1] == 'x' || trimmed[1] == 'X'
			body := strings.TrimSpace(trimmed[4:])
			if body == "" {
				continue
			}
			if err := d.ItemAdd(session, db.KindTodo, body); err != nil {
				return fmt.Errorf("add todo from editor: %w", err)
			}
			if checked {
				items, err := d.ItemList(session)
				if err != nil {
					return err
				}
				// Toggle the last inserted todo.
				for i := len(items) - 1; i >= 0; i-- {
					if items[i].Kind == db.KindTodo {
						if err := d.ItemToggle(items[i].ID); err != nil {
							return fmt.Errorf("toggle todo from editor: %w", err)
						}
						break
					}
				}
			}

		case db.KindNote:
			if trimmed == "" {
				continue
			}
			if err := d.ItemAdd(session, db.KindNote, trimmed); err != nil {
				return fmt.Errorf("add note from editor: %w", err)
			}
		}
	}
	return nil
}
