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
- [x] 4 presets (default, minimal, powerline, blocks)
- [x] Dynamic prefix-active state
- [x] Active/inactive window styling
- [x] Preset picker with previews

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
- [x] Tabbed dashboard: This Session, Sessions, Settings, Help
- [x] Session list with local + external source groups
- [x] Settings tab — theme picker with swatches, bar preset selection
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

### Workspaces
- [x] Auto-grouped sessions — attaching to an attached session creates an
      independent viewport (shared windows, separate focus)
- [ ] Workspace concept — tag sessions to a named project/workspace
- [ ] `zmux new -w <workspace> [name]` — create session in a workspace
- [ ] Picker groups sessions by workspace
- [ ] Switch entire workspace at once
- [ ] Workspace-aware dashboard
- [ ] Compose: workspace members can have grouped sessions (multi-monitor)

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

### Picker UX Configuration
- [ ] Optional vim-style navigation mode (j/k + / to search, i/a for insert)
- [ ] Configurable search behavior: always-on (current) vs explicit / trigger
- [ ] Configurable via `[picker]` section in .zmux.toml

### Distribution
- [ ] GitHub releases with goreleaser
- [ ] Homebrew tap
- [ ] AUR package
- [ ] Nix flake
