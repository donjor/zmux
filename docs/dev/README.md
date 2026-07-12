# Development

How to change zmux safely. Read this after root [AGENTS.md](../../AGENTS.md)
and the docs [route map](../README.md), then jump to the domain/reference guide
that owns the path.

## Read-before-edit route

| Area | Read first |
| ---- | ---------- |
| CLI, tmux behavior, workspace/session, logical tabs | [../architecture.md](../architecture.md), [../reference/cli.md](../reference/cli.md) |
| Keybindings, generated help, command palette | [../reference/keybindings.md](../reference/keybindings.md), `internal/keys/**` |
| QA runner or checklist specs | [qa.md](qa.md), [agent-grounding.md](agent-grounding.md) |
| Agent-facing zmux skill/Pi extension test prompts | [test-prompts/](test-prompts/), [../domains/pi-zmux-extension.md](../domains/pi-zmux-extension.md) |
| Bar density, pane headers, status-line layout | [../domains/bar-density.md](../domains/bar-density.md) |
| Terminal evidence/capabilities | [../reference/terminal-current.md](../reference/terminal-current.md), [../reference/terminal-capabilities.md](../reference/terminal-capabilities.md) |
| Pi extension, shared zmux skill, agent guardrails | [../domains/pi-zmux-extension.md](../domains/pi-zmux-extension.md), [agent-grounding.md](agent-grounding.md) |
| Install/setup/shell integration | [../setup.md](../setup.md), root [README](../../README.md) |

## Commands

```sh
make build                 # compile ./cmd/zmux
make test                  # unit tests
make test-race             # race detector suite; mirrors Worktrunk merge gate
make test-integration      # integration tests; builds first
make gen-doctrine          # explicitly rewrite committed runtime projections
make check-doctrine        # generator tests + non-mutating byte freshness
make test-agent-surfaces   # doctrine gate + Pi extension + QA lint + single doctor
make lint                  # go vet + golangci-lint + gofumpt check
make fmt                   # gofumpt formatting
make vuln                  # govulncheck
./qa lint                  # validate QA walkthrough specs
./qa                       # human QA picker
```

Run the narrow test first, then `make test`. For material CLI, tmux, keybinding,
or TUI behavior, also run `make build` and the relevant `./qa` checklist. For
agent-facing tool/doctrine changes, run `make test-agent-surfaces`; for broad
agent-surface changes, also hand a fresh session the relevant prompt under
[test-prompts/](test-prompts/).

The Worktrunk pre-merge gate in [../../.config/wt.toml](../../.config/wt.toml)
runs `make check-doctrine`, `make lint`, and `make test-race`; the pre-push hook
matches it, and GitHub CI also checks doctrine freshness before build, race tests,
integration tests, and govulncheck.

## Guides

| Guide | Purpose |
| ----- | ------- |
| [agent-grounding.md](agent-grounding.md) | How agents prove visible/tmux behavior in the isolated `zzmux` sandbox. |
| [qa.md](qa.md) | Repo-local QA runner, checklist semantics, and current checklist inventory. |
| [test-prompts/](test-prompts/) | Copy-paste prompts for fresh sessions to audit agent skill/Pi extension surfaces. |

## Source routing

- CLI commands: `internal/cli/`, registered from `internal/cli/root.go`.
- tmux boundary: `internal/tmux/`; use `tmux.Runner`, not direct tmux exec.
- Logical tabs and placement: `internal/tabs/`, `internal/tabstate/`, plus
  `internal/cli/tab_target.go` for address resolution.
- Workspace/session identity: `internal/workspace/`, `internal/session/`, and
  `internal/cli/session_target.go`.
- Keybindings/help/palette: `internal/keys/`, `internal/help/`, `internal/tui/palette/`.
  Run `make keys-gen` after keybinding changes.
- QA runner/checklists: `cmd/qa/`, `internal/qa/`, `internal/tui/qapicker/`, and
  `checklists/*.toml`.
- Agent integration: `agent-doctrine/` for shared truth, `skills/zmux/` for
  Claude mechanics/hooks, and `pi-zmux/` for typed Pi operations/generated guidance.
- Terminal evidence/capabilities: `internal/terminal`, `internal/wm`,
  `internal/snapshot`, and `internal/cli/terminal.go`.

## Style and invariants

- Keep side effects behind typed interfaces (`tmux.Runner`, `config.FS`,
  `qa.CmdRunner`, etc.).
- Do not run `zmux init` inside tmux; it intentionally refuses.
- Route logical-tab name/address changes through `internal/cli/tab_target.go` so
  full, pane, and hidden placements stay addressable.
- Route workspace/session target changes through `internal/cli/session_target.go`;
  raw tmux names are debug/interop fallbacks.
- `zzmux` is the isolated edge profile for live grounding; use `./dev.sh zzmux`
  for QA that should not touch the active `zmux` profile.
- Use `zmux doctor` / `zzmux doctor` when shell lifecycle behavior looks stale.
- Long-running or interactive commands belong in zmux tabs/panes, not hidden
  shell jobs.

## Release and docs

Root [CHANGELOG.md](../../CHANGELOG.md) is hand-curated. `cliff.toml` is only a
draft helper for release notes; never regenerate the changelog directly from it.
Forward work lives in [../ROADMAP.md](../ROADMAP.md). Current-state docs should
be updated in the same change as the code they describe.
