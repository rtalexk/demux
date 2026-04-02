package session

import (
    "time"

    "github.com/rtalexk/demux/internal/tmux"
)

// WindowTemplate defines a reusable window configuration in sessions.toml.
type WindowTemplate struct {
    ID             string `toml:"id"`
    Name           string `toml:"name"`
    AfterCreateCmd string `toml:"after_create_cmd"`
    From           string `toml:"from"` // inherit by id from another template
}

// ConfigEntry is parsed from sessions.toml / private.toml.
type ConfigEntry struct {
    Name     string   `toml:"name"`
    Path     string   `toml:"path"`
    Worktree bool     `toml:"worktree"`
    Group    string   `toml:"group"`
    Labels   []string `toml:"labels"`
    Icon     string   `toml:"icon"`
    Windows  []string `toml:"windows"` // ordered list of window_template names
}

// DisplayName returns the session identifier used in Tmux and in the TUI.
func (e ConfigEntry) DisplayName() string {
    return e.Name
}

// Session is the unified session type for the TUI sidebar.
type Session struct {
    DisplayName string              // config name or raw Tmux name
    IsLive      bool                // currently running in Tmux
    IsConfig    bool                // has a config entry
    Panes       map[int][]tmux.Pane // populated when IsLive
    Activity    time.Time           // populated when IsLive
    Config      *ConfigEntry        // populated when IsConfig
}

// ResolveWindowSpecs maps a list of window template ids to tmux.WindowSpec values.
// It returns the resolved specs and a slice of ids that were not found.
func ResolveWindowSpecs(ids []string, templates map[string]WindowTemplate) (specs []tmux.WindowSpec, unknown []string) {
    for _, id := range ids {
        t, ok := templates[id]
        if !ok {
            unknown = append(unknown, id)
            continue
        }
        specs = append(specs, tmux.WindowSpec{Name: t.Name, AfterCreateCmd: t.AfterCreateCmd})
    }
    return specs, unknown
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
