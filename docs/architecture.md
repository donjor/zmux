# zmux Architecture

A contributor-oriented map of the codebase. Read this once and you should know where to make a change for any user-visible behavior.

For the product vision (the *why*), see [VISION.md](VISION.md). For keybindings (the user-visible API), see [keybindings.md](keybindings.md).

---

## Top-level tree

```
zmux/
├── cmd/                  # main entry points (binaries)
│   ├── zmux/             # thin launcher (main.go) — calls internal/cli.Run
│   └── uiproto/          # internal UI prototyping harness (not shipped)
├── internal/             # all business logic (Go's `internal/` visibility)
├── docs/                 # this directory (architecture, vision, keybindings, etc.)
├── themes/iterm2/        # downloaded theme cache (gitignored; not an embed source)
├── tests/                # integration tests (build tag: `integration`)
├── skills/zmux/          # Claude Code agent skill (symlinked by ./dev.sh)
├── pi-extension/         # Pi agent TypeScript extension (separate build)
├── legacy/v0/            # archived bash+gum prototype — see legacy/v0/README.md
├── Makefile              # build / test / lint / install
├── install.sh            # end-user installer (build + install + shell integration)
├── dev.sh                # MAINTAINER convenience (build + install + agent symlinks)
└── CLAUDE.md             # AI-agent context (concise, points to this file)
```

### Top-level dirs at a glance

| Path | Status | Notes |
|------|--------|-------|
| `cmd/` | Active | Go entry points |
| `internal/` | Active | All packages live here (Go internal/ visibility) |
| `docs/` | Active | This file and friends |
| `internal/session/templates/` | Active | Session templates — the **only** `go:embed` source (`internal/session/embed.go`) |
| `internal/theme/bundled/` | Active | Bundled themes — the **only** `go:embed` source (`internal/theme/embed.go`) |
| `themes/iterm2/` | Generated | Downloaded theme cache; gitignored. Runtime themes resolve under `~/.zmux/themes` |
| `legacy/v0/{templates,themes}/` | Archived | v0's own real asset copies (de-symlinked from the repo root) |
| `tests/` | Active | Integration tests, run with `go test -tags integration` |
| `skills/zmux/` | Active | Optional Claude Code agent integration |
| `pi-extension/` | Active | Optional Pi agent integration (TypeScript) |
| `legacy/v0/` | Archived | Old bash prototype — preserved, unsupported |

---

## `internal/` package map

Numbers below are approximate lines of non-test Go code per package, ordered by surface-area depth.

### Foundation packages (no zmux deps)

| Package | Lines | Role |
|---------|-------|------|
| `config` | ~250 | TOML config loading, defaults, `FS` interface for tests |
| `debug` | ~70 | Opt-in debug logging (`ZMUX_DEBUG=1`) |
| `procfs` | ~65 | Linux `/proc` process-tree inspection |
| `tablabel` | ~30 | Stable optional tab-label overlay format |
| `termtitle` | ~80 | tmux terminal-title format contract + parser (no zmux deps; dissolves the latent tmux↔terminal cycle) |
| `terminal` | ~240 | Resolves strict screenshot targets for the current tmux client |
| `keys` | ~280 | Keybinding registry — single source of truth for `conf.go`, help surfaces, and the generated `docs/keybindings.md` |
| `guard` | ~270 | Classifies a shell command for agent terminal-hygiene (raw-tmux / background-job / shared-interaction); ruleset shares `testdata/zmux-guard-corpus.jsonl` with the Claude hook + pi classifier. Pure leaf, no deps |

### tmux boundary

| Package | Lines | Role |
|---------|-------|------|
| `tmux` | ~1580 | Typed wrapper around the `tmux` CLI — `Runner` interface, `MockRunner` for tests, generated `tmux.conf` |

**`Runner` is the boundary.** Production uses `tmux.NewRunner()` which shells out to `tmux`. Tests use `tmux.MockRunner` for deterministic, observable interactions. Anything that calls `os/exec` for tmux **must** route through `Runner`.

### Domain packages

| Package | Lines | Role |
|---------|-------|------|
| `session` | ~490 | Session model, CRUD, TOML templates |
| `workspace` | ~630 | Workspace state (TOML), session tracking, reconciliation |
| `theme` | ~650 | Theme parsing, semantic palette, resolver, embed of bundled themes |
| `bar` | ~2100 | Status bar generation, presets, two-line rendering, preview |
| `sync` | ~220 | Pull-only theme sync targets (Ghostty, Neovim) |
| `source` | ~430 | External source discovery (tmux sockets, catalog) + generic attach fallback |
| `overmind` | ~120 | Overmind control client (`Client` interface: connect/restart/stop/logs) |
| `setup` | ~190 | Shell-rc integration: pure plan + apply behind `config.FS` (markers, `.bak`, dry-run, removal) |
| `wm` | ~150 | Window manager adapters (Hyprland today) |
| `snapshot` | ~470 | Terminal/TUI evidence bundle — per-pane text/ANSI captures + optional strict PNG screenshot (`zmux snapshot`); Go-native port of the pi-parley vision tool. All side effects via `tmux.Runner` / `config.FS` / `TargetResolver` |

### UI

The flat `tui` root package was dissolved (S3) into focused leaf/surface packages; there is no longer a `package tui`.

| Package | Role |
|---------|------|
| `tui/styles` | Shared lipgloss styles leaf (`Styles`, `NewStyles`) |
| `tui/workspaceview` | Workspace-view data adapter leaf (consumed by picker **and** dashboard; deps: session+workspace only) |
| `tui/picker` | Primary workspace+session picker (model/update/view/actions + local keymap) |
| `tui/tabpicker` | Alt+` tab switcher |
| `tui/themepicker` | Standalone theme picker |
| `tui/wizard` | First-run `zmux init` setup wizard |
| `tui/palette` | Command palette: registry, providers, executor |
| `tui/dashboard` | Tabbed dashboard `App` and `Tab` interface |
| `tui/dashboard/tabs` | Tab implementations: current, sessions, themes, bar, settings, help |
| `tui/views` | Shared row/column components |
| `tui/outline` | Tree-outline component |
| `preview` | UI prototype framework (`Page`, `Controls`, `RenderContext`) |

### File size guidance

Prod files over ~500 lines are a smell — read carefully to decide whether the size is intentional cohesion or accidental accumulation. As of the post-omega / cli-extraction refactors, **no production file exceeds 500 lines**; the largest and how to think about them:

| File | Lines | Notes |
|------|-------|-------|
| `internal/tui/dashboard/tabs/settings.go` | 496 | Settings tab — config fields + live preview. Cohesive; split per field-group if it grows. |
| `internal/preview/bar/draft/multisession.go` | 462 | Draft / non-production preview code with known dead helpers. Safe to prune when this directory next gets attention. |
| `internal/tui/dashboard/tabs/current.go` | 458 | Session tab; already has `current_actions.go` / `current_tree.go` splits alongside it. |
| `internal/bar/generate.go` | 430 | tmux.conf + bar generation. Cohesive generator. |
| `internal/tui/dashboard/tabs/sessions_actions.go` | 412 | Workspace-tab action handlers, split out of `sessions.go`. |
| `internal/tmux/client.go` | 408 | Typed tmux CLI wrapper surface. |

If you're adding code that pushes one of these past ~500, consider splitting along an existing seam in the same package (e.g. `xxx_helpers.go`, `xxx_view.go`) rather than continuing to grow the monolith.

### Recent splits (May 2026)

Four large monoliths were split following the per-preset / MVU / popup-mode patterns. Reading order if you're new:

| Package / surface | Top-level file (post-split) | Split files |
|-------------------|------------------------------|-------------|
| `internal/bar` rendering | `render.go` (~195L — `BarContext`, dispatchers) | `render_context.go` (GatherContext + git/lang/dir helpers), `render_<preset>.go` (one per preset: default, minimal, blocks, rounded, hacker, zen, starship, powerline+rpowerline) |
| `internal/tui` picker | `picker.go` (~240L — model + Update) | `picker_update.go`, `picker_actions.go`, `picker_outline.go`, plus pre-existing `picker_view.go`, `picker_search.go`, `picker_external.go`, `picker_types.go` |
| `internal/cli` root | `root.go` (~160L — cobra wiring + `Run` + shorthand + version check) | `popup_modes.go` (--dashboard / --palette / --tab-picker), `session_picker.go` (outside-tmux flow) |
| `internal/tui/dashboard/tabs` bar tab | `bar.go` (~275L — types + lifecycle + Update + keys) | `bar_view.go`, `bar_helpers.go` |
| `internal/preview/pane` page | `page.go` (~210L — Page + Render + Dump) | `page_fixtures.go` (workload fixtures), `page_layouts.go` (split/grid/stacked/focus-rail + chrome), `page_util.go` (fit/clamp helpers). 7 dead functions pruned. |

---

## Key interfaces (the seams)

Anything that does I/O or talks to the OS sits behind an interface so tests can use a mock. **Don't add new direct `os/exec` or `os.ReadFile` calls in business logic.**

| Interface | Package | Production impl | Mock |
|-----------|---------|-----------------|------|
| `tmux.Runner` | `internal/tmux` | `NewRunner()` (shells out) | `MockRunner` |
| `config.FS` | `internal/config` | OS filesystem (`RealFS`) | inline test fakes |
| `bar.Prober` | `internal/bar` | `ExecProber` (git/lang shellout) | fake prober |
| `source.prober` (unexported) | `internal/source` | `systemProber` (socket scan / `ps` / live probe) | `fakeProber` — `Discover()` wraps `discoverWith(systemProber{})` |
| `overmind.Client` | `internal/overmind` | `CLI` (shells out to overmind) | injected via `App.Overmind`; tests pass a fake/noop |
| `theme.EnvSetter` | `internal/theme` | tmux setenv | test fake |
| `theme.HTTPClient` | `internal/theme` | `http.Client` (for theme download) | test fake |
| `sync.SyncTarget` | `internal/sync` | per-target impls (ghostty, nvim) | composed |
| `sync.CmdRunner` | `internal/sync/nvim` | `os/exec` | test fake |
| `wm.CommandRunner` | `internal/wm/hyprland` | `hyprctl` shellout | test fake |
| `wm.Adapter` | `internal/wm` | `HyprlandAdapter` | mockable |
| `procfs.Inspector` | `internal/procfs` | reads `/proc` | mockable |
| `preview.Page` / `preview.Control` | `internal/preview` | many | n/a (composition) |
| `dashboard.Tab` | `internal/tui/dashboard` | each tab impl | n/a |
| `palette.ActionProvider` | `internal/tui/palette` | per-provider | composition |

If you're adding new I/O — file, network, exec — introduce or reuse an interface. If you find yourself shelling out directly in a tab or domain package, that's a smell.

---

## Major subsystems

### Status bar rendering

The bar is generated from a preset (powerline / blocks / minimal / etc.) and resolved against the current theme palette and live tmux state. Pipeline:

1. `internal/bar/bar.go` — preset definitions and the public render entry point.
2. `internal/bar/render.go` + `render_<preset>.go` — turn preset + theme + live `BarContext` into a single tmux status string (split per preset).
3. `internal/bar/render_context.go` — `GatherContext` assembles the `BarContext`.
4. `internal/bar/probe.go` — `bar.Prober` (default `ExecProber`) supplies git + language state; injected into `GatherContext` so the bar can be rendered without subprocesses in tests.
5. `internal/bar/generate.go` — produces the `set -g status-{left,right}` lines emitted into the generated tmux conf.
6. `internal/bar/preview.go` — visual preview used by the dashboard's Bar tab.

The render pipeline reads from `time.Now()` for timestamps, `bar.Prober` for git branch/dirty/ahead-behind and language detection, `tmux.Runner` for session/window metadata, and the resolved palette from `internal/theme`. Git/language side-effects are behind the `Prober` seam; remaining direct `time.Now()` usage is the main non-injected input.

### Workspace + session model

`internal/workspace` owns the user's workspaces (TOML files in `~/.zmux/workspaces/`). Each workspace tracks N sessions, each session tracks its name + tmux session group (clones share windows but have independent current-window pointers — see CLAUDE.md "Session Groups").

`internal/session` owns the per-session model, templates (in `internal/session/templates/`), and the `RootName()` helper that resolves clone names (`dev-b` → `dev`).

`workspace.Store.WorkspaceFor()` collapses clones internally — UI surfaces must resolve `#S` (raw tmux session name) to the root before display.

### TUI layout

```
internal/tui/                 — no flat package; focused leaves + surfaces
├── styles/                   — shared lipgloss styles leaf
├── workspaceview/            — workspace-view data adapter (picker + dashboard)
├── picker/                   — primary session/workspace picker
│   ├── picker.go             — model + Update
│   ├── picker_view*.go       — view rendering (list / help splits)
│   ├── picker_actions.go     — selection/CRUD actions
│   └── keymap.go             — picker-local keymap (component keys stay local)
├── tabpicker/                — Alt+` tab switcher
├── themepicker/              — standalone theme picker
├── wizard/                   — zmux init setup wizard
├── outline/                  — tree-outline subcomponent
├── views/                    — shared row/column components
├── palette/                  — command palette
│   ├── registry.go           — ActionProvider interface
│   ├── executor.go           — runs selected action
│   └── providers/            — built-in palette actions
└── dashboard/                — tabbed popup (prefix+Space)
    ├── tab.go                — Tab interface + msg routing
    ├── app.go                — DashboardApp (bubbletea Model)
    └── tabs/
        ├── current.go        — Session tab (active workspace's sessions)
        ├── sessions.go       — Workspaces tab (all workspaces)
        ├── themes.go         — Themes tab (pick/clone/edit)
        ├── bar.go            — Bar tab (preset + segment toggle)
        ├── settings.go       — Settings tab (config fields)
        ├── help.go           — Help tab (renders from internal/keys)
        └── shared_*.go       — rename/confirm overlays shared across tabs
```

Tabs all implement `dashboard.Tab` — `Activate`, `Deactivate`, `Update`, `View`. Cross-tab messages use `dashboard.TargetedMsg` / `AppIntentMsg`.

### Configuration

`~/.zmux.conf` is the user's TOML config. `internal/config` loads it via `FS` so tests use in-memory fakes. Themes resolve against three sources in order: user custom (`~/.zmux/themes/`), bundled (`go:embed` from `internal/theme/bundled/`), downloaded iterm2 (`themes/iterm2/`).

### Generated tmux config

`internal/tmux/conf.go` emits the entire `~/.zmux/tmux.conf` from the user's `config.Config` and the resolved `theme.Palette`. This is the file `zmux apply` writes. Hooks, status hooks, and bar hooks are generated here; the **keybindings come from the `internal/keys` registry** (`conf.go` references `keys.X.Key`), so the generated config, help surfaces, and `docs/keybindings.md` never drift.

### Preview framework

`internal/preview` is a reusable `Page` + `Control` harness used by the dashboard for the bar and theme previews. `internal/preview/pane/` and `internal/preview/bar/` host the concrete pages. The framework is intentionally separated from the production TUI so previews don't drag in production state.

---

## Cobra command tree

`internal/cli/root.go` wires the cobra command tree (`cmd/zmux/main.go` is a thin
launcher that calls `cli.Run`). Top-level commands (subset):

```
zmux
├── init              — first-time setup wizard
├── setup             — shell-rc integration (`setup shell`, via internal/setup)
├── apply             — regenerate tmux.conf and apply everything
├── new               — create a new session/workspace
├── ls                — list sessions
├── tabs              — list tabs in current session
├── tab               — tab management (rename, swap, refresh-names, …)
├── pane              — pane management (respawn, etc.)
├── send              — send keys to a window
├── type              — type text into a window
├── watch             — read window output
├── run               — run a command in a named window
├── workspace         — workspace management subtree
├── theme             — theme list / apply / clone / edit
├── bar               — bar preset commands
├── terminal          — terminal capability probes
├── help              — top-level help (prefix+? popup)
├── status            — internal status JSON
├── completion        — shell completion (cobra-generated)
└── keys (hidden)     — maintainer tooling: `keys gen` regenerates docs/keybindings.md from internal/keys
```

Plus a handful of UI launchers triggered by tmux bindings:

```
zmux --picker         # workspace+session picker
zmux --palette        # command palette
zmux --dashboard      # tabbed dashboard
zmux --tab-picker     # Alt+` tab switcher
```

These flags live in `root.go` and route into the corresponding `tui/` flow.

---

## Where to make common changes

| Want to… | Start in |
|----------|----------|
| Add a new CLI subcommand | `internal/cli/<name>.go` (cobra) — register in `internal/cli/root.go` |
| Add a tmux keybinding | `internal/keys` — add the `Binding` to the right table. `conf.go`, the dashboard Help tab, `zmux help`, and `docs/keybindings.md` all derive from it. Run `make keys-gen` to regenerate the doc (the `TestKeybindingsDocInSync` golden test / CI enforces freshness) |
| Add a dashboard tab | Implement `dashboard.Tab` under `internal/tui/dashboard/tabs/`. Register in `dashboard.App`. See an existing tab (e.g. `themes.go`) as a template. |
| Add a bar preset | `internal/bar/bar.go` — preset table + segment definitions. Update preview in `internal/bar/preview.go` if visual changes need preview. |
| Add a theme | Drop a new file into `internal/theme/bundled/` (iterm2-color-schemes format). It will be `go:embed`'d on next build. |
| Change a generated tmux behavior | `internal/tmux/conf.go` — but verify `internal/tmux/conf_test.go` still covers the new section. |
| Change session/workspace behavior | `internal/workspace/` (workspace-level) or `internal/session/` (per-session). Run integration tests (`make test-integration`) — they exercise the built `zmux` CLI end-to-end (binary output, not real tmux). |

---

## Conventions enforced in CLAUDE.md

A few rules worth re-stating here because they're easy to miss:

- **All side effects behind interfaces.** Direct `os/exec`, `os.ReadFile`, `http.Get` in business logic is rejected at review time. Use or introduce a typed interface.
- **Tests use `tmux.MockRunner`.** Never spin up real tmux in unit tests. Integration tests under `tests/` (build tag: `integration`) exercise the built `zmux` binary's CLI end-to-end (pure-output commands; no real tmux). Real-tmux coverage, if needed, belongs in future tmux-dependent integration tests.
- **Don't run `zmux init` inside tmux.** It refuses; this is intentional, not a bug.
- **Explicit DI, no package-global `app`** — `app.App` (in `internal/app`) is built once in `main` (`app.New()`) and threaded through `NewRootCmd(app)`. Each command is a `newXCmd(app)` constructor capturing it; flag state is constructor-local.
- **Session-group clones** (`dev-b`, `dev-c`) collapse to root in `ListSessions()` and `WorkspaceFor()`. UI must resolve `#S` to root before display — search for `RootName` in code if you're touching session labels.

---

## Out of scope for this doc

- Detailed product vision — see [VISION.md](VISION.md)
- Keybinding reference — see [keybindings.md](keybindings.md)
- Terminal capability matrix — see [terminal-capabilities.md](terminal-capabilities.md)
- Pi extension API — see [pi-zmux-extension.md](pi-zmux-extension.md)

---

## When this doc drifts

If a package map row, interface table, or "where to make changes" line is wrong, fix it in the same PR that introduced the drift. The contributor expectation is that this file matches reality.
