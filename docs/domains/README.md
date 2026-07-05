# Domains

Domain ownership notes for source areas that need more than the root
architecture map.

## Owned paths

- `internal/bar/**` and `internal/tmux/conf.go` are covered by [bar-density.md](bar-density.md) when the change affects status-line density or pane-header display.
- `pi-extension/**` and `skills/zmux/**` are covered by [pi-zmux-extension.md](pi-zmux-extension.md) when the change affects Pi typed tools or agent guardrails.
- Other source roots fall back to [../architecture.md](../architecture.md) until they gain their own domain doc.

## Invariants

- Domain docs are local rule surfaces, not broad product/vision prose.
- A domain doc must name the source paths it owns and the tests or docs that drift with them.
- Keep root [AGENTS.md](../../AGENTS.md), [../dev/README.md](../dev/README.md), and the domain doc aligned when a local invariant changes.

## Reusable primitives

- Use [../architecture.md](../architecture.md) for the package map and interface seams.
- Use [../reference/cli.md](../reference/cli.md) for command behavior.
- Use [../dev/README.md](../dev/README.md) for verification commands.
- Use the domain docs below for local hazards and update triggers.

## Split-logic warnings

- Do not duplicate package maps from architecture in every domain doc.
- Do not bury command usage in a domain doc when it belongs in reference docs.
- Do not turn a short ownership note into a directory; a flat `docs/domains/<name>.md` is enough until there are multiple guide docs.

## Guide map

| Domain doc | Source area |
| ---------- | ----------- |
| [bar-density.md](bar-density.md) | `internal/bar/**`, status-line density behavior, pane-header display constraints. |
| [pi-zmux-extension.md](pi-zmux-extension.md) | `pi-extension/**`, `skills/zmux/**`, Pi typed tools, and agent guardrails. |

## Read-before-edit route

- Start with root [AGENTS.md](../../AGENTS.md) and [../README.md](../README.md).
- Read [../architecture.md](../architecture.md) for the package map.
- Read the domain doc that owns the paths you will touch.
- Read [../dev/README.md](../dev/README.md) for verification commands.

## Update triggers

Add or update a domain doc when a source area gains local invariants, reusable
primitives, split-logic hazards, or update rules that do not belong in the root
architecture map.
