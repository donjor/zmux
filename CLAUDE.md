# zmux — Development Guide

## Quick Reference

- **Language:** Go (bubbletea + lipgloss + cobra)
- **Build:** `make build`
- **Install (maintainer, with skill/extension symlinks):** `./dev.sh` (= `./dev.sh zmux`)
- **Install (plain, contributor-style):** `make install`
- **Edge build for testing:** `./dev.sh zzmux` (or `make install-zzmux`) — builds +
  installs an identical binary as `zzmux` (→ `~/.local/bin/zzmux`) so you can test
  changes without overwriting the live `zmux` you're running. `zzmux` is **fully
  isolated** via a binary-name profile (`config.Profile` from `argv[0]`): its own
  tmux socket (`-L zzmux`), config (`~/.zzmux.toml`), generated conf (`~/.zzmux.conf`),
  state dir (`~/.zzmux/`), and source discovery. So `zzmux apply`/`init`/workspace
  ops never touch the live `zmux` — run it freely, even nested. Bundled themes work;
  user-custom themes are profile-local (shared-library read-fallback is a follow-up).
- **Run tests:** `go test ./...` (or `make test`); `make test-race` for the race-detector run CI gates on
- **Run integration tests:** `go test -tags integration ./tests/...` (or `make test-integration`, which builds first)
- **Vulnerability scan:** `make vuln` (govulncheck; needs Go ≥1.25 — the toolchain auto-fetches)
- **Lint:** `make lint` (`go vet` + `golangci-lint`, incl. gofumpt format check); `make fmt` to auto-format
- **Pre-push gate (primary):** `make hooks` installs the versioned `scripts/hooks/pre-push` (via `core.hooksPath`) — runs `make lint` + `make test-race`, but **only on pushes that update `master`** (feature/wip pushes stay cheap, matching the git philosophy below). This repo is local-first; this hook is the real gate. Force on any push with `GATE_ALL=1 git push`; bypass once with `git push --no-verify`.
- **CI** (`.github/workflows/ci.yml`, secondary/dormant — `master` runs ahead of `origin`): three jobs — **lint**, **test** (vet + `go test -race` + integration), **vuln** (govulncheck)
- **v0 prototype:** `legacy/v0/bin/zmux0` (bash+gum; see `legacy/v0/README.md`)

## Development Workflow

After making changes, always:
1. `go build ./cmd/zmux/` — verify it compiles
2. `go test ./...` — run all tests
3. `./dev.sh` — build and install to test live

`./dev.sh` handles "text file busy" errors by removing the old binary first.

## Project Layout

```
cmd/zmux/                Thin launcher (main.go) — calls cli.Run(app.New(), version)
internal/cli/            Command tree (cobra commands, root, Run) — importable, testable
internal/config/         TOML config loading, defaults, FS interface
internal/theme/          Theme parsing, palette, resolver, go:embed bundled themes
internal/tmux/           Typed tmux CLI wrapper, mock, conf generation
internal/bar/            Status bar presets, generation, preview, two-line rendering
internal/session/        Session model, CRUD, TOML templates
internal/workspace/      Workspace state (TOML), session tracking, reconciliation
internal/sync/           Theme sync targets (ghostty, nvim)
internal/source/         External source discovery (sockets, catalog) + attach fallback
internal/overmind/       Overmind control client (Client interface)
internal/keys/           Keybinding registry — single source for conf.go, help, docs
internal/setup/          Shell-rc integration: plan/apply behind config.FS (markers, .bak)
internal/termtitle/      tmux terminal-title format contract + parser (leaf, no deps)
internal/terminal/       Resolves screenshot target for the current tmux client
internal/preview/        UI proto framework (Page, Controls, RenderContext)
internal/debug/          Opt-in debug logging (ZMUX_DEBUG=1)
internal/tui/            No flat package — dissolved into focused surfaces/leaves:
internal/tui/styles/        Shared lipgloss styles leaf
internal/tui/workspaceview/ Workspace-view data adapter (picker + dashboard)
internal/tui/picker/        Primary workspace+session picker
internal/tui/tabpicker/     Alt+` tab switcher
internal/tui/themepicker/   Standalone theme picker
internal/tui/wizard/        zmux init setup wizard
internal/tui/outline/       Tree-outline component
internal/tui/dashboard/  Tabbed dashboard app (DashboardApp, Tab interface)
internal/tui/dashboard/tabs/  Tab implementations: current (Session), sessions
                              (Workspaces), themes, bar, settings, help
                              Shared: scroll.go, mode_state.go, shared_*.go
internal/tui/palette/    Command palette (registry, providers, executor)
internal/tui/views/      Shared view components (SessionRow, WindowRow, TabBar, etc.)
```

## Key Patterns

- **All side-effects behind interfaces** — `tmux.Runner`, `config.FS`, etc.
- **Explicit DI composition root** — `app.App` (in `internal/app`) holds injected deps. The thin `cmd/zmux/main.go` builds it via `app.New()` and hands it to `cli.Run(app, version)` (in `internal/cli`), which builds the tree via `NewRootCmd(app, version)`. Each cobra command is a `newXCmd(app *apppkg.App)` constructor capturing `app`; flag state is constructor-local. **No package-global `app`.**
- **Tests use `tmux.MockRunner`** — configurable mock for all tmux operations
- **Bundled assets via `go:embed`** — themes in `internal/theme/bundled/`, templates in `internal/session/templates/`
- **Keybindings live in `internal/keys`** — the registry is the single source of truth. `conf.go` references `keys.X.Key` (never hardcode a bind char), help surfaces render from it, and `docs/keybindings.md` is generated (`make keys-gen`; the `TestKeybindingsDocInSync` golden test enforces freshness). Component-local TUI keys (cursor nav, etc.) stay in their surface packages.

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
- Don't reach for a package-global `app` (there isn't one) — accept `app *apppkg.App` as a constructor param and capture it in the command's closures

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
