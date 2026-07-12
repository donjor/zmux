# Setup

Install, update, and profile setup for zmux.

## Overview

zmux installs one Go binary and generates tmux config for a named profile. The
normal profile is `zmux`; maintainers can use `zzmux` as an isolated edge
profile with separate binary, socket, config, and state.

Read this with the root [README](../README.md) for user-facing usage and
[dev/README.md](dev/README.md) for maintainer verification.

## Prerequisites

- tmux 3.2 or newer.
- Go toolchain for building from source.
- `~/.local/bin` on `PATH` for the default install location.
- A shell you are willing to update if you enable lifecycle integration.

## Run

Choose one of the install/update paths below, then run `zmux init` for first-time
profile setup or `zmux apply` / `zmux refresh` after config changes.

## Install paths

### End-user install

```bash
git clone https://github.com/donjor/zmux.git
cd zmux
./install.sh
```

`./install.sh`:

- checks Go and tmux dependencies;
- builds the Go binary;
- installs `zmux` to `~/.local/bin/zmux`;
- offers shell lifecycle integration;
- offers to run `zmux init`.

It writes the live profile config to `~/.zmux.toml` and profile state under
`~/.zmux/`.

### Manual install

```bash
make build
make install
zmux init
```

`make build` compiles `./cmd/zmux`. `make install` copies the built binary into
`~/.local/bin/zmux`. `zmux init` is the interactive first-run wizard and refuses
inside tmux so it can safely generate outer tmux config.

### Update

```bash
git pull
make install
zmux apply
```

`make install` rebuilds and copies the binary. `zmux apply` regenerates and
applies the generated tmux config for the active profile. Use `zmux refresh` if
the current client needs truecolor/terminal-feature refresh after config changes.

## Shell integration

Shell lifecycle integration lets normal foreground commands publish
running/done/failed state into tmux pane options. That gives `zmux run`, typed
commands, and agent tabs the same lifecycle glyph substrate.

Commands:

```bash
zmux setup shell   # install/update managed rc block
zmux doctor        # check rc block and current shell freshness
```

After installing or updating shell integration, open a fresh shell/tab. Existing
shell processes keep already-loaded functions.

## Edge profile

Maintainers should use the edge profile before touching the live profile:

```bash
./dev.sh zzmux
zzmux init
zzmux doctor
```

`./dev.sh zzmux` installs `~/.local/bin/zzmux` only. It does not mutate live
shell startup files or shared agent integration state. The edge profile writes
its own config/state and tmux socket, so QA can run without corrupting active
`zmux` sessions.

Use `./dev.sh zmux` only when you intentionally want to refresh the live binary
and shared agent integrations. It links the owned sources into the configured
`ZMUX_SKILLS_ROOT`, installs and verifies only the `zmux` skill for Claude, Codex,
and Antigravity, then stages and verifies only the `pi-zmux` package for Pi. It
runs `agent-doctrine` freshness validation before build/link/sync mutation and
refuses stale generated projections; regenerate explicitly with
`make gen-doctrine` first.

## Output and failure behavior

Setup commands print the files they plan to write or refresh. Dependency,
permission, unsupported-context, and stale-shell problems fail with normal exit
status and a message that points at the next check (`zmux doctor`, a fresh
shell, or a missing dependency). Most commands support `--help`; use it before
adding or changing command examples in docs.

## When this doc drifts

Update this file when install scripts, profile paths, shell integration behavior,
`zzmux` isolation, or setup failure guidance changes.
