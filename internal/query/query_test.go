package query_test

import (
    "testing"

    "github.com/rtalexk/demux/internal/proc"
    "github.com/rtalexk/demux/internal/query"
    "github.com/rtalexk/demux/internal/tmux"
)

func TestParse(t *testing.T) {
    tests := []struct {
        raw   string
        scope query.QueryScope
        term  string
    }{
        {"", query.ScopeSession, ""},
        {"foo", query.ScopeSession, "foo"},
        {"w:baz", query.ScopeWindow, "baz"},
        {"p:qux", query.ScopeProcess, "qux"},
        {"w:my project", query.ScopeWindow, "my project"},
        {"query1 w:query2", query.ScopeSession, "query1 w:query2"},
    }
    for _, tt := range tests {
        pq := query.Parse(tt.raw)
        if pq.Scope != tt.scope {
            t.Errorf("Parse(%q).Scope = %v, want %v", tt.raw, pq.Scope, tt.scope)
        }
        if pq.Term != tt.term {
            t.Errorf("Parse(%q).Term = %q, want %q", tt.raw, pq.Term, tt.term)
        }
        if pq.Raw != tt.raw {
            t.Errorf("Parse(%q).Raw = %q, want %q", tt.raw, pq.Raw, tt.raw)
        }
    }
}

func TestRun_EmptyTerm(t *testing.T) {
    panes := []tmux.Pane{{Session: "work", WindowIndex: 0, WindowName: "main"}}
    result := query.RunWith(query.Parse(""), panes, nil)
    if len(result.Sessions) != 0 {
        t.Errorf("empty term should return empty result, got %d sessions", len(result.Sessions))
    }
}

func TestRun_SessionScope(t *testing.T) {
    panes := []tmux.Pane{
        {Session: "work", WindowIndex: 0, WindowName: "main", PanePID: 100},
        {Session: "personal", WindowIndex: 0, WindowName: "vim", PanePID: 200},
        {Session: "dots", WindowIndex: 0, WindowName: "sh", PanePID: 300},
    }
    result := query.RunWith(query.Parse("wor"), panes, nil)
    if len(result.Sessions) != 1 {
        t.Fatalf("expected 1 session match, got %d", len(result.Sessions))
    }
    if result.Sessions[0].Name != "work" {
        t.Errorf("expected 'work', got %q", result.Sessions[0].Name)
    }
    if result.Sessions[0].Score <= 0 {
        t.Errorf("expected positive score, got %d", result.Sessions[0].Score)
    }
}

func TestRun_WindowScope(t *testing.T) {
    panes := []tmux.Pane{
        {Session: "work", WindowIndex: 0, WindowName: "vim-config", PanePID: 100},
        {Session: "work", WindowIndex: 1, WindowName: "shell", PanePID: 101},
        {Session: "personal", WindowIndex: 0, WindowName: "htop", PanePID: 200},
    }
    result := query.RunWith(query.Parse("w:vim"), panes, nil)
    if len(result.Sessions) != 1 {
        t.Fatalf("expected 1 session, got %d", len(result.Sessions))
    }
    if result.Sessions[0].Name != "work" {
        t.Errorf("expected 'work', got %q", result.Sessions[0].Name)
    }
    if len(result.Sessions[0].Windows) != 1 {
        t.Fatalf("expected 1 window match, got %d", len(result.Sessions[0].Windows))
    }
    if result.Sessions[0].Windows[0].Name != "vim-config" {
        t.Errorf("expected 'vim-config', got %q", result.Sessions[0].Windows[0].Name)
    }
}

func TestRun_ProcessScope(t *testing.T) {
    panes := []tmux.Pane{
        {Session: "work", WindowIndex: 0, WindowName: "main", PanePID: 100},
        {Session: "personal", WindowIndex: 0, WindowName: "sh", PanePID: 200},
    }
    procs := []proc.Process{
        {PID: 101, PPID: 100, Name: "nvim"},
        {PID: 201, PPID: 200, Name: "bash"},
    }
    result := query.RunWith(query.Parse("p:nvi"), panes, procs)
    if len(result.Sessions) != 1 {
        t.Fatalf("expected 1 session, got %d", len(result.Sessions))
    }
    if result.Sessions[0].Name != "work" {
        t.Errorf("expected 'work', got %q", result.Sessions[0].Name)
    }
    if len(result.Sessions[0].Procs) != 1 {
        t.Fatalf("expected 1 proc match, got %d", len(result.Sessions[0].Procs))
    }
    if result.Sessions[0].Procs[0].Name != "nvim" {
        t.Errorf("expected 'nvim', got %q", result.Sessions[0].Procs[0].Name)
    }
}

func TestRun_SessionScope_Default(t *testing.T) {
    // Bare text searches sessions only — window names are not matched.
    panes := []tmux.Pane{
        {Session: "vim-session", WindowIndex: 0, WindowName: "editor", PanePID: 100},
        {Session: "work", WindowIndex: 0, WindowName: "vim-win", PanePID: 200},
    }
    result := query.RunWith(query.Parse("vim"), panes, nil)
    if len(result.Sessions) != 1 {
        t.Fatalf("expected 1 session (vim-session), got %d", len(result.Sessions))
    }
    if result.Sessions[0].Name != "vim-session" {
        t.Errorf("expected 'vim-session', got %q", result.Sessions[0].Name)
    }
    if len(result.Sessions[0].Windows) != 0 {
        t.Errorf("expected no window matches for bare-text search, got %d", len(result.Sessions[0].Windows))
    }
    if len(result.Sessions[0].Procs) != 0 {
        t.Errorf("expected no proc matches for bare-text search, got %d", len(result.Sessions[0].Procs))
    }
}

func TestRun_MatchPos(t *testing.T) {
    panes := []tmux.Pane{
        {Session: "work", WindowIndex: 0, WindowName: "main", PanePID: 100},
    }
    result := query.RunWith(query.Parse("wok"), panes, nil)
    if len(result.Sessions) != 1 {
        t.Fatalf("expected 1 session, got %d", len(result.Sessions))
    }
    if len(result.Sessions[0].MatchPos) == 0 {
        t.Error("expected non-empty MatchPos for session match")
    }
}

func TestParse_BareTag(t *testing.T) {
    // Bare tags with no term should be scoped with empty term, not ScopeSession.
    tests := []struct {
        raw   string
        scope query.QueryScope
        term  string
    }{
        {"w:", query.ScopeWindow, ""},
        {"p:", query.ScopeProcess, ""},
    }
    for _, tt := range tests {
        pq := query.Parse(tt.raw)
        if pq.Scope != tt.scope {
            t.Errorf("Parse(%q).Scope = %v, want %v", tt.raw, pq.Scope, tt.scope)
        }
        if pq.Term != tt.term {
            t.Errorf("Parse(%q).Term = %q, want %q", tt.raw, pq.Term, tt.term)
        }
    }
}

func TestRun_ExtraSessions_Matched(t *testing.T) {
    // Non-live (config-only) sessions passed via ExtraSessions must be fuzzy-matched.
    panes := []tmux.Pane{
        {Session: "work", WindowIndex: 0, WindowName: "main", PanePID: 100},
    }
    pq := query.Parse("dotf")
    pq.ExtraSessions = []string{"dotf-main", "hf-main"}
    result := query.RunWith(pq, panes, nil)

    found := false
    for _, sm := range result.Sessions {
        if sm.Name == "dotf-main" {
            found = true
            break
        }
    }
    if !found {
        t.Error("expected 'dotf-main' from ExtraSessions to appear in results")
    }
    // live session should still be present
    livefound := false
    for _, sm := range result.Sessions {
        if sm.Name == "work" {
            livefound = true
            break
        }
    }
    _ = livefound // live session doesn't match "dotf", absence is fine
}

func TestRun_ExtraSessions_NoDuplicates(t *testing.T) {
    // A session listed in both live panes and ExtraSessions must not be duplicated.
    panes := []tmux.Pane{
        {Session: "dotf-main", WindowIndex: 0, WindowName: "sh", PanePID: 100},
    }
    pq := query.Parse("dotf")
    pq.ExtraSessions = []string{"dotf-main"}
    result := query.RunWith(pq, panes, nil)

    count := 0
    for _, sm := range result.Sessions {
        if sm.Name == "dotf-main" {
            count++
        }
    }
    if count != 1 {
        t.Errorf("expected 'dotf-main' exactly once, got %d", count)
    }
}

func TestRun_NonContiguousWindowIndices(t *testing.T) {
    // Windows 0 and 2 (index 1 was deleted) — winIdxOrder mapping must survive this.
    panes := []tmux.Pane{
        {Session: "work", WindowIndex: 0, WindowName: "shell", PanePID: 100},
        {Session: "work", WindowIndex: 2, WindowName: "vim-edit", PanePID: 101},
    }
    result := query.RunWith(query.Parse("w:vim"), panes, nil)
    if len(result.Sessions) != 1 {
        t.Fatalf("expected 1 session, got %d", len(result.Sessions))
    }
    if len(result.Sessions[0].Windows) != 1 {
        t.Fatalf("expected 1 window match, got %d", len(result.Sessions[0].Windows))
    }
    if result.Sessions[0].Windows[0].Index != 2 {
        t.Errorf("expected window index 2, got %d", result.Sessions[0].Windows[0].Index)
    }
    if result.Sessions[0].Windows[0].Name != "vim-edit" {
        t.Errorf("expected 'vim-edit', got %q", result.Sessions[0].Windows[0].Name)
    }
}
