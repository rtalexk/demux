package session

import (
    "errors"
    "fmt"
    "os"
    "path/filepath"

    "github.com/BurntSushi/toml"
)

type sessionsFile struct {
    Sessions []ConfigEntry `toml:"session"`
}

// LoadConfigSessions loads sessions.toml then private.toml from configDir.
// Missing files are silently ignored. Invalid entries (missing name/alias/path)
// are skipped with a message to stderr. Duplicate DisplayNames: last entry wins.
func LoadConfigSessions(configDir string) ([]ConfigEntry, error) {
    var result []ConfigEntry
    seen := map[string]int{} // DisplayName → index in result

    for _, name := range []string{"sessions.toml", "private.toml"} {
        path := filepath.Join(configDir, name)
        var f sessionsFile
        _, err := toml.DecodeFile(path, &f)
        if err != nil {
            if errors.Is(err, os.ErrNotExist) {
                continue
            }
            return result, fmt.Errorf("load %s: %w", name, err)
        }
        for _, e := range f.Sessions {
            if e.Name == "" || e.Alias == "" || e.Path == "" {
                fmt.Fprintf(os.Stderr, "demux: skipping session with missing name/alias/path in %s\n", name)
                continue
            }
            dn := e.DisplayName()
            if i, ok := seen[dn]; ok {
                fmt.Fprintf(os.Stderr, "demux: duplicate session %q in %s (overrides previous)\n", dn, name)
                result[i] = e
            } else {
                seen[dn] = len(result)
                result = append(result, e)
            }
        }
    }
    return result, nil
}
