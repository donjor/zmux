# AGENTS.md - zmux

zmux is a Go CLI that wraps tmux with workspace/session management, recipes,
themes, a popup dashboard, logical tabs, and agent-friendly terminal controls.
Use [docs/architecture.md](docs/architecture.md) for the code map and
[docs/ROADMAP.md](docs/ROADMAP.md) for forward work.

## Commands

```sh
make build            # compile ./cmd/zmux
make test             # unit tests
make test-race        # race-detector suite used by the pre-push gate
make test-integration # integration tests; builds first
make gen-doctrine     # explicitly rewrite committed agent projections
make check-doctrine   # generator tests + non-mutating freshness check
make test-agent-surfaces # doctrine + Pi package + QA + single doctor
make lint             # go vet + golangci-lint + gofumpt check
make fmt              # gofumpt formatting
make vuln             # govulncheck
./dev.sh              # maintainer install for live zmux + skill/extension links
./dev.sh zzmux        # isolated edge binary/profile for testing
./qa lint             # validate QA walkthrough specs
./qa                  # human QA picker
```

After code changes, run the narrow test first, then `make test`. For material CLI
or tmux behavior, also run `make build` and the relevant `./qa` checklist.

## Layout

- `cmd/zmux/` - thin launcher; command implementation lives under `internal/cli`.
- `agent-doctrine/` - neutral terminal rules/scenarios and deterministic Claude/Pi generator.
- `cmd/qa/` - repo-local QA walkthrough runner invoked by `./qa`.
- `internal/` - Go implementation; side effects sit behind typed interfaces.
- `internal/tabs` + `internal/tabstate` - logical tabs, placement, and glyph state.
- `internal/tmux` - tmux boundary; use `tmux.Runner`, never direct tmux exec.
- `internal/tui/` - focused Bubble Tea UI packages; no flat root TUI package.
- `checklists/` - committed QA walkthrough TOML specs.
- `skills/zmux/` - Claude skill/hooks plus committed generated doctrine/testing projection.
- `pi-zmux/` - Pi extension, generated compact guidance/testing projection, and tests.
- `legacy/v0/` - archived bash prototype; do not extend it.

## Gotchas

- Do not run `zmux init` inside tmux; it intentionally refuses.
- Keep business logic off direct `os/exec`, `os.ReadFile`, and network calls.
  Reuse or introduce interfaces such as `tmux.Runner`, `config.FS`, or
  `qa.CmdRunner`.
- Keybindings come from `internal/keys`; run `make keys-gen` after changes and
  let `TestKeybindingsDocInSync` verify `docs/reference/keybindings.md`.
- Logical tabs are pane-canonical. Route name/address changes through
  `internal/cli/tab_target.go` so `run`, `watch`, `log`, `send`, `type`,
  `state`, and tab verbs keep working for full, pane, and hidden tabs.
- Workspace/session targets are local-label aware. Route CLI target changes
  through `internal/cli/session_target.go`; raw tmux names are debug/interop
  fallbacks, not the normal user-facing address.
- Session-group clones like `dev-b` collapse to their root in user-facing
  labels. Use `session.RootName` where raw `#S` could leak.
- `zzmux` is the isolated edge profile: its own socket, config, state dir, and
  generated tmux conf. Use it for live testing without touching the active
  `zmux` profile.
- Worktrunk's pre-merge gate lives in `.config/wt.toml` and mirrors CI/pre-push:
  `make lint` plus `make test-race`. Use `wt merge` so the gate runs.
- Long-running or interactive processes belong in zmux tabs, not the agent
  shell. Use `zmux run -n`, `zmux watch`, `zmux send`, and `zmux type`.
- Author shared agent outcomes/scenarios only under `agent-doctrine/`; never
  hand-edit generated skill/Pi projections. `./dev.sh zmux` checks freshness but
  does not regenerate, while `./dev.sh zzmux` remains binary-only.

## References

- [README.md](README.md) - user-facing install and command reference.
- [docs/architecture.md](docs/architecture.md) - package map and change guide.
- [docs/dev/qa.md](docs/dev/qa.md) - QA runner behavior.
- [docs/dev/agent-grounding.md](docs/dev/agent-grounding.md) - drive zzmux to ground/QA
  your own changes (spawn protocol, `--now` time injection, dev-QA split).
- [docs/domains/pi-zmux-extension.md](docs/domains/pi-zmux-extension.md) - Pi integration.
- [skills/zmux/SKILL.md](skills/zmux/SKILL.md) - terminal orchestration skill.
