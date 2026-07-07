# zmux

An opinionated tmux management wrapper: workspaces, sessions, logical tabs,
themes, status bars, recipes, terminal evidence, and agent-safe command control
from one Go binary.

## Overview

zmux is a layer over tmux, not a replacement for it. It keeps normal tmux
prefix keys, panes, and generated config, then adds:

- workspace-local session labels and a workspace-first picker;
- a popup dashboard, command palette, and help/keybinding surfaces;
- logical tabs that can be full windows, joined panes, or hidden parked tabs;
- theme and status-bar presets with pull-only theme sync;
- recipe-driven workspace/session/tab launch plans;
- terminal commands for reviewable agent and scripting workflows;
- shell lifecycle glyphs and bounded tab output logging.

The active implementation is Go. The original bash prototype is archived under
[`legacy/v0/`](legacy/v0/) and documented as unsupported legacy reference.

## Setup

Requirements:

- tmux 3.2 or newer
- Go toolchain
- Linux or macOS

Install the live profile:

```bash
git clone https://github.com/donjor/zmux.git
cd zmux
./install.sh
```

`./install.sh` checks dependencies, builds `zmux`, installs it to
`~/.local/bin/zmux`, offers shell lifecycle integration, and can launch the
first-run `zmux init` wizard. It writes user config under `~/.zmux.toml` and
profile state under `~/.zmux/`.

Common setup commands:

```bash
make install       # rebuild + copy ~/.local/bin/zmux
make build         # compile ./cmd/zmux into ./zmux
zmux init          # first-run setup wizard; run outside tmux
zmux apply         # regenerate/apply generated tmux config
zmux refresh       # apply config and refresh the current tmux client
zmux doctor        # check shell integration freshness
```

See [`docs/setup.md`](docs/setup.md) for manual install, edge-profile, shell
integration, and update paths.

## Usage

Run `zmux` outside tmux to open the workspace picker. Inside tmux, keybindings
open the dashboard, command palette, and tab/session pickers.

Workspace and session commands:

```bash
zmux new <workspace> [session...]          # create workspace sessions
zmux open <workspace>/<session>            # attach/open a local session label
zmux open <workspace> [session] --pin-view # persistent grouped viewport
zmux fork <new-session-label> [--dir path] # copy tab names/order only
zmux ls [workspace]                        # list workspaces or session labels
zmux tabs [workspace/session]              # list logical tabs
zmux where --json                          # current workspace/session/tab/pane
zmux kill <name>                           # smart workspace/session kill
```

Logical tab and pane commands:

```bash
zmux tab state <state> [tab]       # attention/failed/running/ready/done/clear
zmux tab status <tab> --json       # lifecycle + command/turn metadata
zmux tab inspect <tab> --json      # state + output tail + warnings
zmux tab peer ensure <tab> --command '<cmd>' --json  # safe peer create/reuse
zmux tab pane <tab> [--into host]  # join a tab as a pane
zmux tab full [tab]                # promote pane-of tab back to full tab
zmux tab hide <tab>                # park a tab in the hidden dock
zmux tab show <tab>                # return a hidden tab
zmux pane open <name> --no-focus -- <cmd>
zmux pane list --session           # session joined-pane inventory
```

Recipe, theme, and bar commands:

```bash
zmux run <recipe> --dry-run        # print a recipe plan
zmux run <recipe> -y               # run recipe defaults without prompts
zmux recipe list|show|lint|fork|edit|create
zmux theme set <name>
zmux theme sync                    # pull from configured target
zmux bar <preset>
zmux bar show
```

Agent/scripting terminal commands:

```bash
zmux run 'make test' -n tests -T 180        # reviewable one-shot
zmux run 'python3 -m http.server' -n web -d # detached long-running tab
zmux wait web --for output:'Serving HTTP' --json  # structured wait evidence
zmux wait peer --for turn:ready --json           # fresh peer lifecycle wait
zmux watch web -l 20                             # tail visible output
zmux log start web --ansi                   # bounded background recording
zmux log tail web
zmux send web C-c
zmux type shell 'git status'
zmux type peer 'review this' --mark-peer-running --wait-turn ready --json
zmux snapshot --no-png
zmux terminal current --json
zmux terminal capabilities
```

Terminal commands write to named tmux tabs/panes, not hidden shell jobs. They
return command status, tab lifecycle state, captured output, or evidence paths
depending on the subcommand. Usage errors and unsupported targets fail closed
and are surfaced through normal command exit status plus `--help` output.

## Keybindings

Default prefix: `Ctrl+Space`.

High-traffic bindings:

| Key | Action |
| --- | --- |
| `prefix + Space` | Dashboard |
| `prefix + p` | Command palette |
| `prefix + ?` | Help viewer |
| `prefix + c` | New tab |
| `prefix + n` / `prefix + N` | Next / previous tab |
| `prefix + J` | Join a tab as a pane |
| `prefix + F` | Promote pane-tab to full tab |
| `prefix + w` | Workspace session picker |
| `Alt+w` | Workspace switcher, no prefix |
| ``Alt+` `` | Tab switcher, no prefix |

The generated keybinding source of truth is
[`docs/reference/keybindings.md`](docs/reference/keybindings.md). Update
`internal/keys`, run `make keys-gen`, and let tests verify the generated doc.

## Configuration

User config lives at `~/.zmux.toml`:

```toml
theme = "ayu-dark"
prefix = "C-Space"

[bar]
preset = "default"
layout = "two-line"
indicator = "dots"
top_bar = "tabs"

[recipes]
paths = ["~/.zmux/recipes"]

[sync]
target = "ghostty" # ghostty, nvim, or none
```

Profile state and custom themes live under `~/.zmux/`. The isolated edge binary
`zzmux` uses its own socket, config, and state so maintainers can QA changes
without mutating the active profile.

## Agent integration

zmux exposes an agent-safe terminal control surface:

- use `zmux run`, `watch`, `log`, `send`, `type`, and pane/tab verbs for visible
  terminal work;
- use Pi typed tools from `pi-extension/` when running inside Pi, including `zmux_peer_ensure`, `zmux_tab_inspect`, `zmux_type` peer-wait options, `zmux_callback`, and `zmux_peer_handoff` for agent peer loops;
- use `skills/zmux/SKILL.md` for shared agent doctrine;
- never hide long-running work behind `&`, `nohup`, `disown`, or raw tmux.

Detailed agent and Pi integration docs:

- [`docs/dev/agent-grounding.md`](docs/dev/agent-grounding.md)
- [`docs/domains/pi-zmux-extension.md`](docs/domains/pi-zmux-extension.md)
- [`skills/zmux/SKILL.md`](skills/zmux/SKILL.md)

## Development

```bash
make build
make test
make test-race
make test-integration
make test-agent-surfaces
make lint
make vuln
./qa lint
```

Start with the narrow test, then run `make test`. For command, tmux, keybinding,
TUI, or agent-facing behavior, also run the relevant QA checklist. Maintainers
should use `./dev.sh zzmux` for edge-profile proof before touching live shell or
agent integration.

## Docs

- [`docs/README.md`](docs/README.md) - documentation route map.
- [`docs/product.md`](docs/product.md) - product framing.
- [`docs/architecture.md`](docs/architecture.md) - code map and seams.
- [`docs/setup.md`](docs/setup.md) - install/update/setup details.
- [`docs/dev/README.md`](docs/dev/README.md) - development workflow.
- [`docs/reference/`](docs/reference/) - CLI, keybindings, terminal, and legacy reference.
- [`docs/domains/`](docs/domains/) - domain-specific ownership notes.
- [`docs/ROADMAP.md`](docs/ROADMAP.md) - forward work.
- [`CHANGELOG.md`](CHANGELOG.md) - shipped history.

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md) and root [`AGENTS.md`](AGENTS.md).

## License

TBD. The license has not been finalized; contact the maintainer before reuse or
redistribution.
