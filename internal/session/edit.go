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

// tomlQuote wraps s in double quotes with only the escapes required for a
// TOML basic string: backslash and double-quote. Unicode is left as literal.
func tomlQuote(s string) string {
    s = strings.ReplaceAll(s, "\\", "\\\\")
    s = strings.ReplaceAll(s, "\"", "\\\"")
    return "\"" + s + "\""
}

func formatBlock(e ConfigEntry) string {
    var sb strings.Builder
    sb.WriteString("\n[[session]]\n")
    sb.WriteString(fmt.Sprintf("name  = %s\n", tomlQuote(e.Name)))
    sb.WriteString(fmt.Sprintf("alias = %s\n", tomlQuote(e.Alias)))
    sb.WriteString(fmt.Sprintf("path  = %s\n", tomlQuote(e.Path)))
    if e.Worktree {
        sb.WriteString("worktree = true\n")
    }
    if len(e.Labels) > 0 {
        quoted := make([]string, len(e.Labels))
        for i, l := range e.Labels {
            quoted[i] = tomlQuote(l)
        }
        sb.WriteString(fmt.Sprintf("labels   = [%s]\n", strings.Join(quoted, ", ")))
    }
    if e.Icon != "" {
        sb.WriteString(fmt.Sprintf("icon     = %s\n", tomlQuote(e.Icon)))
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

// RemoveEntry removes the [[session]] block matching both name and alias from path.
// Returns an error if the file does not exist or the entry is not found.
func RemoveEntry(path, name, alias string) error {
    data, err := os.ReadFile(path)
    if err != nil {
        return fmt.Errorf("read %s: %w", filepath.Base(path), err)
    }

    blocks, preamble := splitBlocks(string(data))

    target := -1
    for i, b := range blocks {
        if blockHasField(b, "name", name) && blockHasField(b, "alias", alias) {
            target = i
            break
        }
    }
    if target == -1 {
        return fmt.Errorf("session %q (alias %q) not found in %s", name, alias, filepath.Base(path))
    }

    blocks = append(blocks[:target], blocks[target+1:]...)

    var sb strings.Builder
    sb.WriteString(preamble)
    for _, b := range blocks {
        sb.WriteString(b)
    }

    result := strings.TrimRight(sb.String(), "\n")
    if result != "" {
        result += "\n"
    }

    return os.WriteFile(path, []byte(result), 0644)
}

// splitBlocks splits TOML content into a preamble (lines before first [[session]])
// and a slice of [[session]] block strings (each starting with "[[session]]\n").
func splitBlocks(content string) (blocks []string, preamble string) {
    lines := strings.Split(content, "\n")
    var pre []string
    var cur []string
    inBlock := false

    for _, line := range lines {
        if strings.TrimSpace(line) == "[[session]]" {
            if inBlock {
                blocks = append(blocks, strings.Join(cur, "\n")+"\n")
                cur = nil
            }
            inBlock = true
            cur = append(cur, line)
        } else if inBlock {
            cur = append(cur, line)
        } else {
            pre = append(pre, line)
        }
    }
    if inBlock && len(cur) > 0 {
        blocks = append(blocks, strings.Join(cur, "\n")+"\n")
    }
    preamble = strings.Join(pre, "\n")
    if preamble != "" && !strings.HasSuffix(preamble, "\n") {
        preamble += "\n"
    }
    return blocks, preamble
}

// blockHasField reports whether the block contains key = "value" (handles extra spaces around =).
func blockHasField(block, key, value string) bool {
    needle := tomlQuote(value)
    for _, line := range strings.Split(block, "\n") {
        trimmed := strings.TrimSpace(line)
        if !strings.HasPrefix(trimmed, key) {
            continue
        }
        rest := strings.TrimPrefix(trimmed, key)
        if len(rest) == 0 || (rest[0] != '=' && rest[0] != ' ' && rest[0] != '\t') {
            continue
        }
        rest = strings.TrimSpace(rest)
        if strings.HasPrefix(rest, "=") {
            val := strings.TrimSpace(strings.TrimPrefix(rest, "="))
            if val == needle {
                return true
            }
        }
    }
    return false
}
