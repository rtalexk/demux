package session

import (
    "errors"
    "fmt"
    "os"
    "path/filepath"

    "github.com/BurntSushi/toml"
)

type sessionsFile struct {
    Sessions        []ConfigEntry    `toml:"session"`
    WindowTemplates []WindowTemplate `toml:"window_templates"`
}

// SessionsConfig is the parsed result of sessions.toml and private.toml.
type SessionsConfig struct {
    Entries         []ConfigEntry
    WindowTemplates map[string]WindowTemplate // resolved by id, from-inheritance applied
}

// LoadConfigSessions loads sessions.toml then private.toml from configDir.
// Missing files are silently ignored. Invalid entries (missing name/alias/path)
// are skipped with a message to stderr. Duplicate DisplayNames: last entry wins.
func LoadConfigSessions(configDir string) (SessionsConfig, error) {
    var cfg SessionsConfig
    seen := map[string]int{} // DisplayName → index in cfg.Entries
    var rawTemplates []WindowTemplate

    for _, name := range []string{"sessions.toml", "private.toml"} {
        path := filepath.Join(configDir, name)
        var f sessionsFile
        _, err := toml.DecodeFile(path, &f)
        if err != nil {
            if errors.Is(err, os.ErrNotExist) {
                continue
            }
            return cfg, fmt.Errorf("load %s: %w", name, err)
        }
        for _, e := range f.Sessions {
            if e.Name == "" || e.Alias == "" || e.Path == "" {
                fmt.Fprintf(os.Stderr, "demux: skipping session with missing name/alias/path in %s\n", name)
                continue
            }
            dn := e.DisplayName()
            if i, ok := seen[dn]; ok {
                fmt.Fprintf(os.Stderr, "demux: duplicate session %q in %s (overrides previous)\n", dn, name)
                cfg.Entries[i] = e
            } else {
                seen[dn] = len(cfg.Entries)
                cfg.Entries = append(cfg.Entries, e)
            }
        }
        rawTemplates = append(rawTemplates, f.WindowTemplates...)
    }

    cfg.WindowTemplates = resolveWindowTemplates(rawTemplates)
    return cfg, nil
}

// resolveWindowTemplates builds an id-keyed map of WindowTemplate with
// single-level from-inheritance applied. Later entries override earlier ones
// with the same id.
func resolveWindowTemplates(raw []WindowTemplate) map[string]WindowTemplate {
    byID := make(map[string]WindowTemplate, len(raw))
    for _, t := range raw {
        byID[t.ID] = t
    }
    resolved := make(map[string]WindowTemplate, len(raw))
    for _, t := range raw {
        if t.From != "" {
            base, ok := byID[t.From]
            if !ok {
                fmt.Fprintf(os.Stderr, "demux: window_template %q references unknown template id %q\n", t.ID, t.From)
                resolved[t.ID] = t
                continue
            }
            merged := base
            merged.ID = t.ID
            merged.Name = t.Name
            merged.From = ""
            if t.AfterCreateCmd != "" {
                merged.AfterCreateCmd = t.AfterCreateCmd
            }
            resolved[t.ID] = merged
        } else {
            resolved[t.ID] = t
        }
    }
    return resolved
}
