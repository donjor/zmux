# zmux v0 — legacy bash prototype

This directory preserves the original bash + [charmbracelet/gum](https://github.com/charmbracelet/gum) implementation of zmux. It is **unsupported** — kept for reference and for anyone still relying on it.

For the actively developed v1 (Go), use the top-level `Makefile` (`make install`).

## What's in here

| Path | Purpose |
|------|---------|
| `bin/zmux0` | Main bash entry point (session controller) |
| `bin/zmux0-apply-theme` | Applies the configured theme to tmux at runtime |
| `lib/*.sh` | Shared theme/init/status/sync/help helpers |
| `tmux/zmux.tmux.conf` | v0 tmux config (sourced from `~/.tmux.conf`) |
| `install.sh` | Symlinks `zmux0` and `zmux0-apply-theme` into `~/.local/bin/` |
| `themes/`, `templates/` | v0's own asset copies (`themes/bundled/`, `templates/*.sh`) |

`themes/` and `templates/` are **real directories owned by v0**. v1 (Go) is the single source of truth and embeds its own copies via `go:embed` (`internal/theme/bundled/`, `internal/session/templates/`), so v0 keeps independent copies here rather than sharing through symlinks.

## Install (v0)

```bash
./legacy/v0/install.sh   # links zmux0 to ~/.local/bin/zmux0
zmux0 init               # sets up ~/.zmux.conf, sources tmux/zmux.tmux.conf in ~/.tmux.conf
```

Requires [gum](https://github.com/charmbracelet/gum) (`go install github.com/charmbracelet/gum@latest` or via brew/pacman/etc.).

## Migrating from v0 to v1

If your `~/.tmux.conf` was set up by `zmux0 init`, it contains lines like:

```
source-file /path/to/zmux/tmux/zmux.tmux.conf
run-shell  "/path/to/zmux/bin/zmux0-apply-theme"
```

Those paths are now under `legacy/v0/`. Either update them to the new paths, or remove them and run `zmux init` (v1) to regenerate the config.

## Status

v0 is feature-frozen. Bug fixes only if something user-facing breaks; no new features. The v1 codebase (under `cmd/` and `internal/`) supersedes everything here.
