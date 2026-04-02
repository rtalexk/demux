# demux

**demux** is a session manager and dashboard for tmux. It allows you to move fast across tmux sessions, shows you what is running across all your sessions: processes, git branches, ports, all without leaving the terminal.

Configure your coding agent, or any other tool, to alert you when it needs attention.

![demux TUI](docs/assets/normal_view.png)

![demux compact mode](docs/assets/compact_view.png)

## Motivation

**TL;DR:** My entire workflow lives in the terminal. Each tmux session is a project or worktree. In this new era of agentic programming, nearly every session has a Claude instance running tasks for me, and keeping track of all of them became a real problem. demux is the result.

<details>

<summary>The longer, less exciting story</summary>

For over a year I relied on [Sesh](https://github.com/joshmedeski/sesh). Simply fantastic. It covered everything I needed: fast, frictionless navigation between tmux sessions.

Then everything changed when the agentic programming nation attacked. I adapted, leaned into AI for day-to-day work, and started running more tasks in parallel. But parallel work and ADHD are a terrible combination. I kept leaving Claude waiting on a response for hours at a time. Deeply inefficient.

I looked at a few tools ([Conductor](https://www.conductor.build/), [LazyAgent](https://github.com/illegalstudio/lazyagent), [opensessions](https://github.com/ataraxy-labs/opensessions)), but none of them fit my setup or the way I work: fast session switching via Sesh, plus visibility into which session actually needs my attention, without UX disruption or UI bloating.

I had already extended my personal CLI with commands to [manage git worktrees](https://github.com/rtalexk/dotfiles/tree/main/alx/cmd/worktree). I only discovered [Worktrunk](https://github.com/max-sixty/worktrunk) after the fact, and while my implementation is limited, it does what I need. I may switch eventually, but not any time soon.

_opensessions_ came closest to what I was looking for, and its premise overlaps significantly with mine. But Sesh's UX is too deeply wired into my muscle memory, and I was already mid-implementation anyway. What I needed was Sesh, but with alerts and process information.

Why not just configure Claude to send OS-level notifications? I hate notifications. Almost all of them are blocked on my phone, and only a handful are allowed on the desktop. They're disorganized, and they interrupt flow. What I actually want is: _tell me you need attention, and I'll get to you when I'm free._

### How demux fits into my workflow

When I pick up a ticket (Linear or GitHub issue):

```bash
alx wt add <name> [branch]
# alx wt add user-signup feat/PROJ-32/add-user-signup
```

This assumes a repository structure like:

```
./my-node-app
├── project.toml
├── .env
├── main/
├── dev/
├── feature-1/
├── feature-2/
└── user-signup/
```

Where `project.toml` looks like:

```toml
alias = "rem"
on_create = "npm install"
copy_files = [".env"]

[demux]
windows = ["editor", "shell"]
```

`alx wt add` reads that file, creates a worktree in the repo, spins up a tmux session named `<alias>-<worktree>` (e.g. `rem-user-signup`), copies any files listed in `copy_files`, runs the `on_create` command, and registers the session in demux's `private.toml` forwarding the `[demux]` block.

Claude knows this workflow. It uses my CLI to take tickets independently with worktrees, opens PRs, and notifies me via `demux alert set ...`.

demux will keep evolving as my workflow does. tmux and Neovim are constants ((Neo)?Vim for 15+ years, tmux for 5+). I don't expect to replace them any time soon.

</details>

## Features

- Jump fast across tmux sessions
- Live sidebar of tmux sessions with git status (branch, dirty, ahead/behind)
- Process list per session: CPU, memory, uptime, listening port, working directory
- Alert system: set info/warn/error alerts on any window or pane, pluggable on any tool
- Fuzzy search across sessions, windows, and processes
- Compact popup mode for use in a tmux split or popup
- Scriptable CLI: list sessions, procs, ports, alerts in text/table/json
- Tmux status bar integration via `demux status`
- Auto-clears alerts on pane focus via tmux hooks
- Fully themeable with a Catppuccin Mocha default
- Session config: define sessions with groups, labels, icons, and window templates

## Install

**Homebrew (macOS/Linux)**

```bash
brew install rtalexk/demux/demux
```

**Go**

```bash
go install github.com/rtalexk/demux@latest
```

## Quick Start

Launch the TUI:

```bash
demux
```

Launch in compact mode (useful as a tmux popup):

```bash
demux --compact
```

Set `DEMUX_POPUP=1` to make demux quit automatically after switching to a
session. Pair this with a tmux popup binding:

```tmux
bind-key K display-popup -E -w 80% -h 80% "DEMUX_POPUP=1 demux"
bind-key k display-popup -E -w 30% -h 80% "DEMUX_POPUP=1 demux --compact"
```

Start with the search input focused:

```bash
demux --search
```

## Key Bindings

| Key                 | Action                          |
| ------------------- | ------------------------------- |
| `j` / `k`           | Move down / up                  |
| `g` / `G`           | Jump to top / bottom            |
| `J` / `K`           | Jump down / up (large step)     |
| `h`                 | Focus sidebar                   |
| `l`                 | Focus process list              |
| `enter`             | Switch to session               |
| `o`                 | Open / attach to session        |
| `y`                 | Yank session name to clipboard  |
| `x`                 | Kill selected process           |
| `r`                 | Restart selected process        |
| `L`                 | View process log                |
| `R`                 | Force refresh                   |
| `t`                 | Filter: tmux sessions only      |
| `a`                 | Filter: all sessions            |
| `c`                 | Filter: config sessions only    |
| `w`                 | Filter: worktree sessions only  |
| `!`                 | Filter: sessions with alerts    |
| `tab` / `shift+tab` | Cycle focus                     |
| `[` / `]`           | Collapse / expand process group |
| `{` / `}`           | Collapse / expand all groups    |
| `?`                 | Toggle help overlay             |
| `q` / `ctrl+c`      | Quit                            |

Press `?` inside the TUI for the full interactive reference.

## Configuration

The default config path is `~/.config/demux/demux.toml`. Generate a
commented starting point with:

```bash
demux config init > ~/.config/demux/demux.toml
```

The generated file is fully commented and covers all available options.

### Sessions

Define sessions in `~/.config/demux/sessions.toml`. Sensitive entries can
go in `~/.config/demux/private.toml`, which is gitignore-friendly.

Add a session from the command line:

```bash
demux session config-add --name myproject --path ~/code/myproject
```

Or write it by hand:

```toml
[[session]]
name     = "myproject"          # must match the tmux session name
path     = "~/code/myproject"   # root directory of the session
group    = "work"               # optional group label in the sidebar
labels   = ["rust", "api"]      # optional tags
icon     = "⚙︎"                  # optional icon shown in the sidebar
worktree = false                # true if the path is a git worktree
windows  = ["editor", "term"]   # window templates to create on launch
```

### Window Templates

Window templates let you define reusable tmux window layouts. They live in
`sessions.toml` alongside your session entries.

```toml
[[window_templates]]
id               = "editor"         # referenced by [[session]].windows
name             = "editor"         # tmux window name
after_create_cmd = "nvim ."         # command to run after the window is created

[[window_templates]]
id               = "term"
name             = "terminal"
after_create_cmd = ""

# inherit from another template and override fields
[[window_templates]]
id   = "server"
from = "term"                       # copies name and after_create_cmd from "term"
name = "server"
after_create_cmd = "cargo run"
```

Create the windows for a session:

```bash
demux session create-windows --session myproject --windows editor,term,server
```

### Path Aliases

Shorten verbose paths displayed in the TUI. The longest matching prefix wins.

Before:

```
/Users/alex/code/myproject/src
```

After (with the alias below):

```
~/code/myproject/src
```

Config:

```toml
[[path_aliases]]
prefix  = "$HOME"   # supports environment variables
replace = "~"
```

### Theme

demux ships with a Catppuccin Mocha theme. All colors are configurable
under the `[theme]` section of `demux.toml`.

## Hooks

### tmux

demux can automatically clear alerts when you navigate between panes,
windows, or sessions. Print the hook configuration with:

```bash
demux hooks init --agent tmux
```

> **Note:** The `--agent` flag will be renamed in a future version.

Then paste the output into `~/.tmux.conf` and reload:

```bash
tmux source ~/.tmux.conf
```

This is my Tmux hooks configuration

```tmux
# Clears Demux alerts when switching between panes within the same window.
set-hook -g after-select-pane   "run-shell 'demux event pane_focus --target #{session_name}:#{window_index}.#{pane_index} 2>/dev/null; true'"

# Clears Demux alerts when switching windows (after-select-pane does not fire for window switches).
set-hook -g after-select-window "run-shell 'demux event pane_focus --target #{session_name}:#{window_index}.#{pane_index} 2>/dev/null; true'"

# Clears Demux alerts when switching sessions (after-select-window does not fire for session switches).
set-hook -g client-session-changed "run-shell 'demux event pane_focus --target #{session_name}:#{window_index}.#{pane_index} 2>/dev/null; true'"

# Clears Demux alerts when switching back from another application.
set-hook -g client-focus-in "run-shell 'demux event pane_focus --target #{session_name}:#{window_index}.#{pane_index} 2>/dev/null; true'"
```

### Tmux Status Bar

Add a live alert summary to your tmux status bar:

```tmux
set -g status-right "#(demux status)"
```

This outputs a colored count of active alerts (info, warn, error). When
there are no alerts it shows a green indicator.

### Claude

I have the following configuration at `~/.claude/settings.json`. This will make Claude send demux alerts whenever it pauses for a permission prompt, waits for input, or finishes a task, so your tmux status line always reflects what Claude is doing in each pane.

<details>

<summary>Claude Notification settings</summary>

```jsonc
{
  "hooks": {
    "Notification": [
      {
        "matcher": "permission_prompt",
        "hooks": [
          {
            "type": "command",
            "command": "demux alert set --target \"$(tmux display-message -t \"$TMUX_PANE\" -p '#S:#I.#P')\" --reason \"awaiting permission\" --level warn",
          },
        ],
      },
      {
        "matcher": "idle_prompt",
        "hooks": [
          {
            "type": "command",
            "command": "demux alert set --target \"$(tmux display-message -t \"$TMUX_PANE\" -p '#S:#I.#P')\" --reason \"awaiting input\" --level info",
          },
        ],
      },
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "demux alert set --target \"$(tmux display-message -t \"$TMUX_PANE\" -p '#S:#I.#P')\" --reason \"task complete\" --level info",
          },
        ],
      },
    ],
    "SubagentStop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "demux alert set --target \"$(tmux display-message -t \"$TMUX_PANE\" -p '#S:#I.#P')\" --reason \"subagent complete\" --level info",
          },
        ],
      },
    ],
    "StopFailure": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "demux alert set --target \"$(tmux display-message -t \"$TMUX_PANE\" -p '#S:#I.#P')\" --reason \"task failed\" --level error",
          },
        ],
      },
    ],
  },
}
```

</details>

## CLI Reference

```bash
demux                          # launch the TUI
demux --compact                # compact mode (sidebar + search only)
demux --search                 # start with search focused
demux --format text|table|json # output format for CLI commands
demux --log-level off|error|warn|info|debug

demux session list             # list all tmux sessions
demux session list --git       # include git columns
demux session list --git-only  # session + git columns only
demux session config-add       --name <n> --path <p> [--group <g>] [--labels <l>] [--worktree] [--private]
demux session config-get       --name <n>
demux session config-remove    --name <n> [--private]
demux session create-windows   --session <n> --windows <ids>

demux windows --session <n>    # list windows in a session
demux windows --session <n> --git

demux procs                    # list processes across all sessions
demux procs --session <n>      # filter to a session
demux procs --window <n:idx>   # filter to a window
demux procs --git              # include git column

demux ports                    # list all TCP listening ports

demux alert list               # list active alerts
demux alert set   --target <session:window[.pane]> --reason <text> [--level info|warn|error]
demux alert remove --target <session:window[.pane]>

demux query <term>             # fuzzy search sessions, windows, processes
demux query <term> --session-name-only  # output session names only (for fzf)

demux status                   # alert summary for tmux status bar
demux status --format json

demux event pane_focus         # clear alerts for the focused pane (used by hooks)

demux config init              # print default config to stdout
demux hooks init --agent tmux|claude
```

## Roadmap

- **AI coding agent integration:** Surface the state of running AI agents
  directly in the TUI.
- **Sticky sidebar mode:** A persistent sidebar that retains its position
  and selection as you switch between sessions.
- **Richer ports view:** Expand the `ports` command with process trees,
  protocol details, and per-session grouping.
- **Richer session rows:** Show more at a glance on each sidebar row.
- **Live config reload:** Pick up changes to `demux.toml` without
  restarting.
- **Per-pane environment variables:** Inspect the environment of any pane
  from the process list.

## Contributing

Open an issue to report a bug or propose a feature. Pull requests are not
being accepted yet; the project is still in its early stages. Starting a
discussion first is the best way to get something considered.

## Special thanks

- [Sesh](https://github.com/joshmedeski/sesh), by Josh Medesky. Be sure to check it out and leave some ❤️

## License

MIT. See [LICENSE](LICENSE).
