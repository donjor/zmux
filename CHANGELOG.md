# Changelog

Notable changes, newest first. Forward work lives in
[docs/ROADMAP.md](docs/ROADMAP.md). Format: [Keep a Changelog](https://keepachangelog.com);
versioning is semver-ish until the first public release.

## [Unreleased]
> Release tag: pending | Topics: `agents`, `workspace` | Compare: `v0.8.0...HEAD`

### Added

- **`zmux session run`** `agents` - create a detached session in the current (or
  `--workspace`) workspace and launch a command as its first/only tab — no focus
  steal, no blank shell tab. The orchestration-safe worker-spawn primitive
  (`tmux new-session -d -n <tab>`); the `agent-worker` and `orchestrate` doctrines
  now teach it instead of the attach-oriented `zmux new <ws> <session>`.
- **Workspace-local session identity** `workspace` - workspaces now store v3
  session records with local labels, stable IDs, generated raw tmux names, and
  `@zmux_*` tmux metadata. Commands accept `workspace/session` targets while
  normal UI surfaces display local labels instead of raw `zws_...` names; legacy
  v1/v2 sessions are repaired into managed names and clone-like labels such as
  `worker-x` stay valid.

## [0.8.0] - 2026-06-09
> Release tag: `v0.8.0` | Topics: `watch`, `tabs`, `qa`, `recipes`, `agents`, `docs` | Compare: `v0.7.0...v0.8.0`

### Added

- **Idle watch mode** `watch` `agents` - `zmux watch --idle` supports buddy-style
  CLI integration and agent-driver workflows.
- **Logical tab placement** `tabs` - logical tabs gained pane/full placement,
  docking, hidden tabs, state glyphs, MRU repair, and targeting through
  `run`, `watch`, `send`, `type`, `state`, `label`, `kill`, and tab movement.
- **Repo-local QA runner** `qa` - `./qa`, checklist parsing/linting, scorecard
  state, command checks, verdict overrides, and a picker flow for human QA.
- **Recipes** `recipes` - session templates were replaced with recipe-driven
  workflows and updated user docs.
- **Agent worker doctrine** `agents` - the shipped zmux skill now documents
  read-only peers and write-capable workers, including Codex, Claude, Pi, and
  Antigravity launch profiles.

### Changed

- **Agent docs alignment** `docs` `agents` - README and architecture docs now
  describe peer and worker doctrine instead of peer-only integration.
- **Shared skills install** `dev` - maintainer install now aligns with the
  shared skills setup.
- **Session switching keys** `keys` - prefix Alt digits switch sessions.

### Fixed

- **Cross-profile safety** `tmux` - default endpoint access refuses mismatched
  `$TMUX` sockets and surfaces cross-profile errors through degraded listings.
- **QA verdict retention** `qa` - settled human verdicts are preserved unless
  explicitly forced.
- **Logical tab row chrome** `bar` `tabs` - tab-row spacing and state glyphs were
  polished.

## [0.7.0] - 2026-06-04
> Release tag: `v0.7.0` | Topics: `bar`, `picker`, `tabs`, `guard`, `agents`, `docs` | Compare: `v0.6.0...v0.7.0`

### Added

- **Bar and picker QoL** `bar` `picker` - rounded caps, profile badges,
  always-two-line bar ownership, picker hints, inline delete confirmation, and
  two-level session/tab switching.
- **Agent terminal guard** `guard` `agents` - `zmux-guard` redirects raw tmux
  commands in managed sessions and shares the terminal-hygiene corpus.
- **Buddy-via-zmux design** `agents` `docs` - the agent workflow was ratified
  around `watch --idle`, LLM-as-adapter behavior, and no extra orchestrator
  above zmux.

### Fixed

- **Tab targeting** `tabs` - `run`, `send`, and `type` resolve tabs by
  `@zmux_label` instead of being confused by tmux autorename.
- **Status refresh hooks** `bar` - hooks guard against missing clients and route
  through the active profile binary.

## [0.6.0] - 2026-05-26
> Release tag: `v0.6.0` | Topics: `architecture`, `ci`, `snapshot`, `zzmux`, `tui`, `docs` | Compare: `v0.5.0...v0.6.0`

### Added

- **Public repo prep** `docs` - contributing docs, architecture map, roadmap
  cleanup, and legacy v0 relocation.
- **CI and local gates** `ci` - GitHub Actions, race/vuln/integration gates,
  gofumpt, golangci-lint v2, and a master-scoped pre-push hook.
- **Snapshot evidence** `snapshot` - native terminal text, ANSI, and strict PNG
  capture for review evidence.
- **`zzmux` edge profile** `zzmux` - binary-name-isolated dogfooding profile
  with separate socket, config, state, and generated tmux conf.

### Changed

- **Architecture refactor** `architecture` - command implementation moved under
  `internal/cli`; side-effect-heavy behavior moved behind typed boundaries.
- **TUI stack** `tui` - migrated to Charm v2 and adopted `charmbracelet/log`.
- **Source probing** `architecture` - source discovery and B-purity seams were
  closed after follow-up refactors.

## [0.5.0] - 2026-05-05
> Release tag: `v0.5.0` | Topics: `workspace`, `panes`, `terminal`, `pi`, `tabs` | Compare: `v0.4.0...v0.5.0`

### Added

- **Session tab rework** `workspace` - session navigation, viewport labels,
  grouped clone labels, and status/dashboard viewer counts.
- **Pane workflows** `panes` - pane split, focus, current toggle, dashboard rows,
  pane border styling, respawn binding, and visual previews.
- **Terminal evidence and refresh** `terminal` - strict terminal targeting,
  stable tab labels, truecolor fixes, RGB refresh, and a top-level refresh
  command.
- **Pi extension** `pi` - typed zmux tools, typed pane key tools, respawn/reload
  flows, prompt detection, guardrail bypass, and interactive wait handling.

### Fixed

- **Run cleanup race** `agents` - cleanup and procfs behavior gained coverage.
- **Tab/pane lifecycle** `tabs` `panes` - Ctrl-C no longer implicitly closes
  tabs, duplicate cwd suffixes only render when needed, and dead panes are kept
  only for failed exits.

## [0.4.0] - 2026-04-08
> Release tag: `v0.4.0` | Topics: `workspace`, `dashboard`, `outline`, `tests`, `detox` | Compare: `v0.3.0...v0.4.0`

### Added

- **Workspace-primary picker** `workspace` - picker behavior moved to the
  workspace-primary flat-list model.
- **Shared outline tree** `outline` - picker, dashboard, theme picker, and tab
  picker were migrated toward shared tree rendering.
- **Focused tests** `tests` - themes and palette packages gained meaningful
  coverage during detox.

### Changed

- **Dead shim removal** `detox` - v1 workspace shims, stale aliases, dead
  renderers, dead fields, and unused helpers were removed.
- **Focused TUI files** `tui` - dashboard, themes, wizard, tab helpers, and
  request counters were split or consolidated.

### Fixed

- **Bar rendering** `bar` - clock toggles and per-client status-left behavior
  were corrected.

## [0.3.0] - 2026-04-07
> Release tag: `v0.3.0` | Topics: `keys`, `tabs`, `workspace`, `bar`, `docs` | Compare: `v0.2.0...v0.3.0`

### Added

- **Keybinding system** `keys` - tab reordering, close/previous bindings,
  Alt-number tab switching, Alt-backtick switcher, and keybinding source docs.
- **Workspace objects** `workspace` - first-class workspace/session navigation,
  session tagging, picker grouping, and workspace v2 docs.
- **Dynamic status bar** `bar` - git, workspace, prefix hints, language, clock,
  dir, process, group segments, live preview carousel, and new presets.

### Changed

- **Bar rendering** `bar` - status-left rendering went through native/tmux-format
  experiments and settled on simpler per-window rendering.
- **Run command safety** `run` - commands are written through temporary scripts
  and echoed in target tabs.

### Fixed

- **Hotkey consistency** `keys` - detach, close, kill, and settings navigation
  keys were normalized.
- **Workspace nil guards** `workspace` - tab move and workspace type nil guards
  were corrected.

## [0.2.0] - 2026-03-20
> Release tag: `v0.2.0` | Topics: `install`, `picker`, `dashboard`, `sessions`, `agents`, `docs` | Compare: `v0.1.0...v0.2.0`

### Added

- **Installer and init wizard** `install` - shell integration, automatic init,
  inside-tmux refusal, clipboard copy, and cleaner success flows.
- **Session picker and dashboard** `picker` `dashboard` - premium picker UX,
  session commands, live prompts, grouped sessions, hijack mode, settings, and
  command palette.
- **Agent command set** `agents` - `run`, `send`, `watch`, `type`, `tabs`,
  `--follow`, `--until`, wait sentinels, and routing tests.
- **Repo-owned skill docs** `agents` - the zmux skill became repo-owned and
  synced by install/dev scripts.

### Fixed

- **Apply/source loop** `tmux` - source-file behavior and infinite apply loops
  were corrected.
- **Watch baseline matching** `watch` - `watch --until` ignores baseline output
  so old text cannot satisfy new waits.

## [0.1.0] - 2026-03-16
> Release tag: `v0.1.0` | Topics: `core`, `themes`, `tmux`, `bar`, `tui`, `tests` | Compare: initial

### Added

- **Go CLI rewrite** `core` - the Go implementation added a Cobra command tree,
  typed internal packages, unit tests, and integration tests.
- **Theme system** `themes` - bundled themes, Ghostty/iTerm theme loading,
  semantic theme roles, Neovim/Ghostty sync, and palette resolution.
- **tmux config management** `tmux` - generated tmux config, apply/source flows,
  tmux runner boundary, clipboard handling, and parser tests.
- **Status/bar UI** `bar` - status/bar commands, ANSI previews, preset picker,
  and bar generation tests.
- **Bubble Tea TUIs** `tui` - picker, dashboard, command palette, theme picker,
  wizard, shared views, and style primitives.
