# zmux Architecture

A contributor-oriented map of the codebase. Read this once and you should know where to make a change for any user-visible behavior.

For the product vision (the *why*), see [VISION.md](VISION.md). For keybindings (the user-visible API), see [keybindings.md](keybindings.md).

---

## Overview

zmux is a Go CLI around tmux. The command tree lives under `internal/cli`, core
tmux interaction is isolated behind `internal/tmux`, and the user-facing
surfaces are split across focused packages for workspaces, recipes, themes, the
status bar, logical tabs, terminal evidence, and Bubble Tea TUIs. Optional agent
integrations live at the repository edge in `skills/zmux/` and `pi-extension/`.

## Key components

### Top-level tree

```
zmux/
├── cmd/                  # main entry points (binaries)
│   ├── zmux/             # thin launcher (main.go) — calls internal/cli.Run
│   ├── qa/               # repo-local QA walkthrough runner, invoked by ./qa
│   └── uiproto/          # internal UI prototyping harness (not shipped)
├── checklists/           # committed manual/automatic QA walkthrough specs
├── internal/             # all business logic (Go's `internal/` visibility)
├── docs/                 # this directory (architecture, vision, keybindings, etc.)
├── themes/iterm2/        # downloaded theme cache (gitignored; not an embed source)
├── tests/                # integration tests (build tag: `integration`)
├── skills/zmux/          # Agent skill: terminal orchestration + agent peer/worker doctrine
├── pi-extension/         # Pi agent TypeScript extension (separate build)
├── legacy/v0/            # archived bash+gum prototype — see legacy/v0/README.md
├── Makefile              # build / test / lint / install
├── install.sh            # end-user installer (build + install + shell integration)
├── dev.sh                # MAINTAINER convenience (build + install + agent symlinks)
├── AGENTS.md             # canonical AI-agent context
└── CLAUDE.md -> AGENTS.md # compatibility symlink
```

### Top-level dirs at a glance

| Path | Status | Notes |
|------|--------|-------|
| `cmd/` | Active | Go entry points |
| `checklists/` | Active | QA walkthrough TOML specs consumed by `./qa` |
| `internal/` | Active | All packages live here (Go internal/ visibility) |
| `docs/` | Active | This file and friends |
| `internal/recipe/recipes/` | Active | Bundled recipes — the recipe `go:embed` source (`internal/recipe/embed.go`) |
| `internal/theme/bundled/` | Active | Bundled themes — the **only** `go:embed` source (`internal/theme/embed.go`) |
| `themes/iterm2/` | Generated | Downloaded theme cache; gitignored. Runtime themes resolve under `~/.zmux/themes` |
| `legacy/v0/{templates,themes}/` | Archived | v0's own real asset copies (de-symlinked from the repo root) |
| `tests/` | Active | Integration tests, run with `go test -tags integration` |
| `.qa/` | Generated | QA scorecards + cached `cmd/qa` binary; gitignored |
| `skills/zmux/` | Active | Optional agent integration: terminal orchestration plus generic agent peer/worker doctrine |
| `pi-extension/` | Active | Optional Pi agent integration (TypeScript) |
| `legacy/v0/` | Archived | Old bash prototype — preserved, unsupported |

---

## `internal/` package map

Numbers below are approximate lines of non-test Go code per package, ordered by surface-area depth.

### Foundation packages (no zmux deps)

| Package | Lines | Role |
|---------|-------|------|
| `config` | ~325 | TOML config loading, defaults, `FS` interface for tests |
| `debug` | ~70 | Opt-in debug logging (`ZMUX_DEBUG=1`) |
| `procfs` | ~65 | Linux `/proc` process-tree inspection |
| `tablabel` | ~30 | Stable optional tab-label overlay format |
| `termtitle` | ~80 | tmux terminal-title format contract + parser (no zmux deps; dissolves the latent tmux↔terminal cycle) |
| `terminal` | ~240 | Resolves strict screenshot targets for the current tmux client |
| `keys` | ~280 | Keybinding registry — single source of truth for `conf.go`, help surfaces, and the generated `docs/keybindings.md` |
| `guard` | ~480 | Classifies a shell command for agent terminal-hygiene (raw-tmux / background-job / shared-interaction), recursively scanning nested-form payloads (`sh -c`, env/path-prefixed shells, `xargs`, here-docs); ruleset shares `testdata/zmux-guard-corpus.jsonl` with the Claude hook + pi classifier. Pure leaf, no deps |

### tmux boundary

| Package | Lines | Role |
|---------|-------|------|
| `tmux` | ~2450 | Typed wrapper around the `tmux` CLI — `Runner` interface, `MockRunner` for tests, generated `tmux.conf`, logical-pane scan, and placement primitives |

**`Runner` is the boundary.** Production uses `tmux.NewRunner()` which shells out to `tmux`. Tests use `tmux.MockRunner` for deterministic, observable interactions. Anything that calls `os/exec` for tmux **must** route through `Runner`.

### Domain packages

| Package | Lines | Role |
|---------|-------|------|
| `session` | ~430 | Session model, CRUD, tmp-session helpers |
| `recipe` | ~1650 | Recipe discovery, TOML parsing, pure planning, dry-run rendering, execution, plus fork/extend/create authoring |
| `workspace` | ~1250 | Workspace state (TOML), v3 local-label session identity, session tracking, reconciliation |
| `tabs` | ~1450 | Logical-tab identity, scanning, placement, dock hide/show, MRU, reconciliation, lifecycle stamps (`lifecycle.go`), and the reaper (`reap.go`) |
| `tabstate` | ~280 | Pane-canonical tab lifecycle state (`running` / `done` / `failed` / `attention`) and bar glyph formatting |
| `theme` | ~650 | Theme parsing, semantic palette, resolver, embed of bundled themes |
| `bar` | ~2700 | Status bar generation, presets, two-line logical-tabs rendering, preview |
| `sync` | ~220 | Pull-only theme sync targets (Ghostty, Neovim) |
| `source` | ~545 | External source discovery (tmux sockets, catalog) + generic attach fallback |
| `overmind` | ~120 | Overmind control client (`Client` interface: connect/restart/stop/logs) |
| `setup` | ~190 | Shell-rc integration: pure plan + apply behind `config.FS` (markers, `.bak`, dry-run, removal) |
| `wm` | ~150 | Window manager adapters (Hyprland today) |
| `snapshot` | ~525 | Terminal/TUI evidence bundle — per-pane text/ANSI captures + optional strict PNG screenshot (`zmux snapshot`); Go-native port of the pi-parley vision tool. All side effects via `tmux.Runner` / `config.FS` / `TargetResolver` |
| `capturelog` | ~155 | Bounded sink behind `zmux log` (tail-style logging). A `tmux pipe-pane` feeds the hidden `zmux log-sink`; the sink keeps only the trailing `--max-bytes` in memory (exact cap, no rotation files) and flushes the whole buffer via `config.FS`. Optional stateful ANSI/control stripping for a readable plain log |
| `qa` | ~1320 | Repo-local QA walkthrough framework: checklist parse/lint, scorecard state, shell runner, and `cmd/qa` CLI |
| `actions` | ~80 | Typed action model (payloads) the command palette executes through `tmux.Runner` |
| `help` | ~180 | Help-content SSOT (command + keybinding reference) shared by `zmux help` and the prefix+? viewer, so the two can't drift |

### UI

The flat `tui` root package was dissolved into focused leaf/surface packages; there is no longer a `package tui`.

| Package | Role |
|---------|------|
| `tui/styles` | Shared lipgloss styles leaf (`Styles`, `NewStyles`) |
| `tui/workspaceview` | Workspace-view data adapter leaf (consumed by picker **and** dashboard; deps: session+workspace only) |
| `tui/workspaceoutline` | Shared workspace/session/external **row structure** built once and rendered by both the picker and the dashboard Workspaces tab; surface-specific labels, badges, and expansion arrive via `Policy` callbacks |
| `tui/workspacelist` | Reusable workspace-list component used by the workspace switcher |
| `tui/wspicker` | No-prefix `Alt+w` workspace switcher |
| `tui/picker` | Primary workspace+session picker (model/update/view/actions + local keymap) |
| `tui/tabpicker` | Alt+` tab switcher; understands full, pane-of, and hidden logical tabs |
| `tui/themepicker` | Standalone theme picker |
| `tui/wizard` | First-run `zmux init` setup wizard |
| `tui/recipeup` | Recipe create/edit form wizard (`recipe create` / `recipe edit`) |
| `tui/qapicker` | Human-facing QA walkthrough picker for `./qa` |
| `tui/tkey` | Small key helpers shared by TUI surfaces |
| `tui/palette` | Command palette: registry, providers, executor |
| `tui/dashboard` | Tabbed dashboard `App` and `Tab` interface |
| `tui/dashboard/tabs` | Tab implementations: current, sessions, themes, bar, settings, help |
| `tui/helpview` | `prefix+?` interactive help viewer — scroll, fuzzy filter, commands/keys/all scope toggle; renders from `internal/help` |
| `tui/views` | Shared row/column components |
| `tui/outline` | Tree-outline component |
| `tui/scroll` | Shared viewport scrollbar renderer (dashboard tabs + help viewer) |
| `preview` | UI prototype framework (`Page`, `Controls`, `RenderContext`) |

## Decisions

- **Go owns the active implementation** - the bash prototype remains archived in
  `legacy/v0/`, while new features and fixes target the Go packages.
- **`cmd/zmux` stays thin** - command behavior belongs under `internal/cli`, with
  `cmd/zmux/main.go` acting as the launcher.
- **Side effects sit behind interfaces** - tmux, filesystem, process, command,
  source-discovery, and terminal-window behavior are injected or wrapped so tests
  can stay deterministic.
- **Logical tabs are pane-canonical** - windows are presentation containers;
  pane-scoped tab identity lets tabs move between full, pane, and hidden dock
  placements.
- **Generated user surfaces derive from registries** - keybindings and tmux
  config should read from canonical registries instead of hardcoded duplicate
  strings.

Durable decision records live in [decisions/](decisions/).

## Key interfaces (the seams)

Anything that does I/O or talks to the OS sits behind an interface so tests can use a mock. **Don't add new direct `os/exec` or `os.ReadFile` calls in business logic.**

| Interface | Package | Production impl | Mock |
|-----------|---------|-----------------|------|
| `tmux.Runner` | `internal/tmux` | `NewRunner()` (shells out) | `MockRunner` |
| `config.FS` | `internal/config` | OS filesystem (`RealFS`) | inline test fakes |
| `qa.StateFS` | `internal/qa` | `.qa/` JSON scorecards | test fake |
| `qa.CmdRunner` | `internal/qa` | `ShellRunner` (`sh -c`) | test fake |
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

The render pipeline reads from `time.Now()` for timestamps, `bar.Prober` for git branch/dirty/ahead-behind and language detection, `tmux.Runner` for session/window metadata, and the resolved palette from `internal/theme`. Git/language side-effects are behind the `Prober` seam; remaining direct `time.Now()` usage is the main non-injected input. Shared tmux options in `generate.go` also own pane-border headers: single-pane and split windows use the same `<index> <name> <detail>` format, while `status-right` stays reserved for volatile status segments.

### Workspace + session model

`internal/workspace` owns the user's workspaces (`workspaces.toml` under the active profile state dir). Workspace names are globally unique; session labels are local to a workspace, so every workspace can have natural labels like `main`, `dev`, and `server`.

The durable store is v3: each workspace session has a stable ID, a local label, and a generated raw tmux name such as `zws_<workspace>__<label>`. Zmux stamps the live tmux session with `@zmux_managed`, `@zmux_workspace`, `@zmux_session_label`, and `@zmux_session_id`; tmux options are authoritative for live metadata, while raw names are debug/interop output. Migrated v1/v2 records can temporarily retain `legacy_tmux_name` so the next reconcile can rename the live old tmux session to its generated raw name, stamp metadata, and clear the hint.

`internal/session` owns the per-session model and the `RootName()` helper that resolves clone names (`dev-b` for legacy sessions, `__clone_b` for managed sessions). `internal/recipe` owns declarative launch plans and bundled recipes.

`internal/workspace/create.go` exposes `CreateManagedSession` — the single create path shared by `zmux new` and the dashboard, so both produce identically-stamped, addressable `zws_<workspace>__<label>` sessions (and clean up the live tmux session on any post-create failure) instead of the dashboard's old raw-label sessions that the picker and bar could not resolve.

User-facing command targets use `workspace/session`. `internal/cli/session_target.go`
resolves explicit `workspace/session` targets, current-workspace or cwd-local
labels, unique labels across workspaces, and finally raw tmux names as a debug
escape hatch. UI surfaces should display `@zmux_workspace` +
`@zmux_session_label` and use raw names only as operation targets.

### Logical tabs, placement, and state

`internal/tabs` is the logical-tab core. A zmux-managed tab is identified by a
pane-scoped `@zmux_tab_id`; windows are presentation containers. Placement is
computed from live tmux state on each scan:

- `full` — the tab owns a window.
- `pane-of` — the tab is a pane inside another managed tab's window.
- `dock` — the tab is parked in the reserved hidden session `__zmux_dock`.

`internal/cli/tab_target.go` is the shared resolver for name-addressed tab
commands. Keep new tab-targeting behavior behind that choke point so
`run`, `watch`, `send`, `type`, `tab state`, `tab label`, `tab kill`, and
`tab move` continue to reach a logical tab in every placement.

`internal/tabstate` owns lifecycle state (`running`, `done`, `failed`,
`attention`) and formats the bar glyphs. State is pane-canonical, so it follows
the tab when the tab moves between full, pane, and dock placements.

`internal/tabs/reap.go` is the tab reaper. `PlanReap` is pure: it classifies
every scanned row (origin/scope/age/idle/live) into keep/flag/kill/adopt with no
tmux writes. `ApplyReap` is the executor — it scans, plans, then treats kills as
advisory: each is re-validated against a fresh second scan and pane-exact option
reads before `KillWindowByID`, reserving every session group's last window and
vetoing panes with live foreground or background work. Lifecycle stamps
(`internal/tabs/lifecycle.go` — origin/scope/born/stale_at, set-once and
pane-scoped) feed the classifier; `internal/cli/reap.go` exposes the `zmux reap`
command and a throttled lazy sweep (`MaybeReap`) wired into `ls`/`tabs`/`run`
and the `client-attached`/`session-created` hooks baked by `internal/tmux/conf.go`.
Origin inheritance is seeded by `zmux tab mark-agent` (`tabs.MarkAgentShell`),
which the zmux skill's session-start hook calls to tag an agent's own shell
`scope=agent-shell` — tabs that shell spawns then inherit `origin=agent` and the
short agent TTL, while the shell itself is never reaped. For driving and
grounding the reaper's time-gated behaviour, see
[agent-grounding.md](agent-grounding.md).

### QA walkthrough runner

`cmd/qa` is a separate repo-local binary invoked by the root `./qa` wrapper. It
is deliberately not a `zmux qa` subcommand because QA is a repo-maintenance
surface rather than a tmux-management verb.

Checklist specs are committed under `checklists/*.toml`. Scorecards and the
cached built runner live under gitignored `.qa/`. The framework supports both
automatic shell-check steps and human-verdict steps through `internal/tui/qapicker`.
See [qa.md](qa.md).

### TUI layout

```
internal/tui/                 — no flat package; focused leaves + surfaces
├── styles/                   — shared lipgloss styles leaf
├── workspaceview/            — workspace-view data adapter (picker + dashboard)
├── workspaceoutline/         — shared workspace/session/external row structure (picker + dashboard); surface differences via Policy callbacks
├── workspacelist/            — reusable workspace list component
├── wspicker/                 — no-prefix workspace switcher
├── picker/                   — primary session/workspace picker
│   ├── picker.go             — model + Update
│   ├── picker_view*.go       — view rendering (list / help splits)
│   ├── picker_actions.go     — selection/CRUD actions
│   └── keymap.go             — picker-local keymap (component keys stay local)
├── tabpicker/                — Alt+` tab switcher (logical full/rider/hidden rows)
├── themepicker/              — standalone theme picker
├── qapicker/                 — QA walkthrough picker for ./qa
├── tkey/                     — shared key helpers
├── wizard/                   — zmux init setup wizard
├── recipeup/                 — recipe create/edit form wizard
├── outline/                  — tree-outline subcomponent
├── views/                    — shared row/column components
├── scroll/                   — shared viewport scrollbar renderer (dashboard + help)
├── helpview/                 — prefix+? interactive help viewer (scroll / fuzzy / scope)
├── palette/                  — command palette
│   ├── registry.go           — ActionProvider interface
│   ├── executor.go           — runs selected action
│   └── providers/            — built-in palette actions
└── dashboard/                — tabbed popup (prefix+Space)
    ├── tab.go                — Tab interface + msg routing
    ├── app.go                — DashboardApp (bubbletea Model)
    └── tabs/
        ├── current.go        — Session & Workspace tab (active workspace's sessions)
        ├── sessions.go       — Workspaces tab (all workspaces)
        ├── themes.go         — Themes tab (pick/clone/edit)
        ├── bar.go            — Bar tab (preset + segment toggle)
        ├── settings.go       — Settings tab (config fields)
        ├── help.go           — Help tab (renders from internal/keys)
        └── shared_*.go       — rename/confirm overlays + workspace/session mutation helpers shared across tabs
```

Tabs all implement `dashboard.Tab` — `Activate`, `Deactivate`, `Update`, `View`. Cross-tab messages use `dashboard.TargetedMsg` / `AppIntentMsg`.

### Configuration

`~/.zmux.toml` is the user's TOML config. `internal/config` loads it via `FS`
so tests use in-memory fakes. Themes resolve against three sources in order:
user custom (`~/.zmux/themes/`), bundled (`go:embed` from
`internal/theme/bundled/`), downloaded iterm2 (`themes/iterm2/`).

### Generated tmux config

`internal/tmux/conf.go` emits the generated tmux config from the user's
`config.Config` and the resolved `theme.Palette`. For the live `zmux` profile
that path is `~/.tmux.conf`; for the edge `zzmux` profile it is `~/.zzmux.conf`.
This is the file `zmux apply` writes. Hooks, status hooks, and bar hooks are
generated here; the **keybindings come from the `internal/keys` registry**
(`conf.go` references `keys.X.Key`), so the generated config, help surfaces,
and `docs/keybindings.md` never drift.

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
├── refresh           — apply config and refresh the current tmux client
├── new               — create a workspace plus local session labels
├── open              — open a workspace/session (aliases: attach, a)
├── kill              — smart workspace/session kill
├── ls                — list workspaces or local session labels
├── tabs              — list tabs in current or targeted workspace/session
├── where             — current context: workspace/session/tab/pane/cwd (alias: whoami)
├── tab               — tab management (label, move, kill, state, hide/show/pane/full)
├── pane              — pane management (open/toggle/current/list/focus/resize/close)
├── send              — send keys to a window
├── type              — type text into a window
├── watch             — read window output
├── run               — run a command in a named window
├── recipe            — recipe list/show/lint/fork/edit/create
├── workspace         — workspace management subtree
├── session           — session management subtree
├── theme             — theme set/list/sync/pull
├── bar               — bar preset commands
├── terminal          — current/capabilities/refresh probes
├── snapshot          — per-pane text/ANSI plus optional PNG evidence bundle
├── log               — start/stop/status/tail tab output recording (pipe-pane → bounded sink)
├── log-sink (hidden) — internal stdin→bounded-file sink the pipe feeds
├── help              — top-level help (prefix+? popup)
├── status            — internal status JSON
├── completion        — shell completion (cobra-generated)
└── keys (hidden)     — maintainer tooling: `keys gen` regenerates docs/keybindings.md from internal/keys
```

The separate repo-local QA runner is outside the zmux tree:

```
./qa
├── ls                — list checklists + scorecard summaries
├── run               — run automatic checklist steps
├── mark              — record a human/agent verdict
├── status            — print scorecard summary or JSON
├── reset             — clear a checklist scorecard
└── lint              — validate checklist files
```

Plus a handful of UI launchers triggered by tmux bindings:

```
zmux --picker         # workspace+session picker
zmux --palette        # command palette
zmux --dashboard      # tabbed dashboard
zmux --tab-picker     # Alt+` tab switcher
zmux --workspace-picker # Alt+w workspace switcher
```

These flags live in `root.go` and route into the corresponding `tui/` flow.

---

## Where to make common changes

| Want to… | Start in |
|----------|----------|
| Add a new CLI subcommand | `internal/cli/<name>.go` (cobra) — register in `internal/cli/root.go` |
| Add a tmux keybinding | `internal/keys` — add the `Binding` to the right table. `conf.go`, the dashboard Help tab, `zmux help`, and `docs/keybindings.md` all derive from it. Run `make keys-gen` to regenerate the doc (the `TestKeybindingsDocInSync` golden test / CI enforces freshness) |
| Add a dashboard tab | Implement `dashboard.Tab` under `internal/tui/dashboard/tabs/`. Register in `dashboard.App`. See an existing tab (e.g. `themes.go`) as a template. |
| Change logical-tab placement or state | Start in `internal/tabs/` or `internal/tabstate/`, then update the shared resolver in `internal/cli/tab_target.go` if targeting behavior changes. Run `./qa lint` and the relevant checklist under `./qa`. |
| Add a QA walkthrough | Add `checklists/<name>.toml`, validate with `./qa lint`, and document any human-only expectations in the checklist's `expect` fields. |
| Add a bar preset | `internal/bar/bar.go` — preset table + segment definitions. Update preview in `internal/bar/preview.go` if visual changes need preview. |
| Add a theme | Drop a new file into `internal/theme/bundled/` (iterm2-color-schemes format). It will be `go:embed`'d on next build. |
| Change a generated tmux behavior | `internal/tmux/conf.go` — but verify `internal/tmux/conf_test.go` still covers the new section. |
| Change session/workspace behavior | `internal/workspace/` (workspace-level), `internal/session/` (per-session), and `internal/cli/session_target.go` for CLI targeting. Run integration tests (`make test-integration`) — they exercise the built `zmux` CLI end-to-end (binary output, not real tmux). |

---

## Conventions enforced in AGENTS.md

A few rules worth re-stating here because they're easy to miss:

- **All side effects behind interfaces.** Direct `os/exec`, `os.ReadFile`, `http.Get` in business logic is rejected at review time. Use or introduce a typed interface.
- **Tests use `tmux.MockRunner`.** Never spin up real tmux in unit tests. Integration tests under `tests/` (build tag: `integration`) exercise the built `zmux` binary's CLI end-to-end (pure-output commands; no real tmux). Real-tmux coverage, if needed, belongs in future tmux-dependent integration tests.
- **Don't run `zmux init` inside tmux.** It refuses; this is intentional, not a bug.
- **Explicit DI, no package-global `app`** — `app.App` (in `internal/app`) is built once in `main` (`app.New()`) and threaded through `NewRootCmd(app)`. Each command is a `newXCmd(app)` constructor capturing it; flag state is constructor-local.
- **Session-group clones** collapse to root in `ListSessions()` and `WorkspaceFor()`. Managed sessions use `__clone_b`-style raw clone names so local labels ending in `-b` are not misread as clones.

---

## Out of scope for this doc

- Detailed product vision — see [VISION.md](VISION.md)
- Keybinding reference — see [keybindings.md](keybindings.md)
- Terminal capability matrix — see [terminal-capabilities.md](terminal-capabilities.md)
- Pi extension API — see [pi-zmux-extension.md](pi-zmux-extension.md)

---

## When this doc drifts

If a package map row, interface table, or "where to make changes" line is wrong, fix it in the same PR that introduced the drift. The contributor expectation is that this file matches reality.
