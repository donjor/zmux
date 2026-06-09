# zmux — vision

## What is zmux?

An opinionated, all-in-one tmux management wrapper. It replaces tmux's sharp
edges with a beautiful, interactive experience powered by modern CLI tooling.

zmux handles:
- **Session management** — interactive picker, fuzzy search, recipes
- **tmux configuration** — opinionated defaults, keybinds, vim copy mode
- **Theming** — 300+ themes, status bar presets, theme sync across tools
- **Terminal commands** — run, watch, send, type for scripting and agent workflows
- **Source discovery** — multi-socket scanning, overmind integration
- **Quality of life** — help system, prefix hints, session dashboard, command palette

## Who is it for?

Primarily built for the author's workflow (Hyprland, Ghostty, neovim).
Designed to be publishable and useful for anyone running tmux on Linux/macOS
who wants a polished experience without deep tmux knowledge.

## Versioning

### v0 — bash + gum prototype (complete, archived as `zmux0`)
- Bash scripts + charmbracelet/gum for UI
- Proved out the UX, feature set, and config format
- Still available under [`legacy/v0/`](../legacy/v0/) and as the `zmux0` command after `./legacy/v0/install.sh`

### v1 — Go rewrite (current)
- Clean-room rewrite in Go (bubbletea + lipgloss + cobra)
- Single binary, no gum/bash dependency
- TDD from the start — 470+ tests
- TOML config with proper sections
- Installed as `zmux` (the primary command)

## Architecture (v1)

```
cmd/
└── zmux/
    ├── main.go                 entry point, cobra root command
    ├── app.go                  dependency injection struct
    ├── bar.go                  bar commands (list, set, show)
    ├── bar_render.go           bar-render subcommand (called by tmux)
    └── ...                     apply, open, kill, ls, new, run, send, etc.

internal/
├── config/                     TOML config loading, defaults, validation
├── theme/                      theme parsing, resolution, semantic palette
├── tmux/                       tmux command interface, conf generation
├── bar/                        status bar presets, dynamic rendering, tmux hooks
├── session/                    session lifecycle, tmp cleanup
├── recipe/                     recipe discovery, planning, and execution
├── sync/                       theme sync targets (ghostty, nvim)
├── workspace/                  first-class workspace objects (versioned TOML v2)
├── source/                     external source discovery (sockets, overmind)
├── debug/                      opt-in debug logging (ZMUX_DEBUG=1)
└── tui/
    ├── picker.go               workspace picker TUI (outside tmux) — flat list, model+update
    ├── picker_view.go          picker rendering (workspace rows, session rows, ghost cmd)
    ├── picker_search.go        picker fuzzy search and query parsing
    ├── picker_types.go         picker types (pickerMode, WorkspaceViewModel, confirm target)
    ├── picker_external.go      external source section builder for the picker
    ├── tabpicker.go            tab switcher TUI (Alt+`)
    ├── themepicker.go          theme picker with swatches
    ├── wizard.go               init wizard TUI
    ├── keymap.go               shared key mappings
    ├── styles.go               shared lipgloss styles
    ├── tui.go                  shared TUI utilities
    ├── outline/                shared flat-tree model (picker + tab rendering)
    │   ├── row.go              Row, RowKind, stable ID constructors, FormatSessionCount
    │   ├── tree.go             Tree — cursor, expansion, pending-jump, 5-step restore
    │   ├── nav.go              MoveUp/Down/JumpTop/Bottom with selectable-skip
    │   └── scroll.go           ComputeWindow (scroll margin)
    ├── dashboard/              tabbed dashboard app (DashboardApp)
    │   ├── app.go              tab switching, global keys, routing
    │   ├── chrome.go           tab bar rendering, layout chrome
    │   ├── layout.go           size calculations
    │   ├── messages.go         cross-tab message types (intents, broadcasts)
    │   ├── tab.go              Tab interface, TabID constants
    │   └── tabs/               tab implementations
    │       ├── current.go               Session tab — container, init, activation
    │       ├── current_tree.go          Session tab row builder + renderers
    │       ├── current_actions.go       Session tab key dispatch + mode handlers
    │       ├── current_data.go          Session tab fetch + mutation commands
    │       ├── current_overlay.go       Session tab modal overlay rendering
    │       ├── sessions.go              Workspaces tab — container
    │       ├── sessions_tree.go         Workspaces tab row builder + renderers
    │       ├── sessions_actions.go      Workspaces tab key dispatch
    │       ├── sessions_overlay.go      Workspaces tab modal overlay rendering
    │       ├── themes.go                Themes tab — container, bubbletea plumbing
    │       ├── themes_picker.go         Theme list + swatches + filter
    │       ├── themes_editor.go         Inline color editor
    │       ├── themes_bar.go            Bar preset carousel + segment toggles
    │       ├── settings.go              Settings — config fields, sync target
    │       ├── help.go                  Help — keybindings reference
    │       ├── shared_mutations.go      Rename/kill helpers used by both tree tabs
    │       ├── shared_overlay.go        Confirm/rename/create overlay renderers
    │       └── mode_state.go            Unified confirmState / renameState / moveState
    ├── palette/                command palette (spotlight-style overlay)
    │   ├── model.go            bubbletea model, fuzzy search
    │   ├── registry.go         action registry
    │   ├── providers.go        action providers (sessions, themes, etc.)
    │   ├── action.go           action types
    │   └── executor.go         post-selection execution
    └── views/                  shared view components
        ├── sessionrow.go       session list row rendering
        ├── windowrow.go        window list row rendering
        ├── tabbar.go           tab bar widget
        ├── swatch.go           color swatch rendering
        ├── colorpicker.go      color editor widget
        ├── header.go           section headers
        ├── actions.go          action menu rendering
        ├── sessionlist.go      session list widget
        ├── confirm.go          confirmation dialog
        ├── depcheck.go         dependency check display
        └── input.go            text input widget
```

User paths:
```
~/.zmux.toml                    user config
~/.zmux/themes/                 user custom themes
~/.zmux/recipes/                user custom recipes
```

### Dashboard tab system

The dashboard is a tabbed bubbletea application rendered as a tmux popup
(prefix+Space). The architecture follows a container/tab pattern:

- **DashboardApp** — root model that owns the tab bar, handles global keys
  (tab switching, quit), and delegates to the active tab.
- **Tab interface** — each tab implements `Init`, `Update`, `View`, plus
  metadata methods (`ID`, `Title`, `ShortHelp`), `Activate`/`Deactivate`.
- **Tabs:**
  - **Session** (current.go) — windows in the current session, quick actions
  - **Workspaces** (sessions.go) — all sessions grouped by workspace,
    switch/create/kill, overmind process management
  - **Themes** — theme picker with color swatches, color editor,
    bar preset selection with live preview carousel
  - **Settings** — config fields (prefix, sync target, session options)
  - **Help** — keybindings and command reference

Shared view components in `internal/tui/views/` (SessionRow, WindowRow,
TabBar, etc.) are used by multiple tabs for consistent rendering.

### Command palette

The command palette (prefix+p) is a separate bubbletea model that provides
spotlight-style fuzzy search across all available actions:

- **Registry** collects actions from multiple **providers** (session actions,
  theme actions, navigation actions).
- **PaletteModel** renders the filtered list and handles selection.
- **Executor** runs the chosen action and returns a **PostAction** that may
  close the palette, open the dashboard to a specific tab, or show an error.

### Source discovery

The `internal/source/` package discovers tmux sessions beyond the default server:

- **Socket scanning** — reads the tmux socket directory (`/tmp/tmux-<uid>/`)
  for non-default sockets.
- **Process correlation** — single `ps` call to build a process table, then
  matches sockets to known owners (overmind, etc.).
- **Overmind provider** — detects `overmind start` processes, extracts control
  socket and Procfile paths, correlates to tmux sockets.
- **Live probing** — each candidate socket is probed with a timeout to verify
  liveness and collect session lists.
- **Catalog model** — `Catalog` groups sessions into `Local` (default server)
  and `External` (`SourceGroup` entries, each with a `Source` and its sessions).

The picker and dashboard SessionsTab both consume the catalog to show local
and external sessions in grouped sections.

## Design Decisions

### Hybrid dashboard
Inside tmux, zmux provides two complementary interfaces:
- **Command palette** (prefix+p) — fast, spotlight-style overlay for quick actions
  (switch session, run recipe, theme switch)
- **Full dashboard** (prefix+Space) — deeper management view with tabbed layout,
  session list, settings, and help

Both render as tmux popup overlays, activated via keybind (not a pane).

### Declarative recipes
v0 templates were bash scripts. v1 uses declarative TOML recipes:

```toml
name = "dev"
description = "Editor, server, and git"
kind = "session"
workspace = "{{ cwd_name | slug }}"
session = "{{ workspace }}"

[[tabs]]
name = "editor"
command = "nvim ."

[[tabs]]
name = "server"

[[tabs]]
name = "git"

[options]
focus_tab = "editor"
```

Recipes are discovered from:
1. `~/.zmux/recipes/` — user custom
2. Bundled recipes (embedded in binary)

### Visual direction
Charm CLI / Ghostty aesthetic — clean, spacious, beautiful, restrained.
Let the content speak. No unnecessary decoration. Elegant typography.

### tmux coupling
v1 builds directly against tmux. No multiplexer abstraction layer.
Architecture stays clean enough that abstracting later (zellij, etc.)
is feasible without a rewrite.

### Workspace-first access
The `zmux open <workspace> [session]` command is the primary way to access
a workspace. It attaches to an existing session or creates a new one within
the workspace. `zmux attach` is retained as a hidden alias for `open`.

## Configuration — .zmux.toml

```toml
theme = "ayu-dark"
prefix = "C-Space"

[bar]
preset = "default"              # 9 presets — see Status Bar section

[sessions]
auto_cleanup_tmp = true
default_shell = ""

[recipes]
paths = ["~/.zmux/recipes"]

[sync]
target = "ghostty"              # ghostty, nvim, or none
ghostty_config = "auto"         # auto-detected, or explicit path
```

## Themes

### Source
Themes use the iterm2-color-schemes format (key=value pairs with ANSI palette,
background, foreground, cursor, selection colors). This is the same format used
by Ghostty, Alacritty, Kitty, WezTerm, and others.

Themes are sourced from the
[mbadolato/iTerm2-Color-Schemes](https://github.com/mbadolato/iTerm2-Color-Schemes)
repository — the same upstream that Ghostty, Alacritty, and others pull from.
zmux does not depend on any terminal emulator being installed.

### Resolution order
1. `~/.zmux/themes/<name>`          — user custom themes (highest priority)
2. Bundled themes (embedded)        — curated set shipped with zmux
3. `~/.zmux/themes/iterm2/<name>`   — full downloaded set

### Palette mapping
iterm2 ANSI palette maps directly to zmux semantic roles:

| ANSI palette | zmux role | Semantic purpose |
|---|---|---|
| background | `BG` | base background |
| foreground | `FG` | primary text |
| palette 0 (black) | `SURFACE` | cards, elevated panels |
| palette 1 (red) | `ERROR` | destructive actions, errors |
| palette 2 (green) | `SUCCESS` | confirmations, active status |
| palette 3 (yellow) | `ACCENT` | primary accent, branding |
| palette 4 (blue) | `INFO` | links, secondary accent |
| palette 5 (magenta) | `SPECIAL` | unique items, recipes |
| palette 6 (cyan) | `META` | tags, metadata |
| palette 7 (white) | `MUTED` | secondary text, labels |
| palette 8 (bright black) | `DIM` | borders, separators, faint |
| cursor-color | `HIGHLIGHT` | focus indicators, cursor |

Role names are semantic — they describe purpose, not color. A theme's
`ERROR` may be red, pink, or orange depending on the palette. Code uses
`theme.Error` not `theme.Red`.

### Theme picker
Color swatches + metadata display. Shows palette strips, dark/light tag,
semantic role labels. Fast and informative — not a live terminal preview.

## Status Bar

### Design philosophy
The status bar is visually cohesive with starship prompts — same color palette,
similar segment-based layout. They are **not integrated** — they just share
a theme so they look consistent.

### Presets
Users pick a preset in `.zmux.toml`. Each preset defines the layout and style
of the status bar. 9 presets ship with zmux:

**default** — catppuccin-inspired rounded pills, icons, elevated surfaces.
**minimal** — clean, barely decorated, content-first.
**powerline** — angled separators, filled segments, directory chain.
**blocks** — square bracket segments, monospace, dense.
**rounded** — elevated pill segments, premium feel.
**hacker** — matrix-inspired, monospace, dense info, green on dark.
**zen** — ultra-minimal, barely there.
**starship** — colorful prompt-inspired, each segment its own color.
**rpowerline** — rounded powerline — angled fills with rounded caps.

### Dynamic segments
All presets render dynamic segments via `zmux bar-render` (called by tmux
via `#()`): git branch/dirty/ahead-behind, language version detection,
workspace, directory, active process, and group indicator. Segments are
individually toggleable in `[bar.segments]`.

### Behavior
- Normal state: session name (accent), window tabs, dynamic right segments
- Prefix active: session name changes color, right side shows available hotkeys
- Per-window git status via `--dir` (pane working directory)
- Inactive windows are dimmed, two-tone catppuccin-style tab styling
- Current theme colors applied automatically
- Instant refresh on session/window switch via tmux hooks

## Theme Sync

### Design
zmux **owns its own theme**. It never writes to other tools' configs.

Theme sync is a **pull-only** convenience feature. It reads what theme another
tool is using and sets zmux to match.

```
zmux theme sync            — pull from default sync target
zmux theme pull ghostty    — read Ghostty's current theme, apply to zmux
zmux theme pull nvim       — read nvim's colorscheme, find matching theme
```

### Why pull-only?
- zmux is independent. Changing your zmux theme doesn't break anything else.
- You change your Ghostty/nvim theme whenever you want. When you want zmux
  to match, run `zmux theme sync`. One command.
- No file watchers, no background processes, no race conditions.
- Other tools' configs are their business.

### Adding sync targets
Each target implements a `SyncTarget` interface with a `Pull()` method
that returns a theme name. Adding a new target (alacritty, kitty, wezterm)
means implementing one interface.

## Session Management

### Model
- Each terminal window starts zmux outside tmux
- zmux shows the interactive picker: attach existing or create new
- New sessions are temp (`tmp-N`) by default
- Rename to promote to a named session
- Unattached tmp sessions are cleaned up on next zmux start

### Dashboard (inside tmux)
Activated via prefix+Space as a tmux popup. Five tabs:

- **Session** — windows in the current session, quick actions
- **Workspaces** — all sessions grouped by workspace, switch/create/kill
- **Themes** — theme picker with swatches, color editor, bar preset selection
- **Settings** — config fields (prefix, sync, sessions)
- **Help** — keybindings and command reference

### Command palette (inside tmux)
Activated via prefix+p as a tmux popup. Spotlight-style fuzzy search
across all available actions — switch session, run recipe,
change theme, open dashboard tabs.

### Tab switcher (inside tmux)
Activated via Alt+` (no prefix needed). Lightweight popup for the
current session's tabs — search, switch, create, rename, reorder,
close. Faster than the full dashboard for tab management.

## Workspaces

Workspaces are first-class objects that group sessions by project. They are
persisted as versioned TOML v2 in `~/.zmux/workspaces.toml`.

### Model
- A workspace owns one or more sessions. Sessions belong to exactly one workspace.
- `zmux new <workspace> [session...]` creates one or more sessions within a workspace.
- `zmux open <workspace>` attaches to the workspace's last-active session.
- `zmux open <workspace> <session>` attaches to a specific session in a workspace.
- `zmux open <workspace> [session]` is the explicit form for scripts.
- The picker is **workspace-primary**: a single flat list of workspaces with
  their sessions expanded inline (when selected, or when search matches).
- The dashboard "Workspaces" tab groups by workspace.
- The status bar shows workspace name and session position (e.g. `myapp 2/4`).
- Reconcile auto-heals unmanaged sessions into same-named workspaces.
- Workspace name validation: no spaces, no reserved names ("temporary", etc.).

### Picker model (flat list)

The picker shows a single flat list of items with three kinds:

1. **Top action row** — `+ new tmp session` (empty input) or `+ create "<name>"` (typed)
2. **Workspace rows** — name, session count, root dir, last activity, attached indicator
3. **Session rows** — nested under their workspace when expanded; icon, name, window list

Navigation traverses the tree (workspaces and visible sessions). Search uses
`<workspace> <session>` grammar — typing a space filters sessions within the
matched workspace. Tab accepts ghost autocompletion. Matched characters are
underlined. ctrl+x deletes the workspace or session under the cursor.
ctrl+h toggles visibility of empty workspaces. There is no two-level drill-in.

### CLI
```
zmux new <ws>                   Create workspace + 'main' session, attach
zmux new <ws> <session>         Create workspace (if needed) + session, attach
zmux new <ws> <s1> <s2> ...     Variadic — create workspace + multiple sessions
zmux run <recipe>               Open recipe form / run recipe defaults with -y
zmux recipe list                List recipes
zmux open <ws> [session]        Explicit attach (alias: a, attach)
zmux kill <name>                Smart kill — workspace-first, then session
zmux ls                         List workspaces (workspace-primary default)
zmux ls <ws>                    List sessions within a workspace
zmux ls -s                      Flat session list (legacy)
zmux workspace list             List workspaces (alias: ws)
zmux workspace show <ws>        Show sessions in a workspace
zmux workspace kill <ws>        Kill a workspace and all its sessions
zmux session kill <session>     Kill a session
zmux tab move <tab> <dest>      Move tab to another session
zmux tab kill <tab>             Kill a tab
```

### Session navigation keybindings
```
prefix + w                      Workspace session picker
prefix + [ / ]                  Cycle prev / next session in workspace
prefix + Alt+1-9                Direct session switching within workspace
```

## Terminal Commands

zmux provides commands for running processes in named tmux windows and
interacting with them programmatically:

```
zmux run '<cmd>' -n <tab>      Run command in a named window, wait for exit
zmux run '<cmd>' -n <tab> -d   Run detached (don't wait — for servers)
zmux run '<cmd>' -n <tab> -f   Run and follow output
zmux watch <tab>               Capture current output from a window
zmux watch <tab> --until <pat> Wait until output matches a pattern
zmux watch <tab> -f            Follow output continuously
zmux send <tab> <keys>         Send raw keystrokes (e.g., C-c)
zmux type <tab> '<text>'       Type text followed by Enter
```

These are designed for agent workflows (Claude Code, scripts) where
processes need to run in managed windows without blocking the caller.
The `run` command creates or reuses a window by name, so repeated calls
target the same window.

## Keybinds

### tmux prefix: Ctrl+Space

| Key | Action |
|-----|--------|
| Space | zmux dashboard (popup) |
| p | command palette (popup) |
| d | detach |
| ? | help popup |
| , | rename session |
| . | label tab (blank clears) |
| w | workspace session picker |
| [ / ] | prev / next session in workspace |
| Alt+1-9 | switch to session N in workspace |
| c | new tab |
| n / N | next / previous tab |
| < / > | move tab left / right |
| x | close tab (with confirm) |
| v | copy mode (vim) |
| P | paste buffer |
| r | reload config (zmux apply) |

### No-prefix keys

| Key | Action |
|-----|--------|
| Alt+1-9 | jump to tab (no prefix) |
| Alt+` | tab switcher (no prefix) |

### Copy mode (vim)
| Key | Action |
|-----|--------|
| v | begin selection |
| y | yank to clipboard |
| / | search forward |
| ? | search backward |
| Esc | exit |

## CLI Interface

```
zmux                            — workspace picker (outside tmux) / dashboard (inside)

zmux new                        — create tmp-N session (no workspace)
zmux new <ws>                   — create workspace + 'main' session, attach
zmux new <ws> <session...>      — variadic: create workspace + sessions
zmux open <ws> [session]        — open workspace session (aliases: attach, a)
zmux run <recipe>               — recipe form with cwd/workspace/session defaults
zmux run <recipe> -y            — run recipe defaults without prompting
zmux recipe list                — list bundled and user recipes
zmux recipe fork <recipe>       — copy a bundled recipe for editing
zmux kill <name>                — smart kill (workspace-first, then session) (alias: k)
zmux ls                         — list workspaces (workspace-primary)
zmux ls <ws>                    — list sessions within a workspace
zmux ls -s                      — flat session list
zmux tabs [session]             — list tabs (alias: t)

zmux tab move <tab> <dest>      — move tab to another session
zmux tab kill <tab>             — kill a tab
zmux session kill <session>     — kill a session
zmux workspace list             — list workspaces (alias: ws)
zmux workspace show <ws>        — show workspace sessions
zmux workspace kill <workspace> — kill workspace and all its sessions

zmux run '<cmd>' -n <tab>       — run in named window, wait for exit
zmux run '<cmd>' -n <tab> -d    — run detached (servers)
zmux run '<cmd>' -n <tab> -f    — run + follow output
zmux watch <tab>                — capture tab output
zmux watch <tab> --until <pat>  — wait for pattern
zmux watch <tab> -f             — follow output
zmux send <tab> <keys>          — send keystrokes
zmux type <tab> '<text>'        — type text + Enter

zmux theme                      — browse + switch themes (fuzzy picker)
zmux theme set <name>           — set theme directly
zmux theme list                 — list available themes
zmux theme sync                 — pull theme from default sync target
zmux theme pull <target>        — pull theme from specific target (ghostty, nvim)

zmux bar                        — list bar presets with previews
zmux bar <preset>               — set preset directly

zmux init                       — setup wizard (run outside tmux)
zmux apply                      — apply theme + bar to running tmux
zmux status                     — show current config summary
zmux help                       — styled help with keybindings
zmux version                    — version info
zmux completion <shell>         — generate completions (bash/zsh/fish)
```

## Dependencies

### Required
- tmux (>= 3.2)
- Go 1.22+ (build only)

### Runtime (zero dependencies)
Single static binary. No bash, no gum, no external tools required.

### Optional
- wl-copy / xclip / pbcopy — clipboard integration
- curl — theme download during init
- ghostty, nvim — for theme sync targets
