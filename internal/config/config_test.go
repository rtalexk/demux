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
    if cfg.RefreshIntervalMs != 3000 {
        t.Errorf("expected 3000, got %d", cfg.RefreshIntervalMs)
    }
    if cfg.Sidebar.Width != 35 {
        t.Errorf("expected 35, got %d", cfg.Sidebar.Width)
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
ignored_sessions = ["scratch"]

[sidebar]
width = 40

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
    if cfg.Sidebar.Width != 40 {
        t.Errorf("expected 40, got %d", cfg.Sidebar.Width)
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
    if cfg.RefreshIntervalMs != 3000 {
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

func TestDefaults_NewIconFields(t *testing.T) {
    cfg := config.Default()
    if cfg.Theme.IconTmuxSession != "⊞" {
        t.Errorf("expected ⊞, got %q", cfg.Theme.IconTmuxSession)
    }
    if cfg.Theme.IconCfgSession != "⚙︎" {
        t.Errorf("expected ⚙︎, got %q", cfg.Theme.IconCfgSession)
    }
}

func TestDefaults_DefaultFilter(t *testing.T) {
    cfg := config.Default()
    if cfg.Sidebar.DefaultFilter != "t" {
        t.Errorf("expected t, got %q", cfg.Sidebar.DefaultFilter)
    }
}

func TestLoadFromFile_DefaultFilter(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "demux.toml")
    os.WriteFile(path, []byte(`[sidebar]
default_filter = "a"`), 0644)
    cfg, err := config.Load(path)
    if err != nil {
        t.Fatal(err)
    }
    if cfg.Sidebar.DefaultFilter != "a" {
        t.Errorf("expected a, got %q", cfg.Sidebar.DefaultFilter)
    }
}

func TestDefaults_SidebarSort(t *testing.T) {
    cfg := config.Default()
    want := []string{"priority", "last_seen", "alphabetical"}
    if len(cfg.Sidebar.Sort) != len(want) {
        t.Fatalf("expected Sidebar.Sort=%v, got %v", want, cfg.Sidebar.Sort)
    }
    for i, v := range want {
        if cfg.Sidebar.Sort[i] != v {
            t.Errorf("Sidebar.Sort[%d]: expected %q, got %q", i, v, cfg.Sidebar.Sort[i])
        }
    }
}

func TestLoadFromFile_SidebarSort_SingleKey(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "demux.toml")
    os.WriteFile(path, []byte(`[sidebar]
sort = ["last_seen"]`), 0644)
    cfg, err := config.Load(path)
    if err != nil {
        t.Fatal(err)
    }
    want := []string{"last_seen", "priority", "alphabetical"}
    if len(cfg.Sidebar.Sort) != len(want) {
        t.Fatalf("expected %v, got %v", want, cfg.Sidebar.Sort)
    }
    for i, v := range want {
        if cfg.Sidebar.Sort[i] != v {
            t.Errorf("[%d]: expected %q, got %q", i, v, cfg.Sidebar.Sort[i])
        }
    }
}

func TestLoadFromFile_SidebarSort_PartialCustom(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "demux.toml")
    os.WriteFile(path, []byte(`[sidebar]
sort = ["alphabetical", "priority"]`), 0644)
    cfg, err := config.Load(path)
    if err != nil {
        t.Fatal(err)
    }
    want := []string{"alphabetical", "priority", "last_seen"}
    for i, v := range want {
        if cfg.Sidebar.Sort[i] != v {
            t.Errorf("[%d]: expected %q, got %q", i, v, cfg.Sidebar.Sort[i])
        }
    }
}

func TestLoadFromFile_SidebarSort_InvalidDropped(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "demux.toml")
    os.WriteFile(path, []byte(`[sidebar]
sort = ["bogus", "last_seen"]`), 0644)
    cfg, err := config.Load(path)
    if err != nil {
        t.Fatal(err)
    }
    // "bogus" dropped; "last_seen" first; "priority", "alphabetical" filled in
    want := []string{"last_seen", "priority", "alphabetical"}
    for i, v := range want {
        if cfg.Sidebar.Sort[i] != v {
            t.Errorf("[%d]: expected %q, got %q", i, v, cfg.Sidebar.Sort[i])
        }
    }
}

func TestLoadFromFile_SidebarSort_Empty(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "demux.toml")
    os.WriteFile(path, []byte(`[sidebar]
sort = []`), 0644)
    cfg, err := config.Load(path)
    if err != nil {
        t.Fatal(err)
    }
    want := []string{"priority", "last_seen", "alphabetical"}
    for i, v := range want {
        if cfg.Sidebar.Sort[i] != v {
            t.Errorf("[%d]: expected %q, got %q", i, v, cfg.Sidebar.Sort[i])
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
    if cfg.Sidebar.FocusOnOpen != "alert_session" {
        t.Errorf("expected default Sidebar.FocusOnOpen=\"alert_session\", got %q", cfg.Sidebar.FocusOnOpen)
    }
}

func TestLoadFromFile_FocusOnOpen(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "demux.toml")
    os.WriteFile(path, []byte(`[sidebar]
focus_on_open = "alert_window"`), 0644)
    cfg, err := config.Load(path)
    if err != nil {
        t.Fatal(err)
    }
    if cfg.Sidebar.FocusOnOpen != "alert_window" {
        t.Errorf("expected \"alert_window\", got %q", cfg.Sidebar.FocusOnOpen)
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

func TestDefaults_SidebarSwitchFocus(t *testing.T) {
    cfg := config.Default()
    if cfg.Sidebar.SwitchFocus != "severity" {
        t.Errorf("expected default Sidebar.SwitchFocus=\"severity\", got %q", cfg.Sidebar.SwitchFocus)
    }
}

func TestLoadFromFile_SidebarSwitchFocus(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "demux.toml")
    os.WriteFile(path, []byte(`[sidebar]
switch_focus = "newest"`), 0644)
    cfg, err := config.Load(path)
    if err != nil {
        t.Fatal(err)
    }
    if cfg.Sidebar.SwitchFocus != "newest" {
        t.Errorf("expected \"newest\", got %q", cfg.Sidebar.SwitchFocus)
    }
}

func TestDefault_Mode(t *testing.T) {
    cfg := config.Default()
    if cfg.Mode != "full" {
        t.Errorf("expected default Mode %q, got %q", "full", cfg.Mode)
    }
}

func TestLoad_ModeCompact(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "demux.toml")
    if err := os.WriteFile(path, []byte(`mode = "compact"`), 0644); err != nil {
        t.Fatal(err)
    }
    cfg, err := config.Load(path)
    if err != nil {
        t.Fatal(err)
    }
    if cfg.Mode != "compact" {
        t.Errorf("expected Mode %q, got %q", "compact", cfg.Mode)
    }
}

func TestDefault_StatusBarShow(t *testing.T) {
    cfg := config.Default()
    if !cfg.StatusBar.Show {
        t.Error("expected StatusBar.Show to default to true")
    }
}

func TestLoad_StatusBarShowFalse(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "demux.toml")
    if err := os.WriteFile(path, []byte("[status_bar]\nshow = false"), 0644); err != nil {
        t.Fatal(err)
    }
    cfg, err := config.Load(path)
    if err != nil {
        t.Fatal(err)
    }
    if cfg.StatusBar.Show {
        t.Error("expected StatusBar.Show = false when set in config")
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
