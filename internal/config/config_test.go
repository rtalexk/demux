package config_test

import (
    "os"
    "path/filepath"
    "strings"
    "testing"

    "github.com/rtalex/demux/internal/config"
)

func containsStr(s []string, v string) bool {
    for _, x := range s {
        if x == v {
            return true
        }
    }
    return false
}

func TestDefaults(t *testing.T) {
    cfg := config.Default()
    if cfg.RefreshIntervalMs != 2000 {
        t.Errorf("expected 2000, got %d", cfg.RefreshIntervalMs)
    }
    if cfg.SidebarWidth != 30 {
        t.Errorf("expected 30, got %d", cfg.SidebarWidth)
    }
    if cfg.Git.TimeoutMs != 500 {
        t.Errorf("expected 500, got %d", cfg.Git.TimeoutMs)
    }
    if cfg.Git.OnTimeout != "cached" {
        t.Errorf("expected cached, got %s", cfg.Git.OnTimeout)
    }
}

func TestDefaults_IgnoredProcesses(t *testing.T) {
    cfg := config.Default()
    expected := []string{"zsh", "bash", "fish", "sh", "dash", "nu", "pwsh"}
    if len(cfg.IgnoredProcesses) != len(expected) {
        t.Fatalf("expected %d ignored processes, got %d: %v", len(expected), len(cfg.IgnoredProcesses), cfg.IgnoredProcesses)
    }
    for i, v := range expected {
        if cfg.IgnoredProcesses[i] != v {
            t.Errorf("IgnoredProcesses[%d]: expected %q, got %q", i, v, cfg.IgnoredProcesses[i])
        }
    }
}

func TestDefaults_ProcessesConfig(t *testing.T) {
    procs := config.Default().Theme.Processes
    if len(procs.Editors) == 0 {
        t.Error("expected non-empty default editors list")
    }
    if len(procs.Agents) == 0 {
        t.Error("expected non-empty default agents list")
    }
    if len(procs.Servers) == 0 {
        t.Error("expected non-empty default servers list")
    }
    if len(procs.Shells) == 0 {
        t.Error("expected non-empty default shells list")
    }
    // spot-check key entries
    if !containsStr(procs.Editors, "nvim") {
        t.Error("expected nvim in default editors")
    }
    if !containsStr(procs.Agents, "claude") {
        t.Error("expected claude in default agents")
    }
    if !containsStr(procs.Servers, "node") {
        t.Error("expected node in default servers")
    }
    if !containsStr(procs.Shells, "zsh") {
        t.Error("expected zsh in default shells")
    }
}

func TestLoadFromFile(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "dmux.toml")
    os.WriteFile(path, []byte(`
refresh_interval_ms = 1000
sidebar_width = 40
ignored_sessions = ["scratch"]

[git]
enabled = false
timeout_ms = 250
`), 0644)

    cfg, err := config.Load(path)
    if err != nil {
        t.Fatal(err)
    }
    if cfg.RefreshIntervalMs != 1000 {
        t.Errorf("expected 1000, got %d", cfg.RefreshIntervalMs)
    }
    if cfg.SidebarWidth != 40 {
        t.Errorf("expected 40, got %d", cfg.SidebarWidth)
    }
    if len(cfg.IgnoredSessions) != 1 || cfg.IgnoredSessions[0] != "scratch" {
        t.Errorf("unexpected ignored_sessions: %v", cfg.IgnoredSessions)
    }
    if cfg.Git.Enabled {
        t.Error("expected git.enabled = false")
    }
}

func TestMissingFile(t *testing.T) {
    cfg, err := config.Load("/nonexistent/path/dmux.toml")
    if err != nil {
        t.Fatal(err)
    }
    if cfg.RefreshIntervalMs != 2000 {
        t.Errorf("expected defaults, got %d", cfg.RefreshIntervalMs)
    }
}

func TestLoadFromFile_IgnoredProcesses(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "demux.toml")
    os.WriteFile(path, []byte(`ignored_processes = ["bash", "zsh"]`), 0644)

    cfg, err := config.Load(path)
    if err != nil {
        t.Fatal(err)
    }
    if len(cfg.IgnoredProcesses) != 2 {
        t.Fatalf("expected 2 ignored processes, got %d: %v", len(cfg.IgnoredProcesses), cfg.IgnoredProcesses)
    }
    if cfg.IgnoredProcesses[0] != "bash" || cfg.IgnoredProcesses[1] != "zsh" {
        t.Errorf("unexpected ignored processes: %v", cfg.IgnoredProcesses)
    }
}

func TestPathAliases_EnvExpansion(t *testing.T) {
    t.Setenv("MYPROJECTS", "/home/user/projects")
    t.Setenv("PROJ_ALIAS", "proj")
    dir := t.TempDir()
    path := filepath.Join(dir, "demux.toml")
    os.WriteFile(path, []byte(`
[[path_aliases]]
prefix = "$MYPROJECTS"
replace = "$PROJ_ALIAS"

[[path_aliases]]
prefix = "$MYPROJECTS/work"
replace = "work"
`), 0644)

    cfg, err := config.Load(path)
    if err != nil {
        t.Fatal(err)
    }
    if len(cfg.PathAliases) != 2 {
        t.Fatalf("expected 2 aliases, got %d", len(cfg.PathAliases))
    }
    // longest prefix ("/home/user/projects/work") must come first after sort
    if cfg.PathAliases[0].Prefix != "/home/user/projects/work" {
        t.Errorf("expected longest prefix first, got %q", cfg.PathAliases[0].Prefix)
    }
    if cfg.PathAliases[1].Prefix != "/home/user/projects" {
        t.Errorf("expected shorter prefix second, got %q", cfg.PathAliases[1].Prefix)
    }
    if cfg.PathAliases[0].Replace != "work" {
        t.Errorf("unexpected replace for index 0: %q", cfg.PathAliases[0].Replace)
    }
    // replace values are also env-expanded
    if cfg.PathAliases[1].Replace != "proj" {
        t.Errorf("expected Replace to be env-expanded to %q, got %q", "proj", cfg.PathAliases[1].Replace)
    }
}

func TestPathAliases_EmptyPrefixDropped(t *testing.T) {
    t.Setenv("UNSET_VAR", "")
    dir := t.TempDir()
    path := filepath.Join(dir, "demux.toml")
    os.WriteFile(path, []byte(`
[[path_aliases]]
prefix = "$UNSET_VAR"
replace = "x"

[[path_aliases]]
prefix = "/real/path"
replace = "rp"
`), 0644)

    cfg, err := config.Load(path)
    if err != nil {
        t.Fatal(err)
    }
    if len(cfg.PathAliases) != 1 {
        t.Fatalf("expected empty-prefix alias to be dropped, got %d aliases", len(cfg.PathAliases))
    }
    if cfg.PathAliases[0].Prefix != "/real/path" {
        t.Errorf("unexpected alias: %+v", cfg.PathAliases[0])
    }
}

func TestDefaults_AlertFilterWindows(t *testing.T) {
    cfg := config.Default()
    if cfg.AlertFilterWindows != "all" {
        t.Errorf("expected default AlertFilterWindows=\"all\", got %q", cfg.AlertFilterWindows)
    }
}

func TestLoadFromFile_AlertFilterWindows(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "demux.toml")
    os.WriteFile(path, []byte(`alert_filter_windows = "alerts_only"`), 0644)
    cfg, err := config.Load(path)
    if err != nil {
        t.Fatal(err)
    }
    if cfg.AlertFilterWindows != "alerts_only" {
        t.Errorf("expected \"alerts_only\", got %q", cfg.AlertFilterWindows)
    }
}

func TestDefaults_SessionSort(t *testing.T) {
    cfg := config.Default()
    want := []string{"priority", "last_seen", "alphabetical"}
    if len(cfg.SessionSort) != len(want) {
        t.Fatalf("expected SessionSort=%v, got %v", want, cfg.SessionSort)
    }
    for i, v := range want {
        if cfg.SessionSort[i] != v {
            t.Errorf("SessionSort[%d]: expected %q, got %q", i, v, cfg.SessionSort[i])
        }
    }
}

func TestLoadFromFile_SessionSort_SingleKey(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "demux.toml")
    os.WriteFile(path, []byte(`session_sort = ["last_seen"]`), 0644)
    cfg, err := config.Load(path)
    if err != nil {
        t.Fatal(err)
    }
    want := []string{"last_seen", "priority", "alphabetical"}
    if len(cfg.SessionSort) != len(want) {
        t.Fatalf("expected %v, got %v", want, cfg.SessionSort)
    }
    for i, v := range want {
        if cfg.SessionSort[i] != v {
            t.Errorf("[%d]: expected %q, got %q", i, v, cfg.SessionSort[i])
        }
    }
}

func TestLoadFromFile_SessionSort_PartialCustom(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "demux.toml")
    os.WriteFile(path, []byte(`session_sort = ["alphabetical", "priority"]`), 0644)
    cfg, err := config.Load(path)
    if err != nil {
        t.Fatal(err)
    }
    want := []string{"alphabetical", "priority", "last_seen"}
    for i, v := range want {
        if cfg.SessionSort[i] != v {
            t.Errorf("[%d]: expected %q, got %q", i, v, cfg.SessionSort[i])
        }
    }
}

func TestLoadFromFile_SessionSort_InvalidDropped(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "demux.toml")
    os.WriteFile(path, []byte(`session_sort = ["bogus", "last_seen"]`), 0644)
    cfg, err := config.Load(path)
    if err != nil {
        t.Fatal(err)
    }
    // "bogus" dropped; "last_seen" first; "priority", "alphabetical" filled in
    want := []string{"last_seen", "priority", "alphabetical"}
    for i, v := range want {
        if cfg.SessionSort[i] != v {
            t.Errorf("[%d]: expected %q, got %q", i, v, cfg.SessionSort[i])
        }
    }
}

func TestLoadFromFile_SessionSort_Empty(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "demux.toml")
    os.WriteFile(path, []byte(`session_sort = []`), 0644)
    cfg, err := config.Load(path)
    if err != nil {
        t.Fatal(err)
    }
    want := []string{"priority", "last_seen", "alphabetical"}
    for i, v := range want {
        if cfg.SessionSort[i] != v {
            t.Errorf("[%d]: expected %q, got %q", i, v, cfg.SessionSort[i])
        }
    }
}

func TestPathAliases_BackslashEscapedSpace(t *testing.T) {
    t.Setenv("MY_DIR", `/home/user/some\ dir`)
    dir := t.TempDir()
    path := filepath.Join(dir, "demux.toml")
    os.WriteFile(path, []byte(`
[[path_aliases]]
prefix = "$MY_DIR"
replace = "mydir"
`), 0644)

    cfg, err := config.Load(path)
    if err != nil {
        t.Fatal(err)
    }
    if len(cfg.PathAliases) != 1 {
        t.Fatalf("expected 1 alias, got %d", len(cfg.PathAliases))
    }
    if cfg.PathAliases[0].Prefix != "/home/user/some dir" {
        t.Errorf("unexpected prefix: %q", cfg.PathAliases[0].Prefix)
    }
}

func TestDefaults_FocusOnOpen(t *testing.T) {
    cfg := config.Default()
    if cfg.FocusOnOpen != "current_window" {
        t.Errorf("expected default FocusOnOpen=\"current_window\", got %q", cfg.FocusOnOpen)
    }
}

func TestLoadFromFile_FocusOnOpen(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "demux.toml")
    os.WriteFile(path, []byte(`focus_on_open = "alert_window"`), 0644)
    cfg, err := config.Load(path)
    if err != nil {
        t.Fatal(err)
    }
    if cfg.FocusOnOpen != "alert_window" {
        t.Errorf("expected \"alert_window\", got %q", cfg.FocusOnOpen)
    }
}

func TestDefaults_FocusOnOpenFallback(t *testing.T) {
    cfg := config.Default()
    if cfg.FocusOnOpenFallback != "current_window" {
        t.Errorf("expected default FocusOnOpenFallback=\"current_window\", got %q", cfg.FocusOnOpenFallback)
    }
}

func TestLoadFromFile_FocusOnOpenFallback(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "demux.toml")
    os.WriteFile(path, []byte(`focus_on_open_fallback = "first_window"`), 0644)
    cfg, err := config.Load(path)
    if err != nil {
        t.Fatal(err)
    }
    if cfg.FocusOnOpenFallback != "first_window" {
        t.Errorf("expected \"first_window\", got %q", cfg.FocusOnOpenFallback)
    }
}

func TestDefaultPath_ContainsExpectedSuffix(t *testing.T) {
    path, err := config.DefaultPath()
    if err != nil {
        t.Skipf("UserHomeDir not available: %v", err)
    }
    const suffix = ".config/demux/demux.toml"
    if !strings.HasSuffix(path, suffix) {
        t.Errorf("DefaultPath() = %q, want suffix %q", path, suffix)
    }
}

func TestDefaults_AlertSwitchPriority(t *testing.T) {
    cfg := config.Default()
    if cfg.AlertSwitchPriority != "severity" {
        t.Errorf("expected default AlertSwitchPriority=\"severity\", got %q", cfg.AlertSwitchPriority)
    }
}

func TestLoadFromFile_AlertSwitchPriority(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "demux.toml")
    os.WriteFile(path, []byte(`alert_switch_priority = "newest"`), 0644)
    cfg, err := config.Load(path)
    if err != nil {
        t.Fatal(err)
    }
    if cfg.AlertSwitchPriority != "newest" {
        t.Errorf("expected \"newest\", got %q", cfg.AlertSwitchPriority)
    }
}

func TestLoadFromFile_ProcessesConfig(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "demux.toml")
    os.WriteFile(path, []byte(`
[theme.processes]
editors = ["hx"]
agents  = ["aider"]
servers = ["bun"]
shells  = ["fish"]
`), 0644)

    cfg, err := config.Load(path)
    if err != nil {
        t.Fatal(err)
    }
    procs := cfg.Theme.Processes
    if len(procs.Editors) != 1 || procs.Editors[0] != "hx" {
        t.Errorf("unexpected editors: %v", procs.Editors)
    }
    if len(procs.Agents) != 1 || procs.Agents[0] != "aider" {
        t.Errorf("unexpected agents: %v", procs.Agents)
    }
    if len(procs.Servers) != 1 || procs.Servers[0] != "bun" {
        t.Errorf("unexpected servers: %v", procs.Servers)
    }
    if len(procs.Shells) != 1 || procs.Shells[0] != "fish" {
        t.Errorf("unexpected shells: %v", procs.Shells)
    }
}
