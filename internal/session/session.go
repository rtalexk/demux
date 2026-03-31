package session

import (
    "time"

    "github.com/rtalex/demux/internal/tmux"
)

// ConfigEntry is parsed from sessions.toml / private.toml.
type ConfigEntry struct {
    Name     string   `toml:"name"`
    Path     string   `toml:"path"`
    Worktree bool     `toml:"worktree"`
    Alias    string   `toml:"alias"`
    Labels   []string `toml:"labels"`
    Icon     string   `toml:"icon"`
}

// DisplayName returns the session identifier used in Tmux and in the TUI.
func (e ConfigEntry) DisplayName() string {
    return e.Alias + "-" + e.Name
}

// Session is the unified session type for the TUI sidebar.
type Session struct {
    DisplayName string              // <alias>-<name> or raw Tmux name
    IsLive      bool                // currently running in Tmux
    IsConfig    bool                // has a config entry
    Panes       map[int][]tmux.Pane // populated when IsLive
    Activity    time.Time           // populated when IsLive
    Config      *ConfigEntry        // populated when IsConfig
}

// Merge combines live Tmux panes and config entries into a unified []Session.
// Match rule: Tmux session name must equal entry.DisplayName() exactly.
func Merge(panes []tmux.Pane, entries []ConfigEntry) []Session {
    grouped := tmux.GroupBySessions(panes)
    activity := tmux.SessionActivityMap(panes)

    configByName := make(map[string]*ConfigEntry, len(entries))
    for i := range entries {
        configByName[entries[i].DisplayName()] = &entries[i]
    }

    matched := make(map[string]bool, len(entries))
    var sessions []Session

    for name, windows := range grouped {
        s := Session{
            DisplayName: name,
            IsLive:      true,
            Panes:       windows,
            Activity:    activity[name],
        }
        if ce, ok := configByName[name]; ok {
            s.IsConfig = true
            s.Config = ce
            matched[name] = true
        }
        sessions = append(sessions, s)
    }

    for i := range entries {
        dn := entries[i].DisplayName()
        if !matched[dn] {
            sessions = append(sessions, Session{
                DisplayName: dn,
                IsLive:      false,
                IsConfig:    true,
                Config:      &entries[i],
            })
        }
    }

    return sessions
}
