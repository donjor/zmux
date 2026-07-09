# zmux Documentation

Start here when deciding which doc governs a source path.

| Document | Description |
| -------- | ----------- |
| [product.md](product.md) | Current product framing and boundaries. |
| [architecture.md](architecture.md) | Current codebase map, seams, and source routing. |
| [setup.md](setup.md) | Install, update, shell integration, and profile setup. |
| [dev/](dev/) | Development workflow, QA route, and agent grounding guides. |
| [reference/](reference/) | CLI, keybindings, terminal, and legacy command reference. |
| [domains/](domains/) | Domain-specific ownership notes and local invariants. |
| [decisions/](decisions/) | Durable decision records. |
| [ROADMAP.md](ROADMAP.md) | Forward work only. |
| [VISION.md](VISION.md) | Product vision and design intent. |
| [../CHANGELOG.md](../CHANGELOG.md) | Shipped history. |

## Read before editing

| Path / domain | Read first |
| ------------- | ---------- |
| `cmd/**`, `internal/**`, `tests/**`, `Makefile`, `go.mod` | [dev/](dev/), [architecture.md](architecture.md), root [README](../README.md), root [AGENTS](../AGENTS.md) |
| CLI command behavior and target grammar | [reference/cli.md](reference/cli.md), [architecture.md](architecture.md) |
| Keybindings, help, command palette, generated tmux config | [reference/keybindings.md](reference/keybindings.md), [architecture.md](architecture.md) |
| QA runner or checklist specs | [dev/qa.md](dev/qa.md), [dev/agent-grounding.md](dev/agent-grounding.md) |
| Bar density, tab row, pane headers, status-line layout | [domains/bar-density.md](domains/bar-density.md) |
| Terminal window resolution, RGB, or evidence capture | [reference/terminal-current.md](reference/terminal-current.md), [reference/terminal-capabilities.md](reference/terminal-capabilities.md) |
| `skills/zmux/**`, `skills/zmux/test/**`, `pi-zmux/**`, agent guardrails | [domains/pi-zmux-extension.md](domains/pi-zmux-extension.md), [dev/agent-grounding.md](dev/agent-grounding.md), root [AGENTS](../AGENTS.md) |
| Repo gates, release drafts, local maintainer overrides | [dev/](dev/), [setup.md](setup.md), root [CHANGELOG](../CHANGELOG.md), root [AGENTS](../AGENTS.md) |

Forward planning belongs in [ROADMAP.md](ROADMAP.md). Shipped detail belongs in
[../CHANGELOG.md](../CHANGELOG.md).
