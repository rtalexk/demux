# demux v2 migration: alert system → state system

## Why the change

The alert system was a human-facing notification mechanism with no concept of what a tool was
actively doing. As demux is used increasingly with AI coding agents and long-running tools, a
richer lifecycle model was needed. The state system replaces alerts entirely, representing what
is happening in a pane rather than just that something needs attention.

## Removed and replaced CLI commands

| Removed | Replaced by |
|---|---|
| `demux alert set` | `demux state set` |
| `demux alert rm` | `demux state clear` |
| `demux alert ls` | `demux state ls` |
| `--level info\|warn\|error` | `--state working\|waiting\|done\|error` |
| `--level defer --sticky` | `--state flagged` |
| `--reason` | `--message` |

## Removed and replaced config

| Removed | Replaced by |
|---|---|
| `[alerts]` config section | removed, no replacement |
| `color_alert_*` theme tokens | `color_state_*` |

Old `color_alert_*` keys in `demux.toml` are silently ignored; missing `color_state_*` keys
fall back to defaults.

### New theme tokens

| Token | Usage |
|---|---|
| `color_state_working` | `working` text |
| `color_state_working_bg` | `working` background |
| `color_state_waiting` | `waiting` text |
| `color_state_waiting_bg` | `waiting` background |
| `color_state_done` | `done` text |
| `color_state_done_bg` | `done` background |
| `color_state_error` | `error` text |
| `color_state_error_bg` | `error` background |
| `color_state_flagged` | `flagged` text |
| `color_state_flagged_bg` | `flagged` background |

### Removed tokens

`color_alert_defer_sticky`, `color_alert_defer_sticky_bg` — no sticky concept in the new model.

## Hook script migration example

```bash
# before
demux alert set --target "$TARGET" --reason "Running: $TOOL_NAME" --level warn

# after
demux state set --target "$TARGET" --state working --tool claude \
  --message "Running: $TOOL_NAME" || true
```

## TUI keybindings

| Key | Action |
|---|---|
| `!` | Toggle attention filter (sessions with error/flagged/waiting/working states) |
| `X` | Clear state for the currently selected session |

### Attention filter

By default the `!` filter shows sessions with error, flagged, or waiting states. To also
include `working` sessions, add to `demux.toml`:

```toml
[tui]
attention_filter_include_working = true
```

## Write-lock semantics

A tool that calls `demux state set` on a target already owned by a different tool will receive
an error (`ErrStateLocked`) unless the existing state is `done` or `idle`. This prevents two
concurrent agents from clobbering each other's state. The `--force` flag overrides this
check when you need to reset manually.
