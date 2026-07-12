# Reference

Command and generated-surface reference for zmux.

## References

| Guide | Use when |
| ----- | -------- |
| [cli.md](cli.md) | You need the command map, target grammar, output/write behavior, or help/failure expectations. |
| [keybindings.md](keybindings.md) | You change `internal/keys`, generated tmux config, help text, or keybinding docs. |
| [agent-doctrine-matrix.generated.md](agent-doctrine-matrix.generated.md) | You compare shared agent rules, harness enforcement, mechanisms, and known divergences. |
| [terminal-current.md](terminal-current.md) | You work on visible terminal/window resolution. |
| [terminal-capabilities.md](terminal-capabilities.md) | You work on RGB/truecolor capability diagnosis or refresh behavior. |
| [terminal-snapshot-correlation-proposal.md](terminal-snapshot-correlation-proposal.md) | You need the terminal/window snapshot target design history. |
| [legacy-v0.md](legacy-v0.md) | You need archived bash prototype command behavior. |

## Read-before-edit route

- CLI command behavior: read [cli.md](cli.md), then [../architecture.md](../architecture.md).
- Keybindings/help/config generation: read [keybindings.md](keybindings.md), then `internal/keys` and `internal/tmux/conf.go`.
- Agent doctrine capability coverage: read [agent-doctrine-matrix.generated.md](agent-doctrine-matrix.generated.md), then edit `agent-doctrine/` and run `make gen-doctrine`.
- Terminal evidence/capability behavior: read the terminal guide here, then `internal/terminal`, `internal/wm`, `internal/snapshot`, and `internal/cli/terminal.go`.
- Legacy v0 references: read [legacy-v0.md](legacy-v0.md); do not extend `legacy/v0/`.

## Update triggers

Update this index when a reference guide moves, a new CLI/reference surface is
added, or a generated user surface gets a new source of truth.
