# Contributing to zmux

Thanks for considering a contribution. zmux is a Go-based tmux orchestration TUI. This guide gets you from a fresh clone to a green build to a meaningful PR.

For *what* zmux is, see [README.md](README.md). For *how it's built*, read [docs/architecture.md](docs/architecture.md) — it's the contributor map.

## Prerequisites

- **Go 1.25+** (pinned in `go.mod`; the Charm v2 stack requires it)
- **tmux 3.3+** (zmux speaks current tmux; older versions may work but aren't tested)
- A POSIX shell environment (Linux or macOS). Windows is not supported.
- Optional: `gum` only if you want to run the legacy v0 (`legacy/v0/`).

Check your versions:

```bash
go version
tmux -V
```

## Get the code

```bash
git clone https://github.com/donjor/zmux.git
cd zmux
make build
make hooks   # one-time: install the pre-push quality gate (see below)
```

`make build` invokes `go build` with version-info `ldflags` and drops a `zmux` binary at the repo root.

`make hooks` points `core.hooksPath` at the versioned `scripts/hooks/`; the
`pre-push` hook runs `make lint` + `make test-race` — but **only on pushes that
update `master`**. Feature/wip branch pushes stay cheap (wip commits may be
unverified — iterate freely). This is the repo's primary gate; GitHub Actions is
secondary. Force the gate on any push with `GATE_ALL=1 git push`; bypass it once
with `git push --no-verify`.

## Run the tests

```bash
make test               # unit tests (fast)
make test-race          # unit tests with the race detector (mirrors CI)
make test-integration   # integration tests — exercise the built ./zmux CLI (no tmux needed)
make vuln               # govulncheck vulnerability scan
make lint               # go vet + golangci-lint (incl. gofumpt format check)
make fmt                # auto-format with gofumpt
```

CI runs three jobs on every push and PR: **lint** (`golangci-lint`, which also
enforces gofumpt formatting), **test** (`go vet`, then `go test -race ./...` and
the build-tagged integration tests), and **vuln** (`govulncheck`). Keep them
green before opening a PR. `golangci-lint` is pinned in CI; install it locally
with `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`
(and `mvdan.cc/gofumpt@latest` if you want the standalone formatter).

## Install locally

```bash
make install            # builds + copies the binary to ~/.local/bin/zmux
```

`make install` does **not** touch your `~/.claude/` or `~/.pi/` directories — it's safe to run as a contributor.

> **Don't run `./dev.sh` unless you know what it does.** It's a maintainer convenience that also symlinks `skills/zmux/` and `pi-extension/` into your home directory's agent configuration. Use `make install` for normal development.

## The dev loop

The typical change cycle:

1. Edit code under `internal/cli/` (commands) or `internal/` (logic).
2. `make build` — compile check.
3. `make test` — unit tests.
4. `make install` (or `make build && ./zmux <cmd>`) — exercise the change live.
5. For UI changes, you may need to `prefix+r` inside an existing tmux session to reload the generated config.

Common gotchas:

- **`zmux init` refuses inside tmux.** This is intentional; exit your tmux session first.
- **Generated config lives at `~/.zmux/tmux.conf`** and is overwritten by `zmux apply`. Don't hand-edit it.
- **All tmux side effects must go through `internal/tmux.Runner`.** No direct `os/exec` calls in business logic. See [docs/architecture.md → Key interfaces](docs/architecture.md#key-interfaces-the-seams).

## Project layout

See [docs/architecture.md](docs/architecture.md) for the full map. A quick orientation:

```
cmd/zmux/          # thin launcher (main.go) — calls internal/cli.Run
internal/          # all business logic (Go internal/ visibility)
                   #   cli/                — cobra command tree (importable, testable)
                   #   session/templates/  — embedded session templates (go:embed source)
                   #   theme/bundled/      — embedded themes (go:embed source)
docs/              # this guide, architecture, vision, keybindings
themes/iterm2/     # downloaded theme cache (gitignored)
tests/             # integration tests (build tag: integration)
legacy/v0/         # archived bash prototype — unsupported (owns its own asset copies)
```

## Where to make common changes

The fastest path is in [docs/architecture.md → Where to make common changes](docs/architecture.md#where-to-make-common-changes). A few highlights:

- **New CLI subcommand** → `internal/cli/<name>.go`, register in `internal/cli/root.go`.
- **New keybinding** → add the `Binding` to `internal/keys`; `conf.go`, the help surfaces, and `docs/keybindings.md` all derive from it. Run `make keys-gen` to regenerate the doc (`TestKeybindingsDocInSync` enforces freshness).
- **New dashboard tab** → implement `dashboard.Tab` under `internal/tui/dashboard/tabs/`.
- **New theme** → drop a file into `internal/theme/bundled/`; it's `go:embed`'d on next build.

## Style and review

- **No new dependencies without discussion.** Open an issue first.
- **Add tests for new behavior.** Unit tests prefer `tmux.MockRunner` over real tmux.
- **Keep files focused.** A file over ~500 lines without a clear "this is one cohesive thing" justification will get pushback.
- **Don't add comments for what the code already says.** Comments explain *why*, not *what*. See [CLAUDE.md](CLAUDE.md) for the full style stance.
- **Code must be gofumpt-clean.** `make lint` (and CI) enforce this via
  `golangci-lint`; run `make fmt` to auto-format.

## Commit and PR

- Keep commit messages tight: 1-2 lines, no `Co-Authored-By` lines.
- Reference the issue/PR if there is one.
- A good PR is small enough to review in 15 minutes. If it's larger, consider splitting.
- Update `docs/` and `CLAUDE.md` if the code change makes them stale.

## License

The license has not been finalized yet. If you submit a PR, you grant the maintainer the right to relicense your contribution under the eventual license of this project. Open an issue if you need clarity before contributing significantly.

## Filing issues

When reporting a bug, include:

- zmux version (`zmux version`)
- tmux version (`tmux -V`)
- OS + terminal emulator
- Steps to reproduce
- What you expected vs what happened
- Output of `ZMUX_DEBUG=1 zmux <command>` if relevant

## Questions

If something in this guide or [docs/architecture.md](docs/architecture.md) doesn't match reality, that's a docs bug — please file it (or fix it).
