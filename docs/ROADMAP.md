# zmux — roadmap

## v0 — bash + gum prototype (done)

- [x] Session picker with fuzzy search
- [x] Session templates (bash scripts)
- [x] Tmp session model with auto-cleanup
- [x] Dashboard inside tmux (gum-based)
- [x] Theme system (iterm2-color-schemes format)
- [x] Semantic palette mapping (ANSI → roles)
- [x] Theme picker with ANSI color swatches
- [x] 13 bundled curated themes
- [x] iterm2 theme download (~300 themes)
- [x] Theme resolution (user > bundled > iterm2)
- [x] Theme sync — pull from ghostty
- [x] Theme sync — pull from nvim
- [x] Status bar — 4 presets (default, minimal, powerline, blocks)
- [x] Opinionated tmux.conf
- [x] Keybinds (Ctrl+Space prefix, vim copy mode)
- [x] Clipboard integration (wl-copy, xclip, pbcopy)
- [x] Init wizard
- [x] Help system
- [x] `zmux bar` command with inline ANSI previews
- [x] `zmux status` and `zmux help` commands

## v1.0 — Go rewrite (feature parity + polish)

### Foundation
- [x] Go module setup (bubbletea + lipgloss + cobra)
- [x] Standard project layout (`cmd/zmux`, `internal/*`)
- [x] TOML config loading + validation
- [x] Config defaults and `~/.zmux.toml` creation
- [x] tmux command interface (wrapper for tmux CLI)
- [x] tmux.conf generation from config
- [x] Opt-in debug logging (ZMUX_DEBUG=1)

### Theming
- [x] Theme file parser (iterm2-color-schemes format)
- [x] Semantic palette mapping (ANSI → role structs)
- [x] Theme resolution (user > bundled > iterm2)
- [x] Bundled themes embedded in binary
- [x] iterm2 theme download
- [x] Theme picker TUI — swatches + metadata (dark/light, role labels)
- [x] `zmux theme set/list/sync/pull` commands

### Status Bar
- [x] 9 presets (default, minimal, powerline, blocks, rounded, hacker, zen, starship, rpowerline)
- [x] Dynamic rendering via `zmux bar-render` — git, lang, workspace, directory, process
- [x] Configurable segment toggles in `[bar.segments]`
- [x] Dynamic prefix-active state
- [x] Active/inactive window styling, two-tone catppuccin-style tabs
- [x] Preset picker with live preview carousel
- [x] Instant refresh on session/window switch via tmux hooks

### Session Management
- [x] Session picker (outside tmux) — fuzzy search, create, attach
- [x] Tmp session model + auto-cleanup
- [x] Declarative TOML templates
- [x] Template discovery (user + bundled)
- [x] New session from template flow
- [x] `zmux ls` — list sessions
- [x] `zmux tabs` — list tabs in a session
- [x] Attach modes: mirror (auto-grouped), hijack

### Dashboard (inside tmux)
- [x] tmux popup overlay activation via keybind (prefix+Space)
- [x] Tabbed dashboard: Session, Workspaces, Themes, Settings, Help
- [x] Session list with local + external source groups
- [x] Themes tab — theme picker with swatches, color editor, bar preset with live preview
- [x] Settings tab — config fields (prefix, sync, sessions)
- [x] Help tab — keybindings reference

### Command Palette
- [x] Command palette popup (prefix+p)
- [x] Registry + provider architecture
- [x] Fuzzy search across all actions
- [x] Post-action execution (close, open dashboard tab, error)

### Terminal Commands
- [x] `zmux run` — run command in named window, wait/detach/follow
- [x] `zmux watch` — capture output, follow, wait for pattern
- [x] `zmux send` — send keystrokes to window
- [x] `zmux type` — type text + Enter

### Source Discovery
- [x] Multi-socket scanning (tmux socket directory)
- [x] Process correlation (ps-based)
- [x] Overmind provider — detect processes, extract metadata
- [x] Live socket probing with timeout
- [x] Catalog model (local + external groups)

### Theme Sync
- [x] SyncTarget interface
- [x] Ghostty sync target
- [x] Neovim sync target
- [x] `zmux theme sync` / `zmux theme pull <target>`

### Polish
- [x] TUI init wizard (replaces gum-based wizard)
- [x] Shell completions (bash, zsh, fish via cobra)
- [x] `zmux status` / `zmux help` / `zmux version`
- [x] `zmux apply` — regenerate + apply config
- [x] Clipboard detection + integration
- [x] Error handling and user-friendly messages
- [x] Claude Code skill (auto-installed by dev.sh/install.sh)

## v1.x — near-term features

### Session Persistence
- [ ] Save session layout (windows, panes, working dirs)
- [ ] Restore session layout on tmux restart
- [ ] Layout-only — no command re-execution
- [ ] Smarter disconnect handling — survive client drops / network blips more
      gracefully than the current approach

### Workspaces
- [x] Auto-grouped sessions — attaching to an attached session creates an
      independent viewport (shared windows, separate focus)
- [x] Workspace concept — first-class objects (versioned TOML v2)
- [x] `zmux new <workspace> [session...]` — variadic workspace+sessions creation
- [x] `zmux open <workspace> [session]` — workspace-level access
- [x] `zmux attach` retained as hidden alias for `open`
- [x] `zmux <workspace>` shorthand — attach last-active session
- [x] `zmux <workspace> <session>` two-arg shorthand — attach specific session
- [x] `zmux ls` workspace-primary — workspaces by default, `ls <ws>` for sessions
- [x] `zmux kill` workspace-aware — workspace-first, confirms if live sessions
- [x] **Workspace-primary picker** — single flat list with inline session expansion
- [x] Picker: ghost tab completion, fuzzy match underline, ctrl+x delete, ctrl+h toggle empty
- [x] Picker: search grammar `<ws> <session>` for inline filtering
- [x] Workspace-aware dashboard — Session tab + Workspaces tab
- [x] Status bar shows workspace + session position (e.g. `myapp 2/4`)
- [x] Session navigation keybindings (Shift+Alt+1-9, prefix+w, prefix+[/])
- [x] `zmux tab move/kill`, `zmux session kill`, `zmux workspace kill`
- [x] Tab labels show a `[cwd]` suffix only when a duplicate tab name needs
      disambiguating — stable overlay format in `internal/tablabel` (`45e508c`,
      2026-06-03)
- [x] Reconcile auto-heals unmanaged sessions into same-named workspaces
- [x] Workspace name validation (no spaces, no reserved names)
- [x] Dashboard: merged Session tab showing current session windows + sibling sessions
- [ ] Dashboard: Workspaces tab full CRUD (create/rename/delete from dashboard)
- [ ] Workspace-scoped templates (multi-session templates)
- [ ] Fork command: `zmux fork <session>` (shape decided, implementation deferred)
- [ ] Compose: workspace members can have grouped sessions (multi-monitor)

### QoL & polish (intake 2026-05-28)

Promoted from `docs/.NOTES.md` (see intake there for the verbatim raw notes
behind each entry). Bugs lead — these are visibly broken edges, fix before
the SSH push.

Bugs:
- [x] `leader .` rename in top bar errors with `command -t option..` —
      root cause was the prefix-active hint hardcoding `.rename` while `.`
      was bound to label-tab; `prefixHints` now reads from `internal/keys`
      so `,rename` and `.label` match the actual binds (2026-05-28)
- [x] Top bar still shows session pill after that session is deleted —
      two-part fix: dashboard/picker/palette session-kill paths now funnel
      through `workspace.KillSession` which drops store membership when no
      live grouped clone remains; `session-created[2]`/`session-closed[2]`
      tmux hooks force a `refresh-client -S` after `bar-adjust` (2026-05-28)
- [x] Deleting the active session in a workspace from the dashboard kills
      zmux entirely — `CurrentTab.killSession` now switches the client to
      a sibling (and updates last-active) before tmux-killing the target;
      a single-session workspace blocks with a status flash directing the
      user to kill from outside tmux (2026-05-28)
- [x] Dashboard workspace & session rename feels fragile — root cause was
      rename mutations swallowing `wsStore.Rename*` errors (and the
      mutation cmds further swallowing their returned error), so name
      conflicts / validation failures produced silent no-ops; both layers
      now propagate, surfacing as a `SetStatusIntent` error flash
      (2026-05-28)

Picker & workspace nav:
- [x] Create a new session under an existing workspace from the picker — the
      `<ws> <session>` grammar already worked; plan 020 added the ghost-prompt
      hint, footer line, and help-tab callout to surface it (2026-05-28)
- [x] Keybind to switch workspaces without the dashboard — `M-w` no-prefix
      popup via `--workspace-picker`, parallels `M-`backtick`` (2026-05-28)

Status bar:
- [x] Sibling-session pill should indicate which other sessions in the
      workspace are attached *elsewhere* — `AttachState` enum
      (`Unknown|Local|Remote`) feeds `CompactDots`; attached siblings render
      `◉` to distinguish from inactive `○` (2026-05-28)

Dashboard unification:
- [x] Workspaces tab — `/` fuzzy search over workspaces, sessions, and external
      groups (live-filter, commit-on-enter, two-step esc clear); mutation under
      an active filter drops the filter and jumps to the new row (plan 021,
      2026-05-29)
- [x] `internal/tui/workspacelist/` seam adopted in the M-w switcher
      (`wspicker` collapsed 214→123 LOC onto it); the dashboard Workspaces tab
      keeps its richer outline impl (external rows, move, kill-confirm) — folding
      that in stays a possible follow-up (plan 021, 2026-05-29)
- [x] Dashboard modal-esc routing — `Tab.CapturesEscape()` lets a tab keep Esc
      while a modal/filter is open instead of the dashboard quitting out from
      under it; fixed a latent bug where every tab's `esc:cancel` was dead in
      production, plus the Themes tab's committed-filter "esc to clear" hint
      (which previously quit the dashboard) (plan 021, 2026-05-29)
- [~] Session (current) tab — fzf + numbered index still deferred; scope label
      ("sessions in <workspace>") added to the current-tab banner so the
      list's scope is unambiguous (2026-05-28)

Popups:
- [x] Dashboard + scratch shell — rounded outline via `popup-border-lines
      rounded`; dim-terminal-behind deferred (theme-fragility risk too high
      to commit blind) (2026-05-28)
- [x] Scratch shell — fork/extract to a tab via `zmux scratch extract`
      (captures cwd, creates a new tab in the parent session, closes the
      popup) (2026-05-28)

### Bar layout & density (intake 2026-06-03)

Promoted from `docs/.NOTES.md §UI` as a single research/design thread. The
top bar grows with content and crowds out tabs; the underlying tension is
how much chrome to show and at what scale.

- [x] Header overflow — tab survival via two-line row-ownership de-dup (the
      dominant lever: freed ~40–68 cells of bottom-left width), plus the
      already-present native `<`/`>` overflow markers + `list=focus`. Side caps
      measured non-binding, deliberately left alone. Regression net:
      `TestBarWidthBudget`. (2026-06-03)
- [x] Always-2-line bar — two-line is now unconditional (reconcile-to-layout,
      no per-session count collapse). The 1-row `single` layout was removed
      entirely (config normalizes legacy `single` → two-line); `split` is the
      only alternate. (2026-06-03)
- [x] Independent bar/header font scale — *investigated: not possible.* A cell
      grid has no per-region font; sizing is wholly the emulator's job. The
      achievable proxy is **density**, whose levers zmux already owns; future
      follow-up is a bundled *compact mode*. Finding: `docs/bar-density.md`.
      (2026-06-03)

### QoL & polish (intake 2026-06-04)

- [ ] Picker (main entry, outside tmux): `ctrl+x` delete should keep the cursor
      position — today the cursor moves; confirmed still reproducing there
      (inside-tmux surfaces unaffected)

### Agent-driven terminals — real buddies (intake 2026-06-04)

Promoted from `docs/.NOTES.md §Bulletproof buddy skill replacement` as a single
research/design thread. The idea: drop the pi-sdk adapter layer and run *real*
agent CLIs in zmux tabs — official CLIs always work, and the discussion is
visible right where it happens. zmux-scoped: the buddy-skill rewrite itself
stays out of this repo's roadmap until the spike proves out.

Design ratified 2026-06-04 (full record:
`.dump/discussions/2026-06-04_buddy-via-zmux-design.md`): drive CLIs *as a
human would* — no flags, no hook injection; zmux grows one dumb primitive
(`watch --idle`: quiet-screen capture) and the driving LLM is the adapter,
judging done/question/false-idle from the capture. Screen drives turn-taking;
the CLI's default session JSONL is the passive quote path. No orchestrator
above — the driving session is the orchestrator (resolves the intake's open
question). Long-lived `buddy` tab; multi-buddy explicitly out of scope.

- [ ] Spike — `zmux watch <tab> --idle <s> [--timeout]` (block until pane
      quiet → exit → print capture) + multiline bracketed-paste and
      no-focus-steal verification against a live codex/pi buddy tab; MVP in a
      normal tab (pane-mode unification is in-thread but post-MVP)

### SSH Remote Support
- [ ] `zmux ssh <host>` — connect and auto-attach remote tmux session
- [ ] Remote session discovery — show remote sessions in local picker
- [ ] Remote session management — create, rename, kill from local zmux
- [ ] Transparent local/remote session switching
- [ ] World-class nested-zmux — connecting into a host that *itself* runs zmux
      (prefix/keytable, bar, and theme coordination across the outer/inner layer)

### Contextual Status Bar
- [x] Git branch display with dirty/ahead/behind indicators
- [x] Per-window git status via pane working directory
- [x] Workspace display in status bar
- [x] Language version detection (Go, Node, Rust, Python)
- [x] Active process and directory display
- [ ] Status bar adapts based on session type
- [ ] Custom indicators per workspace/session

### Custom Status Segments
- [x] Built-in segments: git, workspace, clock, lang, directory, process, group
- [x] Segment toggles in `[bar.segments]` config
- [x] Settings tab toggles segments with live preview
- [ ] User-defined custom segments in TOML config
- [ ] Segment ordering (left/right)

### Theme Sync Enhancements
- [ ] File watcher mode (opt-in)
- [ ] Additional sync targets (alacritty, kitty, wezterm)
- [ ] Bidirectional sync (opt-in)

### Picker UX Configuration
- [ ] Optional vim-style navigation mode (j/k + / to search, i/a for insert)
- [ ] Configurable search behavior: always-on (current) vs explicit / trigger
- [ ] Configurable via `[picker]` section in .zmux.toml

### Distribution
- [ ] GitHub releases with goreleaser
- [ ] Homebrew tap
- [ ] AUR package
- [ ] Nix flake

## Engineering & internals

> **Next up (sequence ratified 2026-05-29, "C → A"):**
> (1) ~~plan 021 — finish QoL deferrals~~ **DONE (2026-05-29)**: `wspicker`
> collapsed onto the `internal/tui/workspacelist` seam, `/` search added to the
> dashboard Workspaces tab, dashboard modal-esc routing fixed. The dashboard
> Workspaces tab was *not* lifted onto the seam — its outline impl carries
> external rows / move / kill-confirm beyond the component's surface; folding it
> in stays an optional follow-up. → (1b) ~~plan 022 — zzmux profile-isolation
> hardening + sibling labels~~ **DONE (2026-05-29)**: `popupBind` helper kills
> the hardcoded-`zmux` else-branches, `config.SelfBin(profile)` replaces every
> `os.Executable()→"zmux"` fallback, the scratch-popup title + shell-rc
> auto-start now follow the active profile binary, and sibling tmux servers get
> friendly labels (socket `default`→`zmux`, `zzmux`→`zzmux (edge)`). → (2)
> **world-class SSH / nested-zmux** (see SSH Remote Support above), scoped to a
> ruthless MVP (`zmux ssh <host>` + auto-attach, remote sessions in the local
> picker, attach/switch local↔remote; defer remote CRUD / theme-sync / full
> nested coordination to a follow-up slice) — now the head of the queue. The
> v1.x QoL & polish pass (plan 020, 2026-05-28), Charm v2 (2026-05-25) and
> `zzmux` isolation (2026-05-26) are done — sub-sections retained below for
> record.

### Architecture refactor — done (2026-05-24)

Full record in `docs/refactor/` (plans `016`/`017`, `RUNDOWN-LIGHT-LOG.md`).

- [x] Omega ideal restructure — package-boundary split + DI repeal
      (`internal/app` + `NewRootCmd`), `internal/keys` + `internal/setup`
      registries, `cmd/zmux` → `internal/cli` thin-main, gofumpt +
      golangci-lint v2 tooling. Merged `26dc7a9` / `83e13a9` / `f8cd74a`.
- [x] followup-03 — B-purity seams (2026-05-25): `overmind.Client` injected via
      `App.Overmind` (7 call sites rewired, package wrappers deleted); terminal
      adapter/process injected as cmd params (package globals gone); cli test
      apps use in-memory FS.
- [x] followup-04 — C3 source-discovery prober (2026-05-25): `source.prober`
      seam (`systemProber`/`fakeProber`); `Discover()` → `discoverWith(prober)`
      with orchestration tests.
- [x] `docs/architecture.md` refresh (2026-05-25): stale file-size table (no prod
      file >500 now), seams table (added `source.prober`, noted overmind/terminal
      injection), picker tree nit.

### Charm v2 stack upgrade — done (2026-05-25)

Plan + evidence in `.dump/plans/018_2026-05-25_charm-v2-upgrade/`.

- [x] bubbletea v1.3.10 → v2.0.6 (`charm.land/bubbletea/v2`)
- [x] lipgloss v1.1.0 → v2.0.3 (`charm.land/lipgloss/v2`)
- [x] bubbles v1.0.0 → v2.1.0 (`charm.land/bubbles/v2`) — coupled, migrated together
- [x] Adopt `charmbracelet/log` for structured logging (slog backend in
      `internal/debug`; pins `x/cellbuf v0.0.15` — see `go.mod` note)
- [ ] Follow-up: textinput light-theme styling via `textinput.DefaultStyles(isDark)`
      (all bundled themes are dark today, so v2 dark defaults = parity; this is an
      enhancement, not a regression)

### Dev / dogfooding
- [x] `zzmux` edge binary — separate build + install (`./dev.sh zzmux` /
      `make install-zzmux`) so dev work doesn't clobber the live `zmux` running
      active Claude sessions
- [x] `zzmux` full isolation (2026-05-26) — binary-name profile (`config.Profile`
      keyed off `argv[0]`): own tmux socket (`-L zzmux`) + config (`~/.zzmux.toml`)
      + conf (`~/.zzmux.conf`) + state (`~/.zzmux/`) + source discovery, so
      `zzmux apply`/`init`/workspace ops never touch live `zmux`. Plan + verify:
      `.dump/plans/019_2026-05-26_zzmux-isolation/`. Deferred follow-ups (all
      read-only / benign): (1) resolver read-fallback to the shared `~/.zmux/themes`
      + `~/.zmux/templates` libraries — zzmux gets bundled themes + its own profile
      dirs today, not user-custom shared ones (incl. `DefaultConfig()` templates
      path); (2) `zzmux` symlinked to the `zmux` binary is unsupported — the
      generated conf embeds `os.Executable()`, so install by copy (`./dev.sh zzmux`
      / `make install-zzmux` already copy).
- [x] `zzmux` profile-isolation hardening + sibling labels (2026-05-29) — plan
      `.dump/plans/022_2026-05-29_zzmux-profile-isolation-dry/`. Root cause of the
      "zzmux dashboard shows zmux workspaces" report was a *stale live server*
      whose `prefix+Space` bind pointed at the live `zmux` binary (predated the
      profile-aware bind), not a divergent loader. Hardened the class:
      `internal/tmux/conf.go` `popupBind()` helper routes all six self-invoking
      popup binds and drops the hardcoded-`zmux` `else` branches; `config.SelfBin`
      replaces every `os.Executable()→"zmux"` fallback (apply, popup re-launch ×2,
      wizard); scratch-popup title + shell-rc auto-start follow the active profile
      binary. Sibling servers now read as `zmux` / `zzmux (edge)` in the
      cross-server picker/dashboard (`source.externalLabel`, raw socket kept as
      ID). Plus a P3 DRY cleanup: one `loadWorkspaceView(app, opts)` replaces three
      near-identical loader closures. Deferred follow-ups (buddy-surfaced, all
      non-isolation or scope-adjacent): (1) `runApply` still swallows the
      `SourceFile` error — consistent with apply's deliberate best-effort design
      and not causal to the original staleness, but wrapping it would improve
      diagnosability of conf-source failures; (2) `internal/bar/apply.go` +
      `cli/bar_adjust.go` still call `os.Executable()` directly (different
      fallback semantics — `""`/skip, not `profile.Name`; `bar.Apply` takes no
      profile, so DRY-ing needs a signature change); (3) picker ghost-prompt
      strings (`internal/tui/picker/picker_view_help.go`) still say literal
      `zmux new …` — display-only (the action dispatches in-process,
      profile-correct) but a ghost/CLI divergence under `zzmux`; folding the
      profile name into the picker package is a separate slice.

### Skills & docs
- [x] Rewrite a clean, focused, claude/codex-valid zmux skill — the generic
      `skills/zmux/SKILL.md` got poisoned with pi-specific content (shipped
      `53e3191`)
- [x] Move pi-specific content out into the pi-extension
      (`docs/pi-zmux-extension.md`) (shipped `53e3191`)

### Agent terminal-hygiene guard — done (2026-06-03)

One source of truth — `testdata/zmux-guard-corpus.jsonl` (86 rows) — drives three
classifiers that each assert row-for-row against it, so the redirect ruleset can't drift
across surfaces. Squash-merged `f135b2e` (from `feat/agent-guard-convergence`); the
`kind` (shared shell-surface category) / `decision` (Claude/shell policy) contract lives
in `testdata/zmux-guard-corpus.README.md`.

- [x] Shared corpus gate — `kind` asserted by all three; `decision` asserted by the Go
      classifier + Claude hook, derived by pi (its `direct_zmux` nudge + socket→safe fold
      + interactive→block are documented adapters, not drift)
- [x] Go classifier `internal/guard` + hidden `zmux guard` CLI (`internal/cli/guard.go`,
      exit 2 = block)
- [x] Claude `PreToolUse:Bash` hook `skills/zmux/hooks/zmux-guard.mjs` (live via repo
      symlink `~/.claude/hooks/zmux-guard.mjs`) — redirects raw `tmux` + background `&` to
      zmux equivalents; bypass with `ZMUX_ALLOW=1` or `# zmux: allow`
- [x] Proactive `zmux-context.mjs` SessionStart hook (inside-tmux only, fails open) —
      surfaces existing tabs so agents reuse them instead of spawning shells
- [x] pi `classify.ts` reuses the same corpus in `pi-extension/test/run.mjs`
- [ ] Follow-up: scan command-substitution `$(tmux …)`, `sh -c "tmux …"`, `xargs tmux`,
      and here-doc bodies — currently **not** scanned (corpus is a drift gate, not
      prevention)
