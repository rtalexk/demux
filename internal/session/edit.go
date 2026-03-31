package session

import (
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "github.com/BurntSushi/toml"
)

func AppendEntry(path string, e ConfigEntry) error {
    if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
        return fmt.Errorf("create config dir: %w", err)
    }

    existing, err := loadRawEntries(path)
    if err != nil {
        return err
    }
    for _, ex := range existing {
        if ex.DisplayName() == e.DisplayName() {
            return fmt.Errorf("session %q already exists in %s", e.DisplayName(), filepath.Base(path))
        }
    }

    f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
    if err != nil {
        return fmt.Errorf("open %s: %w", path, err)
    }
    defer f.Close()

    if _, err = fmt.Fprint(f, formatBlock(e)); err != nil {
        return fmt.Errorf("write %s: %w", filepath.Base(path), err)
    }
    return nil
}

func formatBlock(e ConfigEntry) string {
    var sb strings.Builder
    sb.WriteString("\n[[session]]\n")
    sb.WriteString(fmt.Sprintf("name  = \"%s\"\n", e.Name))
    sb.WriteString(fmt.Sprintf("alias = \"%s\"\n", e.Alias))
    sb.WriteString(fmt.Sprintf("path  = \"%s\"\n", e.Path))
    if e.Worktree {
        sb.WriteString("worktree = true\n")
    }
    if len(e.Labels) > 0 {
        quoted := make([]string, len(e.Labels))
        for i, l := range e.Labels {
            quoted[i] = fmt.Sprintf("\"%s\"", l)
        }
        sb.WriteString(fmt.Sprintf("labels   = [%s]\n", strings.Join(quoted, ", ")))
    }
    if e.Icon != "" {
        sb.WriteString(fmt.Sprintf("icon     = \"%s\"\n", e.Icon))
    }
    return sb.String()
}

func loadRawEntries(path string) ([]ConfigEntry, error) {
    var f sessionsFile
    _, err := toml.DecodeFile(path, &f)
    if err != nil {
        if errors.Is(err, os.ErrNotExist) {
            return nil, nil
        }
        return nil, fmt.Errorf("read %s: %w", filepath.Base(path), err)
    }
    return f.Sessions, nil
}
