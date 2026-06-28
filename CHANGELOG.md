# Changelog

Notable changes, newest first. Forward work lives in
[docs/ROADMAP.md](docs/ROADMAP.md). Format: [Keep a Changelog](https://keepachangelog.com);
versioning is semver-ish until the first public release.

## [Unreleased]
> Release tag: pending | Topics: `agents`, `palette`, `help`, `panes`, `tabs` | Compare: `v0.9.0...HEAD`

### Added

- **Interactive `prefix+?` help viewer** `help` - the help popup is now a
  scrollable, fuzzy-filterable viewer (`internal/tui/helpview`) rendering from a
  shared `internal/help` source, so it can't drift from `zmux help`. `Tab` cycles
  a commands/keys/all scope filter; prefix keys are grouped by category with
  aligned columns and titled as zmux's own — only genuinely inherited tmux
  defaults keep the "from tmux" label. A shared scrollbar renderer
  (`internal/tui/scroll`) is reused by the dashboard tabs and the viewer.
- **Command palette reaches every keybound action** `palette` - the palette is
  backed by an action registry (`internal/actions` + palette providers) with
  keybound, session, and logical-tab providers and a typed tmux executor, so
  anything a prefix key does is runnable from `prefix+p`. Logical-tab rows
  (hide/show/promote/join) run through the shared placement service; session rows
  show the workspace label, not the raw `zws_` name. A coverage gate pins the
  palette to the keybinding registry so a new binding can't go palette-less.

- **Readable pane-border headers + single-pane detail** `panes` - joined panes
  render `<index> <name> <detail>` with an active `●` / inactive `○` marker,
  dropping the raw `%id` and `WxH` size noise; a lone pane surfaces its title in
  the top bar, and the prefix helper hints grow a pane-layout cluster when a tab
  is split.
- **Pane layout-control keys** `panes` - `prefix+Shift+Arrow` swaps a pane with
  its directional neighbour, `prefix+=` equalizes the split, and `prefix+s`
  toggles split orientation (horizontal↔vertical), reclaimed from the
  session-picker alias (`prefix+w` still opens the picker). `prefix+Alt+Arrow`
  resize now repeats while the prefix is held (`repeat-time` 500).
- **Join a tab by index** `tabs` - `zmux tab pane <N>` joins the tab at bar
  index N as a pane (opt-in, placement-only); a tab literally labelled with that
  number still wins.

### Fixed

- **Keybind join/promote no longer takes over the screen** `tabs` - `prefix+J`
  (join) and `prefix+F` (promote) run via tmux `run-shell`, which hijacks the
  screen with a keypress-to-dismiss view on any stdout or non-zero exit — so a
  failed join showed `'… tab pane "2"' returned 1` and even a successful one
  dumped its line. A new `--notify` flag flashes the outcome on the status line
  via `display-message -l` (literal, so a `#(...)` tab label can't execute) and
  exits 0; the direct CLI keeps stdout and real exit codes.
- **Interactive sessions' first window is now a joinable tab** `tabs` -
  interactive session creation (`new`, picker, popup, palette) never stamped the
  first window's pane with `@zmux_tab_id`, so joining another tab *into* it
  (`tab pane`, `prefix+J`) failed with "current window is not a zmux tab".
  `session.Create` now stamps it at creation, matching the worker `session run`
  path.
- **Cross-session tab resolution no longer leaks for mutating commands**
  `agents` - tab names are per-session unique, but the server-wide "unique
  elsewhere" convenience in `tabs.Resolve` let a mutation resolve and act on a
  same-named tab in another session (confirmed live: `run -n codex-peer -s A`
  typed into a different session's `codex-peer`). Mutating verbs (`run`, `send`,
  `type`, `tab` state/placement/kill, `log start/stop`) now resolve
  `SessionOnly` — an out-of-session match is dropped to the in-session
  window/raw fallback, so `run` creates in-scope and `send`/`kill`/`log` surface
  a clean miss. Read-only/explicit-cross verbs (`watch`, `log tail`, `tab show`,
  `tab move`) keep the cross-session reach with a warning. ID lookup prefers an
  in-scope row so session-group clones (same-ID rows per clone session) still
  resolve clone-local.

## [0.9.0] - 2026-06-21
> Release tag: `v0.9.0` | Topics: `agents`, `workspace`, `dashboard`, `ui` | Compare: `v0.8.0...v0.9.0`

### Added

- **Tab reaper — auto-expire stale tabs by lifecycle policy** `agents` - a new
  `zmux reap` classifies every tab (origin/scope/age/idle/live) and applies a
  verdict: adopt unborn pre-existing tabs, flag stale ones, kill those an earlier
  sweep already flagged. Agent-created task tabs are litter on a short clock
  (~1h idle); human/pre-existing tabs get a long, visible ramp (flag at 4h, kill
  at 24h). Kills are re-validated pane-exactly against a fresh scan, never drop a
  session's last window, and skip panes with a live foreground process or
  background job. Runs lazily (throttled) from `ls`/`tabs`/`run` and from baked
  `client-attached`/`session-created` tmux hooks; `zmux reap --dry-run` previews
  and nothing is killed without an earlier flag. Protect a tab with
  `run --keep` or `run --scope daemon`; a fresh ad-hoc `run -n <name>` nudges you
  to reuse a roster tab (e.g. `scratch`) instead of accumulating litter. The
  zmux skill's session-start hook tags an agent's own shell `scope=agent-shell`
  (itself never reaped), so tabs the agent spawns inherit `origin=agent` and the
  short agent TTL automatically — no per-command flag or env var needed.
- **Independent per-client views when switching sessions in a workspace**
  `workspace` - switching between a workspace's sessions while already attached
  (prefix+alt+N, session cycle, command palette, dashboard pick) now gives each client its own
  viewport instead of collapsing every attached client onto one shared
  window/pane. When the target session is already attached elsewhere, the switch
  lands on a fresh session-group clone — the same independence you already get
  opening a workspace in a new window; tab select/select-pane target that clone,
  not the shared root. Clones zmux creates carry a `@zmux_clone` marker so the one
  a client leaves is garbage-collected once clientless, while a session you
  grouped by hand (`tmux new-session -t foo -s foo-b`) is never touched.
- **`zmux where` (alias `whoami`) — one-shot current context** `agents`
  `workspace` - report the current context as a single answer: workspace,
  session (local label + raw `zws_…` tmux name), tab, pane id, and cwd. Composes
  the same identity other verbs resolve, so you know what to pass as `-s` without
  parsing the session name. `--json` for tooling. Complements `pane current`
  (pane facts) and `status` (config); it does not replace either.
- **`zmux log` — tail-style output recording** `agents` - record a tab's output
  stream to a bounded file in the background, distinct from one-shot `snapshot`
  and interactive `watch`. `log start <tab>` opens a `tmux pipe-pane` into a
  hidden `zmux log-sink`; recording runs server-side and survives detach.
  `log status` lists active recordings, `log tail <tab>` prints the log, and
  `log stop <tab>` ends it. The sink keeps only the trailing `--max-bytes`
  (default 1 MiB; oldest dropped) so disk never runs away, and strips ANSI for a
  readable plain log unless `--ansi` is passed. Built for line-oriented output
  (servers, builds, tests); fullscreen TUIs log as escape soup — use `snapshot`
  for screen state and `watch -f` for live following.
- **Picker ↔ dashboard convergence on shared workspace/session logic**
  `dashboard` `ui` `workspace` - the workspace+session picker and the dashboard
  Workspaces tab no longer reimplement the same listing/creation logic and drift
  apart. Three outcomes land together: (1) bare `zmux` outside tmux always opens
  the picker, even on a fresh reboot with no live session — the sessionless
  dashboard is now only the auto attach-fallback (close-last-session / vanished
  target), not the explicit invocation — a regression introduced by the earlier
  sessionless-fallback work;
  (2) the dashboard can create a new session under a workspace, matching the
  picker's validation, naming, and attach behavior; (3) both surfaces share one
  row builder (`internal/tui/workspaceoutline`, surface differences via `Policy`
  callbacks) and one create path (`workspace.CreateManagedSession`), so they
  produce identical addressable `zws_<workspace>__<label>` sessions.
- **Session tab search, quick-jump, and pinned scope** `dashboard` `ui` - the
  dashboard's Session & Workspace tab gains the search model from the Workspaces
  tab, scoped to the active workspace: `/` filters sessions by session name or
  any tab name, `1`–`9` quick-jump to (and activate) the nth visible session
  with matching `[N]` badges, and the `N sessions in <ws>` scope cue is pinned
  above the list so it stays visible while rows scroll.
- **Popup focus framing** `ui` - tmux popups (dashboard, palette, pickers) get a
  faint themed border (`popup-border-style` in the palette's dim color) and a
  best-effort host-background dim while a popup is open, restored on close. The
  dim degrades to a no-op outside tmux or without a resolved theme; a hard kill
  of the popup self-heals on the next `apply`/attach.
- **Guard catches nested tmux/background forms** `agents` - the shared classifier
  corpus and all three guards (Go, Claude hook, pi) now recursively scan the
  payloads a segment executes — `sh -c '…'`, `env`/path-prefixed shells,
  `xargs tmux …`, and shell-fed here-doc bodies — so a raw tmux or background
  job one indirection deep is still flagged. Command substitution stays a
  documented gap.
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

### Fixed

- **Top bar drops dead sessions and repaints the tabs row on attach** `ui` -
  killed sessions disappear from the bar at the next render instead of lingering,
  and the second (logical tabs) status line renders immediately on attach rather
  than staying blank until the next tab event.
- **Picker delete keeps the cursor in place** `ui` - deleting a row in the
  outside-tmux picker now lands the cursor on the nearest stable neighbor so
  repeated cleanup stays put, via a shared outline neighbor primitive.
- **`watch --lines` bounds capture height** `agents` - a bounded screen-window
  request now tail-trims the plain capture to the requested line count without
  affecting snapshot/full-capture or idle-detection paths.
- **`kill` addresses managed sessions by target** `workspace` - `zmux kill
  workspace/session` (and bare workspace-local labels) now routes through the
  shared session-target resolver instead of only matching raw tmux names, so a
  managed session is killable by the same address `run`, `send`, and `watch`
  use. `kill -y/--yes` skips the workspace teardown confirmation for scripted
  and agent use.
- **Sessionless dashboard fallback** `workspace` - `zmux` stays usable when no
  workspace or session is active. A zmux-owned attach whose session disappears
  reattaches to the workspace's best remaining live session; with none left it
  lands in the sessionless dashboard with a clear exit path instead of an opaque
  error or dead terminal. The tab-picker `close` action gains the same last-window
  guard `zmux tab kill` already enforces. Bounded retries with seen-set
  termination, `RootName`-aware for grouped sessions, and a `detach-on-destroy`
  precondition check.

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
