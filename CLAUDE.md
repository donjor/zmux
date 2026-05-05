# zmux — Development Guide

## Quick Reference

- **Language:** Go (bubbletea + lipgloss + cobra)
- **Build + install:** `./dev.sh` (builds and copies to ~/.local/bin/zmux)
- **Run tests:** `go test ./...`
- **Run integration tests:** `go test -tags integration ./tests/...`
- **Lint:** `go vet ./...`
- **v0 prototype:** `bin/zmux0` (bash+gum, accessible via `zmux0` alias)

## Development Workflow

After making changes, always:
1. `go build ./cmd/zmux/` — verify it compiles
2. `go test ./...` — run all tests
3. `./dev.sh` — build and install to test live

`./dev.sh` handles "text file busy" errors by removing the old binary first.

## Project Layout

```
cmd/zmux/                CLI entry point, cobra commands, app wiring
internal/config/         TOML config loading, defaults, FS interface
internal/theme/          Theme parsing, palette, resolver, go:embed bundled themes
internal/tmux/           Typed tmux CLI wrapper, mock, conf generation
internal/bar/            Status bar presets, generation, preview, two-line rendering
internal/session/        Session model, CRUD, TOML templates
internal/workspace/      Workspace state (TOML), session tracking, reconciliation
internal/sync/           Theme sync targets (ghostty, nvim)
internal/source/         External source discovery (sockets, overmind, catalog)
internal/preview/        UI proto framework (Page, Controls, RenderContext)
internal/debug/          Opt-in debug logging (ZMUX_DEBUG=1)
internal/tui/            Workspace picker, shared styles, outline tree model
internal/tui/dashboard/  Tabbed dashboard app (DashboardApp, Tab interface)
internal/tui/dashboard/tabs/  Tab implementations: current (Session), sessions
                              (Workspaces), themes, bar, settings, help
                              Shared: scroll.go, mode_state.go, shared_*.go
internal/tui/palette/    Command palette (registry, providers, executor)
internal/tui/views/      Shared view components (SessionRow, WindowRow, TabBar, etc.)
```

## Key Patterns

- **All side-effects behind interfaces** — `tmux.Runner`, `config.FS`, etc.
- **Global `app` variable** in `cmd/zmux/root.go` — don't create `NewApp()` in commands
- **Tests use `tmux.MockRunner`** — configurable mock for all tmux operations
- **Bundled assets via `go:embed`** — themes in `internal/theme/bundled/`, templates in `internal/session/templates/`

## Session Groups (multi-viewport clones)

When attaching to a session that's already attached, zmux creates a **grouped session** (`dev-b`, `dev-c`, ...) sharing the same windows but with an independent current-window pointer. Cleaned up on detach.

- **`session.RootName(name)`** strips `-[b-z]$` suffix → `"dev-b"` → `"dev"`
- **`session.ListSessions()`** collapses clones into the root, summing `AttachedClients`
- **`workspace.Store.WorkspaceFor()`** resolves clones via RootName internally
- **UI surfaces** must resolve `#S` (raw tmux session name) to root before display — otherwise clone names like `dev-b` leak into the bar pill, dashboard labels, etc.
- **`bar_render.go`** uses group-gated resolution: only strips when `groupID != ""` to avoid false positives for sessions genuinely named `foo-b`

## Don't

- Don't run `zmux init` inside tmux — it refuses and tells you to exit first
- Don't add unused dependencies in early phases
- Don't call `os/exec` or `os.ReadFile` directly in business logic — use the interfaces
- Don't create `NewApp()` in cobra commands — use the global `app`

## Terminal Management (when working in a zmux session)

Use zmux commands to manage processes and shared terminals:

```bash
zmux ls                          # list sessions
zmux tabs                          # list tabs in current session
zmux run 'cmd' -n name           # run in named window (creates or reuses)
zmux watch name                  # read window output
zmux send name C-c               # send keys to window
zmux type name 'cmd'             # type command + Enter
```

**Rules:**
- Never run dev servers, builds, or long processes in your shell — use `zmux run -n server`
- Never use `&` for background tasks — use `zmux run`
- Read output with `zmux watch`, don't re-run commands to check status
- For sudo/interactive commands, use `zmux type admin 'sudo ...'` and tell the user

See `skills/zmux/SKILL.md` for full integration guide.
