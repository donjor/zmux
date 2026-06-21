# zmux

An opinionated, all-in-one tmux management wrapper. Session picker, theming,
status bar presets, and a popup dashboard — in a single binary.

## Features

- **Workspace-primary picker** — single flat list of workspaces with inline session expansion, fuzzy search, ghost tab completion, matched-char underlines
- **Dashboard** — 6-tab popup (prefix+Space): Session & Workspace, Workspaces, Themes, Bar, Settings, Help
- **Command palette** — spotlight-style quick actions (prefix+p)
- **Theming** — 300+ themes (iterm2-color-schemes format), semantic palette, color swatches
- **Theme sync** — pull your theme from Ghostty or Neovim
- **Workspaces** — first-class project containers grouping sessions
- **Status bar** — 9 presets with dynamic segments (git, lang, workspace, directory)
- **Recipes** — declarative TOML launch plans for workspaces, sessions, tabs, and commands
- **Terminal commands** — run, watch, send, type for agent/scripting workflows
- **Logical tabs** — tabs can run full-screen, ride as panes, or park hidden while staying addressable
- **Attention states** — running/done/failed/needs-human glyphs in the bar
- **Tab reaper** — auto-expires idle agent-task tabs by lifecycle policy; live work and `--keep`/daemon tabs are spared
- **Multi-source discovery** — find sessions across tmux sockets and overmind
- **Init wizard** — interactive TUI setup
- **Shell completions** — bash, zsh, fish

## Requirements

- tmux >= 3.2
- Linux or macOS

## Install

```bash
git clone https://github.com/donjor/zmux.git
cd zmux
./install.sh
```

The installer:
1. Checks dependencies (Go, tmux >= 3.2)
2. Builds the binary
3. Installs to `~/.local/bin/zmux`
4. Optionally adds shell integration (auto-start on terminal open)
5. Offers to run the `zmux init` setup wizard

### Updating

After pulling new changes:

```bash
make install        # build + copy to ~/.local/bin/zmux
```

This only rebuilds the binary — no config prompts, no shell changes.

### Manual install

If you prefer to do it yourself:

```bash
make build          # builds ./zmux
make install        # copies to ~/.local/bin/zmux
zmux init           # interactive setup wizard (first time only)
```

### Legacy v0 (bash+gum)

The original bash prototype lives under [`legacy/v0/`](legacy/v0/) — preserved but unsupported:

```bash
./legacy/v0/install.sh    # links zmux0 to ~/.local/bin/zmux0
```

See [`legacy/v0/README.md`](legacy/v0/README.md) for details and migration notes.

## Quick start

After install, run the setup wizard:

```bash
zmux init
```

This creates `~/.zmux.toml`, generates `~/.tmux.conf`, and sets up
`~/.zmux/` directories. Restart tmux or `prefix+r` to reload.

To apply theme + bar to a running tmux without the wizard:

```bash
zmux apply
```

After `zmux init`, restart tmux or `prefix+r` to reload config.

## Usage

### Session & Workspace Management

```
zmux                                Workspace picker (outside tmux) / dashboard (inside)

zmux new                            Create tmp-N session (no workspace)
zmux new <ws>                       Create workspace + 'main' session, attach
zmux new <ws> <session>             Create workspace (if needed) + local session label, attach
zmux new <ws> <s1> <s2> <s3>        Variadic — create workspace + multiple sessions
zmux open <ws> [session]            Open workspace/local session (aliases: attach, a)
zmux open <ws>/<session>            Same target grammar as command flags

zmux run <recipe>                   Open the recipe form with defaults
zmux run <recipe> -y                Run defaults without prompting
zmux run <recipe> --dry-run         Print the exact recipe plan
zmux run --command <cmd>            Force command mode on name collisions
zmux run <recipe> --tab-mode ready  Type commands into tabs without pressing Enter
zmux recipe list                    List bundled and user recipes
zmux recipe show <recipe>           Show recipe TOML
zmux recipe lint [recipe...]        Validate recipes
zmux recipe fork <recipe>           Copy a bundled recipe for editing
zmux recipe edit <recipe>           Edit a user recipe

zmux kill <name>                    Smart kill — workspace-first, then session (confirms if live)
zmux ls                             List workspaces (workspace-primary)
zmux ls <ws>                        List local session labels within a workspace
zmux ls -s                          Flat session list (legacy/debug)
zmux tabs [workspace/session]       List tabs in session (riders nested, hidden marked ~)
zmux where                          Current context: workspace/session/tab/pane/cwd (alias: whoami)
zmux where --json                   Same, as JSON (for tooling)

zmux tab move <tab> <dest>          Move tab to another session
zmux tab label [label]              Set/clear stable label for current tab
zmux tab state <state> [tab]        Set lifecycle glyph: attention/running/done/failed/clear
zmux tab pane <tab> [--into host]   Join a tab as a pane beside another tab
zmux tab full [tab]                 Promote focused/named pane-of tab back to full
zmux tab hide <tab>                 Park a tab off the bar in the hidden dock
zmux tab show <tab>                 Return a hidden tab to its origin session
zmux tab kill <tab>                 Kill a tab
zmux reap                           Adopt/flag/kill stale tabs by lifecycle policy
zmux reap --dry-run                 Preview verdicts only — change nothing
zmux session kill <session>         Kill a session
zmux session run <s> -n <t> -- <cmd>  Create a detached local session label, run <cmd> as first tab
zmux workspace list                 List workspaces (alias: zmux ws)
zmux workspace kill <workspace>     Kill a workspace and all its sessions
zmux workspace show <ws>            Show workspace sessions
```

### Terminal Commands

Run commands in named tmux windows, read their output, and send keystrokes.
Designed for scripting and agent workflows (see [Agent Integration](#agent-integration)).
When a command needs a session outside the current workspace, pass
`--session <workspace>/<session>`.

```
zmux run '<cmd>' -n <tab>      Run command, wait for completion
zmux run '<cmd>' -n <tab> -d   Run detached (for servers)
zmux run '<cmd>' -n <tab> -f   Run + follow output
zmux run '<cmd>' -n <tab> --keep         Exempt the tab from auto-reaping
zmux run '<cmd>' -n <tab> --scope daemon Long-lived tab — never auto-reaped
zmux watch <tab>               Capture tab output
zmux watch <tab> --until <pat> Wait for pattern match
zmux watch <tab> --idle 3      Wait until the visible screen is quiet for 3s
zmux watch <tab> -f            Follow output (tail -f)
zmux log start <tab>           Record tab output to a bounded file (background)
zmux log start <tab> --ansi    Keep ANSI colour instead of stripping to plain
zmux log status                List tabs currently being recorded
zmux log tail <tab>            Print a tab's recorded log
zmux log stop <tab>            Stop recording
zmux send <tab> <keys>         Send keystrokes to tab
zmux type <tab> '<text>'       Type text + Enter

zmux pane open <name> -r 40 -- <cmd>  Open right pane, print pane id
zmux pane open --label-tab ...        Preserve tab label before sidecar split
zmux pane toggle <name> -r 40 -- <cmd> Toggle named pane (close/open)
zmux pane current [--json]            Print current pane id/details
zmux pane list / zmux panes           List panes in current window
zmux pane focus <pane>                Focus pane by id/title/index
zmux pane resize <pane> --size 40%    Resize pane width
zmux pane close <pane>                Close pane by id/title/index
zmux terminal refresh                 Reattach client to refresh RGB features
zmux snapshot                         Capture pane text/ANSI + optional PNG evidence
zmux snapshot --no-png                Capture text/ANSI only
```

### Theming

```
zmux theme                     Interactive theme picker
zmux theme set <name>          Set theme directly
zmux theme list                List available themes
zmux theme sync                Pull theme from default sync target
zmux theme pull <target>       Pull theme from ghostty or nvim
```

### Configuration

```
zmux bar                       List bar presets with ANSI previews (live carousel inside tmux)
zmux bar <preset>              Set preset directly
zmux bar show                  Show current preset with preview
zmux init                      Setup wizard (run outside tmux)
zmux apply                     Regenerate + apply config
zmux refresh                   Apply config + refresh current client
zmux status                    Show current config summary
```

### Other

```
zmux version                   Print version
zmux completion <shell>        Generate completions (bash/zsh/fish)
zmux help                      Styled help with keybindings
```

## Keybindings

Prefix: `Ctrl+Space` (configurable)

### tmux prefix

| Key | Action |
|-----|--------|
| prefix + Space | Dashboard |
| prefix + p | Command palette |
| prefix + ! | Scratch shell popup ($SHELL, cwd from active pane) |
| prefix + d | Detach |
| prefix + ? | Help popup |
| prefix + c | New tab |
| prefix + n / N | Next / previous tab |
| prefix + < / > | Move tab left / right |
| prefix + x | Close tab (with confirm) |
| prefix + J | Join a tab into this tab as a pane |
| prefix + F | Promote focused pane-tab to full tab |
| prefix + R | Respawn stopped/dead pane |
| prefix + . | Label tab (blank clears label) |
| prefix + , | Rename session |
| prefix + C | New session in current workspace |
| prefix + w | Workspace session picker |
| prefix + [ / ] | Prev / next session in workspace |
| prefix + Alt+1-9 | Switch to session N in workspace |
| prefix + r | Reload config (zmux apply) |
| prefix + v | Enter copy mode (vi keys) |
| prefix + P | Paste buffer |
| prefix + ← / → / ↑ / ↓ | Focus pane in direction (tmux default) |
| prefix + Ctrl+← / → / ↑ / ↓ | Resize pane by one cell (tmux default) |
| prefix + Alt+← / → / ↑ / ↓ | Resize pane by five cells (tmux default) |
| prefix + q | Show pane numbers/ids (tmux default) |
| prefix + z | Toggle pane zoom (tmux default) |
| prefix + o / ; | Next / previous pane (tmux default) |
| prefix + % / " | Split pane right / below (tmux default) |
| Alt+1-9 | Switch to tab (no prefix) |
| Alt+w | Workspace switcher (no prefix) |
| Alt+Shift+← / → / ↑ / ↓ | Focus pane in direction (no prefix) |
| Alt+` | Tab switcher (no prefix) |

Pane notes: mouse is enabled, so clicking focuses panes and dragging pane
borders resizes them. Failed or signalled foreground commands stay visible as
dead panes, so Ctrl+C spam cannot silently delete the tab; clean exits close
normally. Use `prefix+x` / `zmux tab kill` when you mean to close a stopped tab.
Split windows render
pane-border headers with active pane id, title, command, size, and the
`A-S arrows` focus hint; inactive panes stay
subtle with index/title only. Split indicators and active-border color make the
focused pane visible around the divider/bottom edge while pane backgrounds stay
transparent/default. Single-pane windows keep the border header blank.
Auto-named tabs normally use tmux's command name (`pi`, `bash`, etc.). When
multiple tabs in the same session share that name, zmux marks them as
`name[cwd]` in the bar (for example `pi[zmux]`) with the cwd suffix dimmed.
The current pane's working directory renders as a shortened, right-aligned
overlay on the top row, so cwd changes do not push the logical tabs around.
`zmux panes` lists the current window by default;
use `zmux panes --session` for all panes in the current/target session or
`zmux panes --all` for every session. `zmux tab label [label]` sets a stable
zmux label overlay for a tab while preserving tmux's automatic window name;
blank label clears it, and labeled tabs render as `label [auto-name]`. For
sidecars, `zmux pane open --label-tab ...` snapshots the current tab name as
a label before the sidecar pane can change tmux's auto-name.
`zmux terminal current --json` resolves the visible desktop terminal window
for the invoking tmux client so visual tooling can safely capture screenshots;
see [docs/terminal-current.md](docs/terminal-current.md). `zmux terminal
capabilities` diagnoses the outer tmux client color path and reports whether
RGB/truecolor is active; `zmux refresh` applies zmux config and replaces the
current tmux client with a freshly attached RGB-capable client when capabilities
were changed without a manual detach/reattach. zmux configures
`xterm-256color`, `xterm-ghostty`, and nested `tmux-256color` clients for RGB
so Ghostty panes can render smooth truecolor gradients. Local Ghostty passes
terminal pane keys through, and Hyprland uses `Super+...` window-management
bindings, so the pane keys do not collide with the terminal or WM setup.

### Picker (outside tmux)

| Key | Action |
|-----|--------|
| ↑ / ↓ | Navigate workspaces and sessions (tree traversal) |
| enter (top action) | Create tmp session (empty input) or workspace+main (typed) |
| enter (workspace) | Drill into sessions, or create default session if empty |
| enter (session) | Attach |
| tab | Accept ghost autocompletion |
| ctrl+x | Delete workspace or session under cursor (with confirm) |
| ctrl+h | Toggle hide-empty workspaces |
| 1-9 | Quick-select session by index |
| esc | Clear query, or quit if empty |
| ctrl+c | Quit |

## Configuration

`~/.zmux.toml`

```toml
theme = "ayu-dark"
prefix = "C-Space"

[bar]
preset = "default"
layout = "two-line"
indicator = "dots"
top_bar = "tabs"

[bar.segments]
workspace = true
git = true
lang = true
clock = true
directory = true
process = true
group = true

[sessions]
auto_cleanup_tmp = true

[recipes]
paths = ["~/.zmux/recipes"]

[sync]
target = "ghostty"          # ghostty, nvim, or none
ghostty_config = "auto"
```

## Themes

zmux uses the [iterm2-color-schemes](https://github.com/mbadolato/iTerm2-Color-Schemes)
format — the same upstream used by Ghostty, Alacritty, Kitty, and others.

**11 bundled themes** ship with the binary (no downloads needed):
ayu-dark, atom-one-dark, carbonfox, catppuccin-mocha, dracula, gruvbox-dark,
kanagawa-dragon, material-darker, nord, rose-pine, tokyonight.

**Theme resolution order:**
1. `~/.zmux/themes/<name>` — your custom themes
2. Bundled (embedded in binary)
3. `~/.zmux/themes/iterm2/<name>` — downloaded set from `zmux init`

### Theme sync

zmux can pull your theme from another tool (read-only, never writes to their config):

```bash
zmux theme sync              # pull from default target
zmux theme pull ghostty      # read Ghostty's theme
zmux theme pull nvim         # read Neovim's colorscheme
```

## Recipes

Declarative TOML files that define launch plans:

```toml
name = "dev"
description = "Project workspace: bash, dev server, git"
context = "outside"
kind = "session"
workspace = "{{ cwd_name | slug }}"
session = "{{ session | slug }}"

[defaults]
session = "main"
tab_mode = "run"

[[tabs]]
name = "bash"
command = "code ."

[[tabs]]
name = "dev"
command = "bun run dev"

[[tabs]]
name = "git"
command = "git status"

[options]
focus_tab = "bash"
```

Place custom recipes in `~/.zmux/recipes/`. Bundled recipes include
`dev`, `dev-nvim`, `dev-cc`, `dev-codex`, `claude`, `webdev`, `monitor`,
`shell-fanout`, `worktree-shell`, and `showcase`.

## Status bar presets

| Preset | Description |
|--------|-------------|
| default | Catppuccin-inspired rounded pills, icons, elevated surfaces |
| minimal | Clean, barely decorated, content-first |
| powerline | Angled separators, filled segments, directory chain |
| blocks | Square bracket segments, monospace, dense |
| rounded | Elevated pill segments, premium feel |
| hacker | Matrix-inspired, monospace, dense info |
| zen | Ultra-minimal, barely there |
| starship | Colorful prompt-inspired, each segment its own color |
| rpowerline | Rounded powerline — angled fills with rounded caps |

All presets show dynamic segments: git branch/dirty/ahead-behind, language
version, workspace with session position (e.g. `myapp 2/4`), directory,
active process, and group indicator. Segments are individually toggleable
in `[bar.segments]`.

Preview them: `zmux bar` (live carousel inside tmux, static ANSI outside)

## Agent Integration

zmux includes terminal commands (`run`, `watch`, `send`, `type`) designed for
AI agent workflows. These let agents manage long-running runtimes without
blocking their own shell or hiding process state.

**Key principles:**
- Use normal agent shell tools for bounded one-shot checks.
- Never run servers/watchers/long-lived runtimes in the agent shell — use zmux.
- Never use `&`, `nohup`, or `disown` — use a stable zmux tab.
- Read output with `zmux watch`, don't start duplicate processes to check state.
- For sudo/interactive commands, use `zmux type admin 'sudo ...'` or Pi's typed
  `zmux_interactive_type` tool.

**Example workflow:**
```bash
zmux run 'npm test' -n test --session app/main          # waits for completion
zmux run 'npm run dev' -n server -d --session app/main   # detach for runtimes
zmux watch server --session app/main --until "listening" # wait for ready signal
zmux watch server --session app/main -l 20               # peek at output
zmux send server C-c --session app/main                  # stop server
```

Agent integration lives in this repo:
- `skills/zmux/SKILL.md` is the canonical zmux skill. On the maintainer setup,
  `~/donjor/skills/skills/zmux` symlinks here, and `~/.claude/skills` points at
  that shared skill tree, so Claude does not need a separate repo-local link.
  `./dev.sh zmux` refreshes that shared link, then refreshes the shared skills
  repo's Codex/Pi mirrors without touching global agent settings. Restart Codex
  after the mirror refresh if you need the new skill text in an existing session.
  It also owns the generic agent-peer and agent-worker doctrine for driving real
  CLIs in visible zmux tabs: read-only review loops, write-capable workers bound
  to isolated worktrees, spawn profiles, the `type` -> `watch --idle` loop,
  capture classification, and etiquette. The doctrine lives at
  `skills/zmux/references/agent-peer.md` and
  `skills/zmux/references/agent-worker.md`; the exhaustive command catalog lives
  at `skills/zmux/references/cli-catalog.md`.
  `./install.sh` does not mutate agent skill directories.
- `pi-extension/` registers typed Pi tools and bash guardrails for deterministic
  runtime, tab/pane/send, sidecar, interactive-command, and terminal-capability
  orchestration. `./dev.sh zmux` relinks the Pi extension at
  `~/.pi/agent/extensions/pi-zmux`.

See [docs/pi-zmux-extension.md](docs/pi-zmux-extension.md).

Repo-local QA walkthroughs live in committed `checklists/*.toml` files and run
through `./qa` (a dedicated `cmd/qa` binary, not a `zmux` subcommand):

```bash
./qa                 # picker for human-verdict steps
./qa run <checklist> # agent-sweep automatic steps
./qa status <checklist> [--json]
./qa lint
```

See [docs/qa.md](docs/qa.md).

## Development

```bash
make build            # build binary
make test             # run unit tests
make test-race        # unit tests with the race detector (mirrors CI)
make test-integration # run integration tests (exercises the built CLI; no tmux needed)
make vuln             # govulncheck vulnerability scan
make lint             # go vet + golangci-lint (incl. gofumpt)
make clean            # remove build output
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the contributor guide and [docs/architecture.md](docs/architecture.md) for the codebase map.

## License

TBD — license has not been finalized for this repo yet. If you need to use or
redistribute this code, please open an issue or contact the maintainer first.
