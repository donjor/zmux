# zmux — Ideal Directory Structure

Derived from `README.md`, `docs/VISION.md`, `docs/ROADMAP.md`,
`docs/architecture.md`, `docs/keybindings.md`, `docs/pi-zmux-extension.md`,
`docs/terminal-*.md`, and `CONTRIBUTING.md`. No reference was made to the
current source layout — this is the shape the project *should* take based on
its documented feature surface.

Reviewed with buddy (stable channel) before commit.

## Principles

1. **Domain-first.** Each user-visible feature (session, workspace, tab, pane,
   bar, theme, sync, source, terminal) owns a package under `internal/`.
2. **Boundaries are explicit.** Anything that touches the OS — tmux, fs, http,
   exec, /proc, WM — lives behind an interface in its own package. Business
   logic never imports `os/exec` directly.
3. **Embedded assets live with their `//go:embed` directive.** Single source of
   truth — no top-level mirror directories. Bundled themes ship inside
   `internal/theme/bundled/`; bundled templates inside
   `internal/session/templates/bundled/`.
4. **cobra commands stay thin.** One file per top-level command at
   `cmd/zmux/<name>.go`; heavy logic lives in `internal/`. The global `app`
   composition root remains in `cmd/zmux/root.go`.
5. **TUI is fully isolated.** Each surface (picker, palette, dashboard,
   tabpicker, wizard) is a self-contained subpackage of `internal/tui/`.
6. **User vocabulary.** "Tab" beats "window" in package names — that's the word
   the CLI uses (`zmux tabs`, `zmux tab kill`).
7. **Per-preset / per-segment files** for the bar, mirroring the split pattern
   already validated in `docs/architecture.md → Recent splits`.
8. **Unit tests sit next to the package they cover** (Go convention).
   `tests/` is reserved for integration tests behind the `integration` build
   tag.

## Tree

```
zmux/
├── cmd/
│   ├── zmux/                       # the CLI binary
│   │   ├── main.go
│   │   ├── root.go                 # cobra root, global `app`, version check
│   │   │
│   │   ├── init.go                 # zmux init        (wizard launcher)
│   │   ├── apply.go                # zmux apply
│   │   ├── status.go               # zmux status
│   │   ├── help.go                 # zmux help (styled, with keybindings)
│   │   ├── version.go              # zmux version
│   │   ├── completion.go           # cobra-generated shell completions
│   │   ├── refresh.go              # zmux refresh (re-attach RGB-capable client)
│   │   │
│   │   ├── new.go                  # zmux new <ws> <s1> <s2>...
│   │   ├── open.go                 # zmux open / o / attach / a
│   │   ├── kill.go                 # zmux kill (workspace-first, then session)
│   │   ├── ls.go                   # zmux ls [-s] [<ws>]
│   │   ├── tabs.go                 # zmux tabs [session]
│   │   │
│   │   ├── tab.go                  # zmux tab move|kill|label (subcommand tree)
│   │   ├── pane.go                 # zmux pane open|toggle|focus|resize|close|...
│   │   ├── workspace.go            # zmux workspace list|kill|show (alias: ws)
│   │   ├── session.go              # zmux session kill
│   │   ├── theme.go                # zmux theme set|list|sync|pull
│   │   ├── bar.go                  # zmux bar [<preset>|show]
│   │   ├── terminal.go             # zmux terminal capabilities|current
│   │   │
│   │   ├── run.go                  # zmux run '<cmd>' -n <tab> [-d|-f]
│   │   ├── watch.go                # zmux watch <tab> [--until <pat>] [-f]
│   │   ├── send.go                 # zmux send <tab> <keys>
│   │   ├── type.go                 # zmux type <tab> '<text>'
│   │   │
│   │   ├── popup_modes.go          # --picker / --palette / --dashboard / --tab-picker
│   │   └── session_picker.go       # outside-tmux picker flow wiring
│   │
│   └── uiproto/                    # internal UI prototyping harness (not shipped)
│       └── main.go
│
├── internal/
│   │
│   ├── tmux/                       # the tmux boundary — only package allowed to shell out
│   │   ├── runner.go               # Runner interface
│   │   ├── exec.go                 # production impl (os/exec)
│   │   ├── mock.go                 # MockRunner for tests
│   │   └── conf/                   # generated ~/.zmux/tmux.conf
│   │       ├── conf.go             # top-level emit
│   │       ├── keys.go             # keybindings
│   │       └── hooks.go            # status + bar hooks
│   │
│   ├── config/                     # ~/.zmux.toml load/save, defaults
│   │   ├── config.go
│   │   ├── defaults.go
│   │   └── fs.go                   # FS interface + OS impl
│   │
│   ├── session/                    # session model + CRUD + templates
│   │   ├── session.go              # Session type
│   │   ├── crud.go                 # create/attach/kill
│   │   ├── group.go                # RootName, clone-suffix collapse (dev-b → dev)
│   │   ├── list.go                 # ListSessions (collapse clones, sum clients)
│   │   ├── tmp.go                  # tmp-N model + auto-cleanup
│   │   └── templates/              # declarative TOML templates
│   │       ├── parse.go
│   │       ├── apply.go
│   │       ├── discover.go         # user + bundled discovery
│   │       └── bundled/            # //go:embed dev, claude, webdev, monitor
│   │
│   ├── workspace/                  # workspace state, reconciliation
│   │   ├── workspace.go
│   │   ├── store.go                # TOML state on disk
│   │   ├── reconcile.go
│   │   └── resolve.go              # WorkspaceFor (clone-aware via session.RootName)
│   │
│   ├── tab/                        # zmux "tab" = tmux window (user vocabulary)
│   │   ├── tab.go
│   │   ├── label.go                # stable zmux label overlay
│   │   ├── autoname.go             # name[cwd] disambiguation
│   │   └── move.go                 # cross-session move
│   │
│   ├── pane/                       # pane open/toggle/focus/resize/close + sidecars
│   │   ├── pane.go
│   │   ├── sidecar.go              # zmux pane open --label-tab snapshot
│   │   └── header.go               # pane-border header formatting
│   │
│   ├── theme/                      # iterm2-color-schemes parser + palette + resolver
│   │   ├── parse.go                # iterm2 .itermcolors / .conf parse
│   │   ├── palette.go              # semantic palette mapping
│   │   ├── resolve.go              # user → bundled → iterm2 cache (order matters)
│   │   ├── http.go                 # HTTPClient interface (theme download)
│   │   ├── env.go                  # EnvSetter (tmux setenv)
│   │   └── bundled/                # //go:embed ayu-dark, dracula, nord, ... (11)
│   │
│   ├── bar/                        # status bar generation + rendering + preview
│   │   ├── bar.go                  # public surface + preset registry
│   │   ├── generate.go             # emit `set -g status-{left,right}` into conf
│   │   ├── render.go               # BarContext dispatcher
│   │   ├── render_context.go       # GatherContext + git/lang/dir helpers
│   │   ├── preview.go              # ANSI preview (carousel + dashboard)
│   │   │
│   │   ├── segments/               # dynamic segments (toggleable in [bar.segments])
│   │   │   ├── git.go              # branch / dirty / ahead-behind
│   │   │   ├── lang.go             # language version detection
│   │   │   ├── workspace.go        # workspace + session position (myapp 2/4)
│   │   │   ├── directory.go
│   │   │   ├── process.go          # active process in pane
│   │   │   ├── clock.go
│   │   │   └── group.go            # group/clone indicator
│   │   │
│   │   └── presets/                # one file per preset (9 presets)
│   │       ├── default.go          # catppuccin-inspired rounded pills
│   │       ├── minimal.go
│   │       ├── powerline.go
│   │       ├── rpowerline.go       # rounded powerline
│   │       ├── blocks.go
│   │       ├── rounded.go
│   │       ├── hacker.go
│   │       ├── zen.go
│   │       └── starship.go
│   │
│   ├── sync/                       # pull-only theme sync targets
│   │   ├── target.go               # SyncTarget interface
│   │   ├── ghostty/                # read Ghostty config; never write
│   │   │   └── ghostty.go
│   │   └── nvim/                   # read Neovim colorscheme
│   │       ├── nvim.go
│   │       └── cmd.go              # CmdRunner interface
│   │
│   ├── source/                     # external session discovery
│   │   ├── catalog.go              # Local + External (SourceGroup) catalog
│   │   ├── sockets.go              # scan /tmp/tmux-<uid>/
│   │   ├── procs.go                # single-ps process correlation
│   │   ├── probe.go                # per-socket liveness probe with timeout
│   │   └── providers/
│   │       └── overmind.go         # overmind start detection + Procfile extraction
│   │
│   ├── terminal/                   # outer terminal capabilities + window correlation
│   │   ├── capabilities.go         # `zmux terminal capabilities` (RGB/truecolor probe)
│   │   ├── current.go              # `zmux terminal current --json`
│   │   └── title.go                # stable terminal title metadata writer
│   │
│   ├── wm/                         # window-manager adapters
│   │   ├── adapter.go              # Adapter interface + CommandRunner
│   │   └── hyprland/
│   │       └── hyprland.go         # hyprctl shellout
│   │
│   ├── procfs/                     # /proc inspection (Inspector interface)
│   │   └── procfs.go
│   │
│   ├── setup/                      # first-run wizard non-UI helpers (init can't be a pkg name)
│   │   ├── dirs.go                 # create ~/.zmux/{themes,templates,...}
│   │   ├── shell.go                # shell integration (auto-start on terminal open)
│   │   └── skills.go               # optional symlink: Claude skill + Pi skill/extension
│   │
│   ├── keys/                       # keybinding source of truth (matches docs/keybindings.md)
│   │   ├── actions.go              # Action registry
│   │   └── bindings.go             # prefix / no-prefix / popup binding tables
│   │
│   ├── debug/                      # ZMUX_DEBUG opt-in logging
│   │   └── debug.go
│   │
│   ├── tui/                        # all bubbletea surfaces
│   │   ├── styles/                 # shared lipgloss styles
│   │   │   └── styles.go
│   │   │
│   │   ├── views/                  # shared row/column components
│   │   │   ├── session_row.go
│   │   │   ├── window_row.go
│   │   │   ├── tab_bar.go
│   │   │   └── ...
│   │   │
│   │   ├── outline/                # tree-outline component
│   │   │   └── outline.go
│   │   │
│   │   ├── picker/                 # workspace picker (outside tmux + popup)
│   │   │   ├── model.go
│   │   │   ├── update.go
│   │   │   ├── view.go
│   │   │   ├── search.go           # fuzzy + ghost autocomplete + matched-char underlines
│   │   │   ├── actions.go          # enter, ctrl+x, ctrl+h, ctrl+t, 1–9
│   │   │   └── external.go         # external source section rendering
│   │   │
│   │   ├── palette/                # command palette (prefix+p)
│   │   │   ├── registry.go         # ActionProvider interface
│   │   │   ├── executor.go         # runs selected action
│   │   │   └── providers/          # built-in palette actions
│   │   │
│   │   ├── tabpicker/              # Alt+` tab switcher
│   │   │   └── tabpicker.go
│   │   │
│   │   ├── wizard/                 # init wizard TUI shell (uses internal/setup)
│   │   │   └── wizard.go
│   │   │
│   │   └── dashboard/              # tabbed popup (prefix+Space)
│   │       ├── app.go              # DashboardApp (bubbletea Model)
│   │       ├── tab.go              # Tab interface + msg routing (TargetedMsg, AppIntentMsg)
│   │       ├── shared/             # cross-tab helpers
│   │       │   ├── rename.go       # rename overlay
│   │       │   ├── confirm.go      # confirm overlay
│   │       │   ├── scroll.go
│   │       │   └── mode_state.go
│   │       └── tabs/
│   │           ├── session/        # "Session" tab — current workspace's sessions
│   │           ├── workspaces/     # "Workspaces" tab — all workspaces
│   │           ├── themes/         # picker + clone/edit + swatches
│   │           ├── bar/            # preset chooser + segment toggle + preview
│   │           ├── settings/       # config fields (prefix, sync, sessions)
│   │           └── help/           # keybinding reference
│   │
│   └── preview/                    # UI prototype framework (used by cmd/uiproto)
│       ├── page.go                 # Page interface
│       ├── controls.go             # Control interface
│       ├── render.go               # RenderContext
│       ├── pane/                   # concrete pane preview pages
│       │   ├── page.go
│       │   ├── page_fixtures.go
│       │   ├── page_layouts.go
│       │   └── page_util.go
│       └── bar/                    # concrete bar preview pages
│           └── page.go
│
├── tests/                          # integration tests (build tag: integration)
│   ├── session_test.go
│   ├── workspace_test.go
│   ├── bar_test.go
│   ├── terminal_test.go
│   └── README.md                   # how to run: go test -tags integration ./tests/...
│
├── skills/
│   └── zmux/                       # Claude Code agent skill — symlinked by dev.sh/install.sh
│       ├── SKILL.md
│       └── ...
│
├── pi-extension/                   # Pi agent TypeScript extension (separate build)
│   ├── package.json
│   ├── src/
│   └── README.md
│
├── docs/                           # architecture, vision, roadmap, keybindings, proposals
│   ├── README.md
│   ├── VISION.md
│   ├── ROADMAP.md
│   ├── architecture.md
│   ├── keybindings.md
│   ├── pi-zmux-extension.md
│   ├── terminal-capabilities.md
│   ├── terminal-current.md
│   └── terminal-snapshot-correlation-proposal.md
│
├── legacy/
│   └── v0/                         # archived bash+gum prototype — preserved, unsupported
│
├── Makefile                        # build / test / test-integration / lint / install / clean
├── install.sh                      # end-user installer (build + install + shell integration)
├── dev.sh                          # maintainer convenience (build + install + agent symlinks)
├── README.md
├── CONTRIBUTING.md
└── CLAUDE.md                       # AI-agent context — points to docs/architecture.md
```

## Where each documented feature lives

| Feature (from docs) | Home |
|---------------------|------|
| Workspace-primary picker (outside tmux + popup) | `internal/tui/picker/` |
| Dashboard popup (5 tabs) | `internal/tui/dashboard/` |
| Command palette (prefix+p) | `internal/tui/palette/` |
| Alt+` tab switcher | `internal/tui/tabpicker/` |
| Theming (iterm2 format, semantic palette) | `internal/theme/` |
| Theme sync (Ghostty / Neovim) | `internal/sync/` |
| Workspaces (containers grouping sessions) | `internal/workspace/` |
| 9 status bar presets + dynamic segments | `internal/bar/{presets,segments}/` |
| Declarative TOML templates | `internal/session/templates/` |
| Terminal commands (run/watch/send/type) | `cmd/zmux/{run,watch,send,type}.go` + `internal/tmux/` |
| Multi-source discovery (sockets, overmind) | `internal/source/` |
| Init wizard | `internal/tui/wizard/` + `internal/setup/` |
| Shell completions | `cmd/zmux/completion.go` (cobra-generated) |
| Tab management (rename, move, kill, label) | `cmd/zmux/tab.go` + `internal/tab/` |
| Pane management (open, toggle, focus, resize) | `cmd/zmux/pane.go` + `internal/pane/` |
| `zmux terminal capabilities` / `current` | `internal/terminal/` |
| Hyprland adapter (and future WMs) | `internal/wm/hyprland/` |
| Session-group clones (dev-b, dev-c collapse to dev) | `internal/session/group.go` + `internal/workspace/resolve.go` |
| Generated `~/.zmux/tmux.conf` | `internal/tmux/conf/` |
| Bundled themes (11) | `internal/theme/bundled/` (`//go:embed`) |
| Bundled templates (dev, claude, webdev, monitor) | `internal/session/templates/bundled/` (`//go:embed`) |
| UI prototyping harness | `cmd/uiproto/` + `internal/preview/` |
| Pi agent integration | `pi-extension/` + `skills/zmux/` |
| Claude Code skill | `skills/zmux/` (symlinked) |
| Legacy bash+gum prototype | `legacy/v0/` |

## Notable choices and trade-offs

- **`internal/setup/` not `internal/init/`.** `init` is a reserved Go function
  name and cannot be a package name. Non-UI wizard helpers (mkdir, shell rc
  patches, symlink prompts) live here; the TUI shell lives at
  `internal/tui/wizard/`.
- **Single source of truth for embedded assets.** Bundled themes and templates
  sit next to their `//go:embed` directives — no top-level mirror dirs. The
  current README's `~/.zmux/themes/` user dir is runtime state, not source.
- **`internal/tab/` over `internal/window/`.** The CLI vocabulary is "tab"
  (`zmux tabs`, `zmux tab kill`, `zmux tab move`), even though tmux calls these
  "windows". User-facing names win in package naming.
- **Flat cobra commands.** Each top-level cobra command is a single file in
  `cmd/zmux/`. Subcommand trees (`zmux tab ...`, `zmux pane ...`,
  `zmux workspace ...`, `zmux theme ...`) live in a single file each — keeps
  the root.go wiring small and discoverable. No subdir command packages.
- **Bar split mirrors the validated May-2026 split pattern.** One file per
  preset, one file per segment — matches `docs/architecture.md → Recent splits`.
- **`internal/keys/` as the keybindings source of truth.** Today the binding
  tables are scattered between `tmux/conf/keys.go` and the help/dashboard.
  A single `internal/keys/` package — paired with `docs/keybindings.md` — keeps
  the prefix / no-prefix / popup matrices in one place so the CLI, help popup,
  and generated tmux.conf cannot drift.
- **`tests/` for integration only.** Unit tests live next to their packages
  (Go convention). `tests/` is gated by `//go:build integration` and exercises
  real tmux.
- **No `internal/app/`.** Composition root stays as the global `app` in
  `cmd/zmux/root.go`, per the existing convention.

## What this tree does *not* try to do

- It does not split files below the package level — the per-preset / per-tab /
  per-command file layout is shown above and is the granularity that matters.
- It does not prescribe sub-files for every package; small packages
  (`config`, `procfs`, `debug`, `wm`) stay flat.
- It does not move shipped artifacts (`Makefile`, `install.sh`, `dev.sh`,
  `README.md`, `CLAUDE.md`, `CONTRIBUTING.md`) — those stay at the repo root.
