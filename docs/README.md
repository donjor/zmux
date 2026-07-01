# zmux Documentation

| Document | Description |
| -------- | ----------- |
| [architecture.md](architecture.md) | Current codebase map, seams, and where changes belong. |
| [ROADMAP.md](ROADMAP.md) | Forward work only. |
| [../CHANGELOG.md](../CHANGELOG.md) | Shipped history. |
| [product.md](product.md) | Current product framing and boundaries. |
| [dev/](dev/) | How to change, verify, and release the repo safely. |
| [VISION.md](VISION.md) | Product vision and design intent. |
| [keybindings.md](keybindings.md) | Generated keybinding reference. |
| [qa.md](qa.md) | Repo-local QA walkthrough runner. |
| [agent-grounding.md](agent-grounding.md) | How an agent grounds/QAs zmux changes on the zzmux sandbox. |
| [bar-density.md](bar-density.md) | Bar density and font-scale feasibility finding. |
| [terminal-current.md](terminal-current.md) | Current terminal/window resolution. |
| [terminal-capabilities.md](terminal-capabilities.md) | Terminal color capability diagnostics. |
| [terminal-snapshot-correlation-proposal.md](terminal-snapshot-correlation-proposal.md) | Snapshot target design record. |
| [pi-zmux-extension.md](pi-zmux-extension.md) | Pi extension integration. |
| [decisions/](decisions/) | Durable decision records. |

## Read before editing

| Path / domain | Read first |
| ------------- | ---------- |
| `cmd/**`, `internal/**`, `tests/**`, `Makefile`, `go.mod` | [dev/](dev/), [architecture.md](architecture.md), root [README](../README.md), root [AGENTS](../AGENTS.md) |
| Keybindings, help, command palette, generated tmux config | [keybindings.md](keybindings.md), [architecture.md](architecture.md) |
| QA runner or checklist specs | [qa.md](qa.md), [agent-grounding.md](agent-grounding.md) for live `zzmux` proof |
| Bar density, tab row, pane headers, terminal capability handling | [bar-density.md](bar-density.md), [terminal-capabilities.md](terminal-capabilities.md) |
| Terminal window resolution or evidence capture | [terminal-current.md](terminal-current.md), [terminal-snapshot-correlation-proposal.md](terminal-snapshot-correlation-proposal.md) |
| `skills/zmux/**`, `pi-extension/**`, agent guardrails | [pi-zmux-extension.md](pi-zmux-extension.md), [agent-grounding.md](agent-grounding.md), root [AGENTS](../AGENTS.md) |
| Repo gates, release drafts, local maintainer overrides | [dev/](dev/), [architecture.md](architecture.md), root [CHANGELOG](../CHANGELOG.md), root [AGENTS](../AGENTS.md) |

Forward planning belongs in [ROADMAP.md](ROADMAP.md); shipped detail belongs in
[../CHANGELOG.md](../CHANGELOG.md).
