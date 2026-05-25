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
- [x] Reconcile auto-heals unmanaged sessions into same-named workspaces
- [x] Workspace name validation (no spaces, no reserved names)
- [x] Dashboard: merged Session tab showing current session windows + sibling sessions
- [ ] Dashboard: Workspaces tab full CRUD (create/rename/delete from dashboard)
- [ ] Workspace-scoped templates (multi-session templates)
- [ ] Fork command: `zmux fork <session>` (shape decided, implementation deferred)
- [ ] Compose: workspace members can have grouped sessions (multi-monitor)

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

> **Next up (queued, in order):** (1) Charm v2 stack upgrade → (2) `zzmux` edge
> binary → (3) world-class SSH / nested-zmux (see SSH Remote Support above).

### Architecture refactor — done (2026-05-24)

Full record in `docs/reafactor/` (plans `016`/`017`, `RUNDOWN-LIGHT-LOG.md`).

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

### Skills & docs
- [ ] Rewrite a clean, focused, claude/codex-valid zmux skill — the generic
      `skills/zmux/SKILL.md` got poisoned with pi-specific content
- [ ] Move pi-specific content out into the pi-extension
      (`docs/pi-zmux-extension.md`)
