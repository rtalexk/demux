package config

import (
    "fmt"
    "regexp"
    "strings"
)

// ValidationIssue represents a single config problem found by Validate.
type ValidationIssue struct {
    Field   string // TOML key path, e.g. "theme.color_bg"
    Level   string // "error" or "warn"
    Message string
}

func (v ValidationIssue) String() string {
    return fmt.Sprintf("[%s] %s: %s", v.Level, v.Field, v.Message)
}

var hexColorRE = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Validate checks the Config for structural problems and returns all issues found.
// An empty slice means the config is valid. Errors prevent correct operation;
// warns indicate degraded behavior or non-obvious defaults.
func (c Config) Validate() []ValidationIssue {
    var issues []ValidationIssue
    add := func(level, field, msg string) {
        issues = append(issues, ValidationIssue{Field: field, Level: level, Message: msg})
    }

    // RefreshIntervalMs
    if c.RefreshIntervalMs < MinRefreshIntervalMs {
        add("error", "refresh_interval_ms", fmt.Sprintf("must be ≥ 100, got %d", c.RefreshIntervalMs))
    }

    // Mode
    validModes := map[string]bool{"full": true, "compact": true}
    if c.Mode != "" && !validModes[c.Mode] {
        add("error", "mode", fmt.Sprintf("must be \"full\" or \"compact\", got %q", c.Mode))
    }

    // DefaultFormat
    validFormats := map[string]bool{"text": true, "table": true, "json": true, "": true}
    if !validFormats[c.DefaultFormat] {
        add("error", "default_format", fmt.Sprintf("must be text|table|json, got %q", c.DefaultFormat))
    }

    // Log level
    validLevels := map[string]bool{"off": true, "error": true, "warn": true, "info": true, "debug": true}
    if c.Log.Level != "" && !validLevels[strings.ToLower(c.Log.Level)] {
        add("error", "log.level", fmt.Sprintf("must be off|error|warn|info|debug, got %q", c.Log.Level))
    }

    // Sidebar
    if c.Sidebar.Width < MinSidebarWidth {
        add("error", "sidebar.width", fmt.Sprintf("must be ≥ 10, got %d", c.Sidebar.Width))
    }

    // Git timeout
    if c.Git.TimeoutMs > 0 && c.Git.TimeoutMs < MinGitTimeoutWarnMs {
        add("warn", "git.timeout_ms", fmt.Sprintf("very low timeout (%dms) may cause frequent git errors", c.Git.TimeoutMs))
    }

    // Theme colors
    colorFields := map[string]string{
        "color_bg":       c.Theme.ColorBg,
        "color_surface":  c.Theme.ColorSurface,
        "color_raised":   c.Theme.ColorRaised,
        "color_selected": c.Theme.ColorSelected,
        "color_border":   c.Theme.ColorBorder,

        "color_fg_primary": c.Theme.ColorFgPrimary,
        "color_fg_subtext": c.Theme.ColorFgSubtext,
        "color_fg_muted":   c.Theme.ColorFgMuted,
        "color_fg_dim":     c.Theme.ColorFgDim,
        "color_fg_ghost":   c.Theme.ColorFgGhost,

        "color_session":     c.Theme.ColorSession,
        "color_proc_claude": c.Theme.ColorProcClaude,
        "color_proc_server": c.Theme.ColorProcServer,
        "color_proc_editor": c.Theme.ColorProcEditor,
        "color_proc_child":  c.Theme.ColorProcChild,

        "color_git_dirty":  c.Theme.ColorGitDirty,
        "color_git_behind": c.Theme.ColorGitBehind,
        "color_git_ahead":  c.Theme.ColorGitAhead,

        "color_alert_info":     c.Theme.ColorAlertInfo,
        "color_alert_warn":     c.Theme.ColorAlertWarn,
        "color_alert_error":    c.Theme.ColorAlertError,
        "color_alert_defer":    c.Theme.ColorAlertDefer,
        "color_alert_info_bg":  c.Theme.ColorAlertInfoBg,
        "color_alert_warn_bg":  c.Theme.ColorAlertWarnBg,
        "color_alert_error_bg": c.Theme.ColorAlertErrorBg,
        "color_alert_defer_bg": c.Theme.ColorAlertDeferBg,

        "color_port":               c.Theme.ColorPort,
        "color_port_bg":            c.Theme.ColorPortBg,
        "color_clean":              c.Theme.ColorClean,
        "color_cpu_low":            c.Theme.ColorCpuLow,
        "color_cpu_med":            c.Theme.ColorCpuMed,
        "color_cpu_high":           c.Theme.ColorCpuHigh,
        "color_fg_search_highlight": c.Theme.ColorFgSearchHighlight,
    }
    for field, val := range colorFields {
        if val != "" && !hexColorRE.MatchString(val) {
            add("warn", field, fmt.Sprintf("expected #rrggbb hex color, got %q", val))
        }
    }

    return issues
}
