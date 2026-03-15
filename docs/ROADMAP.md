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
- [ ] Go module setup (bubbletea + lipgloss + cobra)
- [ ] Standard project layout (`cmd/zmux`, `internal/*`)
- [ ] TOML config loading + validation
- [ ] Config defaults and `~/.zmux.toml` creation
- [ ] tmux command interface (wrapper for tmux CLI)
- [ ] tmux.conf generation from config

### Theming
- [ ] Theme file parser (iterm2-color-schemes format)
- [ ] Semantic palette mapping (ANSI → role structs)
- [ ] Theme resolution (user > bundled > iterm2)
- [ ] Bundled themes embedded in binary
- [ ] iterm2 theme download
- [ ] Theme picker TUI — swatches + metadata (dark/light, role labels)
- [ ] `zmux theme set/list/sync/pull` commands

### Status Bar
- [ ] 4 presets (default, minimal, powerline, blocks)
- [ ] Dynamic prefix-active state
- [ ] Active/inactive window styling
- [ ] Preset picker with previews

### Session Management
- [ ] Session picker (outside tmux) — fuzzy search, create, attach
- [ ] Tmp session model + auto-cleanup
- [ ] Declarative TOML templates
- [ ] Template discovery (user + bundled)
- [ ] New session from template flow

### Dashboard (inside tmux)
- [ ] tmux popup overlay activation via keybind
- [ ] Command palette mode — fuzzy action search, quick switch
- [ ] Full dashboard mode — session list, context, actions
- [ ] Toggle between palette and dashboard
- [ ] Theme browser (swatches view)
- [ ] Session management actions (rename, kill, move tab)

### Theme Sync
- [ ] SyncTarget interface
- [ ] Ghostty sync target
- [ ] Neovim sync target
- [ ] `zmux theme sync` / `zmux theme pull <target>`

### Polish
- [ ] TUI init wizard (replaces gum-based wizard)
- [ ] Shell completions (bash, zsh, fish via cobra)
- [ ] `zmux status` / `zmux help` / `zmux version`
- [ ] Clipboard detection + integration
- [ ] Error handling and user-friendly messages

## v1.x — near-term features

### Session Persistence
- [ ] Save session layout (windows, panes, working dirs)
- [ ] Restore session layout on tmux restart
- [ ] Layout-only — no command re-execution

### Workspaces
- [ ] Workspace concept — group related sessions
- [ ] Switch entire workspace at once
- [ ] Workspace-aware dashboard

### SSH Remote Support
- [ ] `zmux ssh <host>` — connect and auto-attach remote tmux session
- [ ] Remote session discovery — show remote sessions in local picker
- [ ] Remote session management — create, rename, kill from local zmux
- [ ] Transparent local/remote session switching

### Contextual Status Bar
- [ ] Status bar adapts based on session type
- [ ] Git branch display in dev sessions
- [ ] Custom indicators per workspace/session

### Custom Status Segments
- [ ] User-defined segments in TOML config
- [ ] Built-in segments: git-branch, clock, hostname, etc.
- [ ] Segment ordering (left/right)

### Theme Sync Enhancements
- [ ] File watcher mode (opt-in)
- [ ] Additional sync targets (alacritty, kitty, wezterm)
- [ ] Bidirectional sync (opt-in)

### Distribution
- [ ] GitHub releases with goreleaser
- [ ] Homebrew tap
- [ ] AUR package
- [ ] Nix flake
