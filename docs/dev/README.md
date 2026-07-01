# Development

How to change zmux safely. Read this after root `AGENTS.md` and
[`docs/README.md`](../README.md), then jump to the domain doc that owns the path.

## Commands

```sh
make build            # compile ./cmd/zmux
make test             # unit tests
make test-race        # race detector suite; mirrors the Worktrunk merge gate
make test-integration # integration tests; builds first
make lint             # go vet + golangci-lint + gofumpt check
make fmt              # gofumpt formatting
make vuln             # govulncheck
./qa lint             # validate QA walkthrough specs
./qa                  # human QA picker
```

Run the narrow test first, then `make test`. For material CLI, tmux, keybinding,
or TUI behavior, also run `make build` and the relevant `./qa` checklist. The
Worktrunk pre-merge gate in [../../.config/wt.toml](../../.config/wt.toml) runs
`make lint` and `make test-race`; GitHub CI adds build, race tests, integration
tests, and govulncheck.

## Source routing

- CLI commands: `internal/cli/`, registered from `internal/cli/root.go`.
- tmux boundary: `internal/tmux/`; use `tmux.Runner`, not direct tmux exec.
- Logical tabs and placement: `internal/tabs/`, `internal/tabstate/`, plus
  `internal/cli/tab_target.go` for address resolution.
- Workspace/session identity: `internal/workspace/`, `internal/session/`, and
  `internal/cli/session_target.go`.
- Keybindings/help/palette: `internal/keys/`, `internal/help/`, `internal/tui/palette/`.
  Run `make keys-gen` after keybinding changes.
- QA runner/checklists: `cmd/qa/`, `internal/qa/`, `internal/tui/qapicker/`,
  and `checklists/*.toml`; see [../qa.md](../qa.md).
- Agent integration: `skills/zmux/` for doctrine/hooks and `pi-extension/` for
  typed Pi tools; see [../pi-zmux-extension.md](../pi-zmux-extension.md).
- Terminal evidence/capabilities: [../terminal-current.md](../terminal-current.md),
  [../terminal-capabilities.md](../terminal-capabilities.md), and
  [../terminal-snapshot-correlation-proposal.md](../terminal-snapshot-correlation-proposal.md).

## Style and invariants

- Keep side effects behind typed interfaces (`tmux.Runner`, `config.FS`,
  `qa.CmdRunner`, etc.).
- Do not run `zmux init` inside tmux; it intentionally refuses.
- Route logical-tab name/address changes through `internal/cli/tab_target.go` so
  full, pane, and hidden placements stay addressable.
- Route workspace/session target changes through `internal/cli/session_target.go`;
  raw tmux names are debug/interop fallbacks.
- `zzmux` is the isolated edge profile for live grounding; use it for QA that
  should not touch the active `zmux` profile.
- Long-running or interactive commands belong in zmux tabs/panes, not hidden
  shell jobs.

## Release and docs

Root [CHANGELOG.md](../../CHANGELOG.md) is hand-curated. `cliff.toml` is only a
draft helper for release notes; never regenerate the changelog directly from it.
Forward work lives in [../ROADMAP.md](../ROADMAP.md). Current-state docs should
be updated in the same change as the code they describe.
