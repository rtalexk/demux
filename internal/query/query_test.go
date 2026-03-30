package query_test

import (
    "testing"

    "github.com/rtalex/demux/internal/proc"
    "github.com/rtalex/demux/internal/query"
    "github.com/rtalex/demux/internal/tmux"
)

func TestParse(t *testing.T) {
    tests := []struct {
        raw   string
        scope query.QueryScope
        term  string
    }{
        {"", query.ScopeAll, ""},
        {"foo", query.ScopeAll, "foo"},
        {"s:bar", query.ScopeSession, "bar"},
        {"w:baz", query.ScopeWindow, "baz"},
        {"p:qux", query.ScopeProcess, "qux"},
        {"s:my project", query.ScopeSession, "my project"},
        {"query1 w:query2", query.ScopeAll, "query1 w:query2"},
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
    result := query.RunWith(query.Parse("s:wor"), panes, nil)
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

func TestRun_AllScope_MultiMatch(t *testing.T) {
    panes := []tmux.Pane{
        {Session: "vim-session", WindowIndex: 0, WindowName: "editor", PanePID: 100},
        {Session: "work", WindowIndex: 0, WindowName: "vim-win", PanePID: 200},
    }
    result := query.RunWith(query.Parse("vim"), panes, nil)
    if len(result.Sessions) != 2 {
        t.Fatalf("expected 2 sessions, got %d", len(result.Sessions))
    }
}

func TestRun_MatchPos(t *testing.T) {
    panes := []tmux.Pane{
        {Session: "work", WindowIndex: 0, WindowName: "main", PanePID: 100},
    }
    result := query.RunWith(query.Parse("s:wok"), panes, nil)
    if len(result.Sessions) != 1 {
        t.Fatalf("expected 1 session, got %q", "work")
    }
    if len(result.Sessions[0].MatchPos) == 0 {
        t.Error("expected non-empty MatchPos for session match")
    }
}
