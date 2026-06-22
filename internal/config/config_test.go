package config_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rtalexk/demux/internal/config"
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
	if cfg.Log.Level != "warn" {
		t.Errorf("Log.Level default = %q, want \"warn\"", cfg.Log.Level)
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
	if cfg.Sidebar.FocusOnOpen != "" {
		t.Errorf("expected default Sidebar.FocusOnOpen=\"\", got %q", cfg.Sidebar.FocusOnOpen)
	}
}

func TestLoadFromFile_FocusOnOpen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demux.toml")
	os.WriteFile(path, []byte(`[sidebar]
focus_on_open = "current_session"`), 0644)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Sidebar.FocusOnOpen != "current_session" {
		t.Errorf("expected \"current_session\", got %q", cfg.Sidebar.FocusOnOpen)
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

func TestDefaults_SidebarShowLastSeen(t *testing.T) {
	cfg := config.Default()
	if !cfg.Sidebar.ShowLastSeen {
		t.Error("expected Sidebar.ShowLastSeen to default to true")
	}
}

func TestDefaults_SidebarShowSessionStats(t *testing.T) {
	cfg := config.Default()
	if !cfg.Sidebar.ShowSessionStats {
		t.Error("expected Sidebar.ShowSessionStats to default to true")
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

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "demux.toml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeTempConfig: %v", err)
	}
	return path
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

func TestDefaults_ActiveSessionIcon(t *testing.T) {
	cfg := config.Default()
	if cfg.Sidebar.ActiveSessionIcon != "►" {
		t.Errorf("expected ►, got %q", cfg.Sidebar.ActiveSessionIcon)
	}
}

func TestLoadFromFile_ActiveSessionIcon(t *testing.T) {
	path := writeTempConfig(t, `
[sidebar]
active_session_icon = "@"
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Sidebar.ActiveSessionIcon != "@" {
		t.Errorf("expected @, got %q", cfg.Sidebar.ActiveSessionIcon)
	}
}

func TestLoad_SidebarProcesses_ScalarMatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demux.toml")
	body := `
[[sidebar.processes]]
match = "claude*"
label = "🤖"

[[sidebar.processes]]
match = "node*"
label = "node"
fg = "#abcdef"
bg = "#101010"
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Sidebar.Processes) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(cfg.Sidebar.Processes))
	}
	if len(cfg.Sidebar.Processes[0].Match) != 1 || cfg.Sidebar.Processes[0].Match[0] != "claude*" || cfg.Sidebar.Processes[0].Label != "🤖" {
		t.Fatalf("entry 0 mismatch: %+v", cfg.Sidebar.Processes[0])
	}
	if cfg.Sidebar.Processes[1].FG != "#abcdef" || cfg.Sidebar.Processes[1].BG != "#101010" {
		t.Fatalf("entry 1 colours mismatch: %+v", cfg.Sidebar.Processes[1])
	}
}

func TestLoad_SidebarProcesses_ArrayMatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demux.toml")
	body := `
[[sidebar.processes]]
match = ["python*", "uvicorn", "uv"]
label = "py"
fg = "#1e1e2e"
bg = "#a6e3a1"
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Sidebar.Processes) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(cfg.Sidebar.Processes))
	}
	got := cfg.Sidebar.Processes[0]
	want := []string{"python*", "uvicorn", "uv"}
	if len(got.Match) != len(want) {
		t.Fatalf("expected %d globs, got %d: %+v", len(want), len(got.Match), got.Match)
	}
	for i, w := range want {
		if got.Match[i] != w {
			t.Fatalf("glob[%d] = %q, want %q", i, got.Match[i], w)
		}
	}
	if got.Label != "py" || got.FG != "#1e1e2e" || got.BG != "#a6e3a1" {
		t.Fatalf("entry mismatch: %+v", got)
	}
}

func TestLoad_SidebarProcesses_ArrayMatch_DropsInvalidGlobs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demux.toml")
	body := `
[[sidebar.processes]]
match = ["python*", "[", "uvicorn"]
label = "py"
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Sidebar.Processes) != 1 {
		t.Fatalf("expected entry kept, got %d", len(cfg.Sidebar.Processes))
	}
	got := cfg.Sidebar.Processes[0].Match
	want := []string{"python*", "uvicorn"}
	if len(got) != len(want) {
		t.Fatalf("expected bad glob filtered out, got %+v", got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Fatalf("glob[%d] = %q, want %q", i, got[i], w)
		}
	}
}

func TestLoad_SidebarProcesses_ArrayMatch_AllInvalidDropsEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demux.toml")
	body := `
[[sidebar.processes]]
match = ["[", "[bad"]
label = "py"

[[sidebar.processes]]
match = ["python*"]
label = "py"
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Sidebar.Processes) != 1 {
		t.Fatalf("expected only valid entry, got %d", len(cfg.Sidebar.Processes))
	}
}

func TestLoad_FiltersInvalidProcesses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demux.toml")
	body := `
[[sidebar.processes]]
match = ""
label = "noop"

[[sidebar.processes]]
match = "node*"
label = ""

[[sidebar.processes]]
match = "["
label = "broken"

[[sidebar.processes]]
match = "claude*"
label = "🤖"
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Sidebar.Processes) != 1 {
		t.Fatalf("expected only the valid entry, got %d: %+v", len(cfg.Sidebar.Processes), cfg.Sidebar.Processes)
	}
	if len(cfg.Sidebar.Processes[0].Match) != 1 || cfg.Sidebar.Processes[0].Match[0] != "claude*" {
		t.Fatalf("expected claude entry, got %+v", cfg.Sidebar.Processes[0])
	}
}

func TestLoad_ProcLabelThemeKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demux.toml")
	body := `
[theme]
color_proc_label_fg = "#111111"
color_proc_label_bg = "#222222"
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Theme.ColorProcLabelFG != "#111111" {
		t.Fatalf("fg mismatch: %q", cfg.Theme.ColorProcLabelFG)
	}
	if cfg.Theme.ColorProcLabelBG != "#222222" {
		t.Fatalf("bg mismatch: %q", cfg.Theme.ColorProcLabelBG)
	}
}

func TestDefaults_SidebarSessionView(t *testing.T) {
	c := config.Default()
	if c.Sidebar.SessionView != "row" {
		t.Errorf("expected SessionView=\"row\", got %q", c.Sidebar.SessionView)
	}
}

func TestLoadFromFile_SessionView_Card(t *testing.T) {
	path := writeTempConfig(t, "[sidebar]\nsession_view = \"card\"\n")
	c, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Sidebar.SessionView != "card" {
		t.Errorf("expected \"card\", got %q", c.Sidebar.SessionView)
	}
}

func TestLoadFromFile_SessionView_InvalidFallsBackToRow(t *testing.T) {
	path := writeTempConfig(t, "[sidebar]\nsession_view = \"banana\"\n")
	c, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Sidebar.SessionView != "row" {
		t.Errorf("expected fallback \"row\", got %q", c.Sidebar.SessionView)
	}
}

func TestLoadFromFile_SessionView_MissingDefaultsRow(t *testing.T) {
	path := writeTempConfig(t, "[sidebar]\nwidth = 40\n")
	c, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Sidebar.SessionView != "row" {
		t.Errorf("expected default \"row\", got %q", c.Sidebar.SessionView)
	}
}

func TestLoadFromFile_SessionView_InvalidWarnsOnStderr(t *testing.T) {
	path := writeTempConfig(t, "[sidebar]\nsession_view = \"banana\"\n")
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	if _, err := config.Load(path); err != nil {
		os.Stderr = origStderr
		t.Fatal(err)
	}
	w.Close()
	os.Stderr = origStderr
	buf, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(buf), `ignoring sidebar.session_view "banana"`) {
		t.Errorf("expected stderr warning, got %q", string(buf))
	}
}

func TestDefaults_CardSeparator(t *testing.T) {
	c := config.Default()
	if c.Sidebar.CardSeparator != config.CardSeparatorBlank {
		t.Errorf("expected CardSeparator default %q, got %q", config.CardSeparatorBlank, c.Sidebar.CardSeparator)
	}
}

func TestLoadFromFile_CardSeparator_Values(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"none", config.CardSeparatorNone},
		{"blank", config.CardSeparatorBlank},
		{"rule", config.CardSeparatorRule},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			path := writeTempConfig(t, "[sidebar]\ncard_separator = \""+tc.input+"\"\n")
			c, err := config.Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if c.Sidebar.CardSeparator != tc.expected {
				t.Errorf("got %q, want %q", c.Sidebar.CardSeparator, tc.expected)
			}
		})
	}
}

func TestLoadFromFile_CardSeparator_InvalidWarnsAndDefaults(t *testing.T) {
	path := writeTempConfig(t, "[sidebar]\ncard_separator = \"banana\"\n")
	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	c, err := config.Load(path)
	w.Close()
	os.Stderr = origStderr
	if err != nil {
		t.Fatal(err)
	}
	buf, _ := io.ReadAll(r)
	if !strings.Contains(string(buf), `ignoring sidebar.card_separator "banana"`) {
		t.Errorf("expected stderr warning, got %q", string(buf))
	}
	if c.Sidebar.CardSeparator != config.CardSeparatorBlank {
		t.Errorf("invalid value should fall back to %q, got %q", config.CardSeparatorBlank, c.Sidebar.CardSeparator)
	}
}

func TestLoadFromFile_SessionViewModeOverrides(t *testing.T) {
	path := writeTempConfig(t, `[sidebar]
session_view = "row"
session_view_mode_compact = "card"
session_view_mode_full = "row"
`)
	c, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Sidebar.SessionViewModeCompact != "card" {
		t.Errorf("compact override: got %q, want \"card\"", c.Sidebar.SessionViewModeCompact)
	}
	if c.Sidebar.SessionViewModeFull != "row" {
		t.Errorf("full override: got %q, want \"row\"", c.Sidebar.SessionViewModeFull)
	}
}

func TestLoadFromFile_SessionViewModeInvalidWarnsAndClears(t *testing.T) {
	path := writeTempConfig(t, "[sidebar]\nsession_view_mode_compact = \"banana\"\n")
	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	c, err := config.Load(path)
	w.Close()
	os.Stderr = origStderr
	if err != nil {
		t.Fatal(err)
	}
	buf, _ := io.ReadAll(r)
	if !strings.Contains(string(buf), `ignoring sidebar.session_view_mode_compact "banana"`) {
		t.Errorf("expected stderr warning, got %q", string(buf))
	}
	if c.Sidebar.SessionViewModeCompact != "" {
		t.Errorf("invalid override should clear to \"\", got %q", c.Sidebar.SessionViewModeCompact)
	}
}

func TestSidebarSticky_DefaultWidth(t *testing.T) {
	cfg := config.Default()
	if cfg.Sidebar.Sticky.Width != config.DefaultStickySidebarWidth {
		t.Errorf("default sticky width: want %d, got %d", config.DefaultStickySidebarWidth, cfg.Sidebar.Sticky.Width)
	}
}

func TestSidebarSticky_LoadOverridesDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demux.toml")
	body := "[sidebar.sticky]\nwidth = 50\n"
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Sidebar.Sticky.Width != 50 {
		t.Errorf("override sticky width: want 50, got %d", cfg.Sidebar.Sticky.Width)
	}
}

func TestSidebarSticky_LoadClampsBelowMinimum(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demux.toml")
	body := "[sidebar.sticky]\nwidth = 4\n"
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() { os.Stderr = origStderr }()
	cfg, err := config.Load(path)
	w.Close()
	out, _ := io.ReadAll(r)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Sidebar.Sticky.Width != config.MinStickySidebarWidth {
		t.Errorf("clamp: want %d, got %d", config.MinStickySidebarWidth, cfg.Sidebar.Sticky.Width)
	}
	if !strings.Contains(string(out), "sidebar.sticky.width") {
		t.Errorf("expected stderr warning mentioning sidebar.sticky.width, got: %s", out)
	}
}

func TestSidebarSticky_DefaultSlotsFalse(t *testing.T) {
	cfg := config.Default()
	if cfg.Sidebar.Sticky.Slots {
		t.Errorf("default slots: want false, got true")
	}
}

func TestSidebarSticky_LoadEnablesSlots(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demux.toml")
	body := "[sidebar.sticky]\nslots = true\n"
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Sidebar.Sticky.Slots {
		t.Errorf("slots: want true, got false")
	}
}

func TestResolvedSessionView(t *testing.T) {
	cases := []struct {
		name     string
		base     string
		compact  string
		full     string
		mode     string
		expected string
	}{
		{"default empty all", "", "", "", "full", "row"},
		{"base only", "card", "", "", "full", "card"},
		{"compact override beats base", "row", "card", "", "compact", "card"},
		{"full override beats base", "card", "", "row", "full", "row"},
		{"compact override ignored in full mode", "row", "card", "", "full", "row"},
		{"full override ignored in compact mode", "card", "", "row", "compact", "card"},
		{"unknown mode falls through to base", "card", "", "", "weird", "card"},
		{"unknown mode falls through to default", "", "", "", "weird", "row"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sc := config.SidebarConfig{
				SessionView:            tc.base,
				SessionViewModeCompact: tc.compact,
				SessionViewModeFull:    tc.full,
			}
			if got := sc.ResolvedSessionView(tc.mode); got != tc.expected {
				t.Errorf("ResolvedSessionView(mode=%q): got %q, want %q", tc.mode, got, tc.expected)
			}
		})
	}
}

func TestSidebarSticky_DefaultFocusFlags(t *testing.T) {
	cfg := config.Default()
	if !cfg.Sidebar.Sticky.FocusOnOpen {
		t.Errorf("default sticky focus_on_open: want true, got false")
	}
	if cfg.Sidebar.Sticky.FocusBeforeToggleClose {
		t.Errorf("default sticky focus_before_toggle_close: want false, got true")
	}
}

func TestSidebarSticky_LoadOverridesFocusFlags(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demux.toml")
	body := "[sidebar.sticky]\nfocus_on_open = false\nfocus_before_toggle_close = true\n"
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Sidebar.Sticky.FocusOnOpen {
		t.Errorf("focus_on_open override: want false, got true")
	}
	if !cfg.Sidebar.Sticky.FocusBeforeToggleClose {
		t.Errorf("focus_before_toggle_close override: want true, got false")
	}
}

func TestAutoShowDefaultsFalse(t *testing.T) {
	if config.Default().Sidebar.Sticky.AutoShow {
		t.Error("Sidebar.Sticky.AutoShow should default to false")
	}
}

func TestAutoShowParsesFromTOML(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/demux.toml"
	if err := os.WriteFile(path, []byte("[sidebar.sticky]\nauto_show = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Sidebar.Sticky.AutoShow {
		t.Error("auto_show = true did not parse into Sidebar.Sticky.AutoShow")
	}
}
