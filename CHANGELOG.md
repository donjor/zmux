# Changelog

Notable changes, newest first. Forward work lives in
[docs/ROADMAP.md](docs/ROADMAP.md). Format: [Keep a Changelog](https://keepachangelog.com);
versioning is semver-ish until the first public release.

## [Unreleased]
> Release tag: pending | Compare: `v0.13.0...HEAD`

### Added

- **Readable Pi dispatcher calls and results** `agents` `pi` — the single `zmux` tool now renders compact operation, destination, lifecycle, and evidence summaries with expanded raw metadata, narrow-terminal wrapping, and sensitive-input redaction. Wait and callback completions also keep large output tails out of model-visible results while retaining raw diagnostics in the expanded wait view.

### Fixed

- **Lightweight live Pi regression routing** `agents` `pi` `qa` — the canonical Terra/medium host flow now uses detached visible workers, exact test-owned cleanup, regex-safe peer callbacks, bounded one-shot execution, unambiguous focus and literal-pane-input prompts, and targeted guidance for named-pane resolution.

## [0.13.0] - 2026-07-11
> Release tag: `v0.13.0` | Topics: `agents`, `pi`, `panes`, `release` | Compare: `v0.12.0...v0.13.0`

### Changed

- **Canonical Pi tool is now simply `zmux`** `agents` `pi` -- the accepted 40-operation dispatcher is the sole `pi-zmux` model surface. The experimental suffix and retired 37-tool implementation modules are gone; `~/donjor/zmux/pi-zmux` is the implementation source of truth.
- **Pi reads live zmux state only on demand** `agents` `pi` -- `pi-zmux` no longer injects pane, tab, runtime, and config state into every agent run. The one-tool schema remains always available, dispatcher operations resolve the live state they need, and `/zmux status` retains the full human diagnostic snapshot.

### Fixed

- **Background agent tabs resolve their complete current-pane row** `agents` `panes` -- `zmux pane current --json` now targets the caller's `$TMUX_PANE` instead of the attached client's active window, preserving session, window, cwd, and process metadata for Pi and peer tabs.

## [0.12.0] - 2026-07-08
> Release tag: `v0.12.0` | Topics: `agents`, `pi`, `tabs`, `wait`, `qa`, `docs` | Compare: `v0.11.2...v0.12.0`

### Added

- **Core agent wait surface** `agents` `tabs` -- `zmux wait`, `zmux tab inspect`, `zmux tab peer ensure`, and `zmux type --wait-* --json` now make fresh command/turn waits, output-regex waits, idle fallback, status snapshots, and safe peer create/reuse first-class CLI behavior.
- **Pi callback and peer-handoff tools use core waits** `agents` `pi` -- `zmux_callback` starts, lists, and cancels live-session-scoped `zmux wait --json` callbacks that notify Pi when a visible tab matches future output or goes idle; `zmux_peer_handoff` types a peer prompt and schedules the same handoff without agent-side sleeps while reporting the evidence basis.
- **First-class peer/tab inspection composites** `agents` `pi` -- `zmux_tab_inspect`, `zmux_peer_ensure`, wait-aware `zmux_type`, and wait/idle `zmux_runtime_logs` are thin adapters over the core CLI surfaces and bundle status, output tail, freshness checks, warnings, and failure kinds so agents do not hand-roll `status`/`watch` loops.
- **Branch-local agent-surface QA prompts** `agents` `qa` -- `docs/dev/test-prompts/` now carries fresh-session prompts for shared skill/CLI and Pi extension smoke testing against the isolated `zzmux` profile.

### Changed

- **Agent peer launch doctrine is role-scoped** `agents` `skills` -- peer and worker references now point Pi at deterministic role-specific launch templates instead of ad hoc command rows.
- **Core CLI internals are slimmer and more reusable** `core` `tabs` `bar` -- command helpers, tab/session targeting, bar rendering, and shared TUI/filter primitives were consolidated while retired preview/theme-download code was removed.

### Fixed

- **Pi peer lifecycle readiness is restored** `agents` `pi` -- the extension now publishes and waits on peer lifecycle state through dedicated lifecycle helpers, so fresh peer turns can be detected reliably.
- **Remote tab sprawl is detected earlier** `agents` `pi` -- Pi zmux tools now warn on likely remote-tab/session sprawl and the guard fixtures cover the generalized remote-command shape.
- **Developer skill sync uses the canonical reconciler** `dev` `skills` -- `./dev.sh` routes skill mirrors through the shared sync path instead of carrying a parallel mirror implementation.

## [0.11.2] - 2026-07-05
> Release tag: `v0.11.2` | Topics: `shell`, `agents`, `pi`, `tabs`, `panes`, `qa` | Compare: `v0.11.1...v0.11.2`

### Added

- **Shell integration freshness doctor** `shell` - `zmux doctor` / `zzmux doctor` now checks the managed rc block, bash login bridge, retired `ble-attach` path, and the current already-open shell's loaded hook version. `setup shell` prints a fresh-shell hint when files are current but the active shell is still running stale hooks.
- **Agent-surface regression gate** `agents` `pi` `skills` - `make test-agent-surfaces` now runs core lifecycle tests, Pi extension typecheck/tests, QA lint, and a shipped zmux skill doctrine doctor so typed tools and agent docs cannot silently drift.
- **Interactive shell QA coverage** `qa` `shell` - the natural-shell checklist now proves real stdout emission, running/done status metadata, interactive Claude launch under ble.sh, and no `[ble: exit 1]` regression on fresh `zzmux` shells.

### Changed

- **`zzmux` edge installs are binary-only by default** `shell` `qa` - `./dev.sh zzmux` no longer mutates live shell startup or shared agent integration state; live shell activation remains an explicit `setup shell` step after edge proof.
- **Agent pane placement is focus-safe by default** `agents` `panes` `tabs` - Pi pane/tab placement tools default to no focus steal, while human keybinding/palette paths keep selecting the pane they create or rejoin.

### Fixed

- **ble.sh lifecycle hooks no longer break interactive TUIs** `shell` - the managed bash lifecycle block no longer forces `ble-attach`, preventing typed interactive CLIs such as `claude` from immediately returning `[ble: exit 1]`.
- **Root-shell lifecycle setup is idempotent through bash login bridges** `shell` - repeated `.profile`/`.bashrc` setup in one shell process no longer double-registers hooks, and nested foreground shells are kept out of the parent pane lifecycle.
- **Lifecycle wait failures now point at the doctor** `agents` `shell` - `zmux run` timeout guidance now tells agents to run `zmux setup doctor` before reinstalling or debugging stale shell hooks.

## [0.11.1] - 2026-07-01
> Release tag: `v0.11.1` | Topics: `agents`, `pi`, `tabs`, `skills` | Compare: `v0.11.0...v0.11.1`

### Added

- **Status-first agent lifecycle surface** `agents` `tabs` - `zmux tab status --json` now exposes `cmdSeq` for fresh command completion and `turnAt` for peer-turn freshness, giving agents a structured state surface instead of relying on terminal settle loops.

### Changed

- **Pi interactive commands use lifecycle status** `agents` `pi` - `zmux_interactive_type` no longer creates temp wrapper/status files for command completion; it reads a baseline `cmdSeq`, types the command, and waits for a fresh `done`/`failed` command state while preserving sudo/password/SSH prompt detection. `zmux_runtime_ensure` now returns ensure/readiness state without unconditionally tailing logs.
- **Agent peer/worker doctrine is status-first** `agents` `skills` - `skills/zmux` now teaches `tab status` / Pi `zmux_tab_status` as the state source, with `watch`, logs, and snapshots reserved for output, evidence, startup/submission hygiene, and uninstrumented fallback.

### Fixed

- **Missing tab lifecycle reads fail closed** `agents` `tabs` - `tab status`, `tab state`, and `tab peer` now error on missing tab-name targets instead of falling through to a raw tmux target that could affect the current pane.

## [0.11.0] - 2026-07-01
> Release tag: `v0.11.0` | Topics: `agents`, `tabs`, `panes`, `workspace`, `session` | Compare: `v0.10.0...v0.11.0`

### Added

- **Natural shell lifecycle glyphs** `tabs` `agents` - `zmux setup shell` now installs root-shell lifecycle hooks so normal typed foreground commands publish running/done/failed glyph state and command metadata. `zmux run` waits on the same silent pane-option result channel instead of printing `:::AGENT_DONE` sentinels or appending `tab state-exit` epilogues, and `zmux tab status [--json]` exposes tab/command/peer status for tooling.
- **Pi zmux typed surface parity** `agents` `pi` - `pi-extension/` now exposes typed tools for reviewable `zmux run` one-shots, workspace/session listing and worker-safe `session run`, tab state/peer/label/move/placement/status, persistent `zmux log`, terminal snapshots, and terminal-current inspection. The Pi bash guard redirects direct `zmux`/raw `tmux` slips to those tools, while `zmux_run` preserves native `zmux run` wait behavior and reports command exits structurally.
- **Workspace session lifecycle commands** `workspace` `session` - `zmux fork <new-session-label> [--dir]` copies the current workspace session's tab names/order into a clean managed session without replaying commands, cloning panes, or coupling to Worktrunk. `zmux open ... --pin-view` creates a persistent grouped viewport over the target session, keeps root workspace membership canonical, excludes the view from ephemeral clone GC, and surfaces it as a distinct `· view` row.
- **Current Pi zmux extension overhaul** `agents` - `pi-extension/` now targets
  current Pi (`0.80.x`), loads as the settings-managed local package, exposes
  refreshed typed tools for runtimes, panes, tab input, zmux config reload,
  soft Pi `/reload` via `zmux_pi_reload`, and hard respawn fallback, and keeps bash guardrails aligned with the shared zmux
  classifier corpus. `./dev.sh zmux` now reports disabled Pi settings and
  removes the retired global extension symlink instead of silently relinking it.
- **Parent-scoped parked pane affordances** `tabs` `panes` - `prefix+h` hides a
  focused joined pane under its parent, `prefix+H` prompts for the visible
  parent-local index/name to rejoin it, and the logical tabs row renders joined
  and hidden children with one compact badge grammar (`󰏤 name`, `󰏤[1] name~`).
  Full tabs stay top-level and are no longer hideable; hidden panes can still be
  promoted to full tabs explicitly.
- **`zmux tab split` joins a new pane in one keystroke** `tabs` - a single
  `prefix` binding creates a managed tab in the current cwd and joins it beside
  the focused pane in one motion, replacing the create-then-join two-step. The
  host is snapshotted before the detached `NewWindow` so creating the tab can't
  steal focus from the pane it joins.
- **Right-click pane/status-row menus** `panes` - right-clicking a joined pane
  or managed logical tab-row cell opens a native tmux `display-menu` scoped to
  the clicked pane via `{mouse}` / `#{pane_id}` with join-back, promote-to-full,
  hide-pane, and kill actions where they apply. (Header drag-swap stays deferred
  — tmux 3.4 can't separate a header drag from native border-resize; see ROADMAP
  → Later.)
- **`pane list --joined` agent discovery surface** `agents` - lists the
  session's joined logical panes (tab, host, anchor, caller) so agents and the
  peer/worker skill doctrine can reuse an already-active joined pane for
  long-running visible work instead of minting a fresh `run -n` tab.

### Fixed

- **Pi reload scheduling no longer races the active turn** `agents` `pi` - `zmux_pi_reload` now waits longer by default, retries when Pi prints the active-response warning, and suppresses wait-only continuation prompts so a successful reload does not trigger a spurious assistant reply.
- **Dashboard workspace/session lifecycle errors stay visible** `dashboard` `workspace` - Workspaces-tab create, rename, move, and kill flows now route validation, tmux, and store failures through visible status errors instead of silently refreshing. Dashboard-created sessions share the same managed-session path as CLI-created sessions, and pinned view rows are blocked from root rename/move mutations.
- **Single-pane windows use the pane-header label surface** `panes` - the
  `<index> <name> <detail>` pane-border header now renders for lone panes too,
  so pane titles and tab labels no longer jump between status-right and the pane
  header depending on whether a tab is split.
- **`snapshot --pane` honours the shared tab resolver** `panes` - a `--pane`
  arg is now resolved through the same logical-tab lookup as `watch`/`send`/`run`
  and the captured pane is named from its pinned tab label (command as fallback)
  instead of the raw command; raw `%pane` ids still bypass the resolver for
  evidence capture.
- **`CurrentHost` is pane-canonical** `tabs` - the join host is now the focused
  pane's logical tab rather than the focused window's full owner, so bare
  `zmux tab pane <tab>`, the command palette's join rows, and `zmux where` agree
  on the same target when focus sits on a joined rider pane.

## [0.10.0] - 2026-06-28

> Release tag: `v0.10.0` | Topics: `agents`, `palette`, `help`, `panes`, `tabs`, `session` | Compare: `v0.9.0...v0.10.0`

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
  first window's pane with `@zmux_tab_id`, so joining another tab _into_ it
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
  resolve clone-local. The create path drops an all-out-of-session _ambiguity_
  too: a roster name live in 2+ sibling sessions previously surfaced a fatal
  `ambiguous — use an id` and refused the local spawn; `run -n <name>` now
  creates in the current session, while a genuine in-session collision still
  errors.
- **Owned-attach fallback detaches on destroy without a global override**
  `session` - the sessionless/owned-attach reattach path needs its client to
  detach when the session is destroyed, but a global `detach-on-destroy off`
  preference made tmux refuse the attach outright. It is now set per-session, so
  the fallback works without touching the user's global setting.

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
