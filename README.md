# zmux

An opinionated, all-in-one tmux management wrapper. Session picker, theming,
status bar presets, and a popup dashboard — in a single binary.

## Features

- **Session picker** — fuzzy search, create, attach, templates
- **Dashboard** — tabbed view as a tmux popup (prefix+Space)
- **Command palette** — spotlight-style quick actions (prefix+p)
- **Theming** — 300+ themes (iterm2-color-schemes format), semantic palette, color swatches
- **Theme sync** — pull your theme from Ghostty or Neovim
- **Status bar** — 4 presets: default, minimal, powerline, blocks
- **Templates** — declarative TOML session layouts
- **Terminal commands** — run, watch, send, type for agent/scripting workflows
- **Multi-source discovery** — find sessions across tmux sockets and overmind
- **Init wizard** — interactive TUI setup
- **Shell completions** — bash, zsh, fish

## Requirements

- tmux >= 3.2
- Linux or macOS

## Install

```bash
git clone https://github.com/donjor/zmux.git
cd zmux
./install.sh
```

The installer:
1. Checks dependencies (Go, tmux >= 3.2)
2. Builds the binary
3. Installs to `~/.local/bin/zmux`
4. Optionally adds shell integration (auto-start on terminal open)
5. Tells you to run `zmux init`

### Manual install

If you prefer to do it yourself:

```bash
make build          # builds ./zmux
make install        # copies to ~/.local/bin/zmux
zmux init           # interactive setup wizard
```

### Legacy v0 (bash+gum)

The original bash prototype is still in the repo as `bin/zmux0`:

```bash
./install-v0.sh     # links zmux0 to ~/.local/bin/zmux0
```

## Quick start

After install, run the setup wizard:

```bash
zmux init
```

This creates `~/.zmux.toml`, generates `~/.tmux.conf`, and sets up
`~/.zmux/` directories. Restart tmux or `prefix+r` to reload.

To apply theme + bar to a running tmux without the wizard:

```bash
zmux apply
```

After `zmux init`, restart tmux or `prefix+r` to reload config.

## Usage

### Session Management

```
zmux                           Session picker (outside tmux) / dashboard (inside)
zmux <name>                    Attach or create session (shorthand)

zmux new [name]                Create session + attach (alias: zmux n)
zmux new -t <tmpl> [name]      Create from template
zmux attach <name>             Attach to existing session (alias: zmux a)
zmux attach --mirror <name>    Shared view (independent viewport)
zmux attach --hijack <name>    Steal session from other client
zmux kill <name>               Kill session (alias: zmux k)
zmux ls                        List sessions
zmux tabs [session]            List tabs in session (alias: zmux t)
```

### Terminal Commands

Run commands in named tmux windows, read their output, and send keystrokes.
Designed for scripting and agent workflows (see [Agent Integration](#agent-integration)).

```
zmux run '<cmd>' -n <tab>      Run command, wait for completion
zmux run '<cmd>' -n <tab> -d   Run detached (for servers)
zmux run '<cmd>' -n <tab> -f   Run + follow output
zmux watch <tab>               Capture tab output
zmux watch <tab> --until <pat> Wait for pattern match
zmux watch <tab> -f            Follow output (tail -f)
zmux send <tab> <keys>         Send keystrokes to tab
zmux type <tab> '<text>'       Type text + Enter
```

### Theming

```
zmux theme                     Interactive theme picker
zmux theme set <name>          Set theme directly
zmux theme list                List available themes
zmux theme sync                Pull theme from default sync target
zmux theme pull <target>       Pull theme from ghostty or nvim
```

### Configuration

```
zmux bar                       List bar presets with ANSI previews
zmux bar <preset>              Set preset (default/minimal/powerline/blocks)
zmux init                      Setup wizard (run outside tmux)
zmux apply                     Regenerate + apply config
zmux status                    Show current config summary
```

### Other

```
zmux version                   Print version
zmux completion <shell>        Generate completions (bash/zsh/fish)
zmux help                      Styled help with keybindings
```

## Keybindings

Prefix: `Ctrl+Space` (configurable)

| Key | Action |
|-----|--------|
| prefix + Space | Open zmux dashboard popup |
| prefix + p | Open command palette popup |
| prefix + d | Detach from session (back to terminal) |
| prefix + ? | Open help popup |
| prefix + v | Enter copy mode (vi keys) |
| prefix + c | New window |
| prefix + n | Next window |
| prefix + , | Rename session |
| prefix + . | Rename tab |
| prefix + s | Switch session |
| prefix + x | Kill session |
| prefix + r | Reload config (zmux apply) |
| Alt+1-5 | Switch to window (no prefix) |

## Configuration

`~/.zmux.toml`

```toml
theme = "ayu-dark"
prefix = "C-Space"

[bar]
preset = "default"

[sessions]
auto_cleanup_tmp = true

[templates]
paths = ["~/.zmux/templates"]

[sync]
target = "ghostty"          # ghostty, nvim, or none
ghostty_config = "auto"
```

## Themes

zmux uses the [iterm2-color-schemes](https://github.com/mbadolato/iTerm2-Color-Schemes)
format — the same upstream used by Ghostty, Alacritty, Kitty, and others.

**11 bundled themes** ship with the binary (no downloads needed):
ayu-dark, atom-one-dark, carbonfox, catppuccin-mocha, dracula, gruvbox-dark,
kanagawa-dragon, material-darker, nord, rose-pine, tokyonight.

**Theme resolution order:**
1. `~/.zmux/themes/<name>` — your custom themes
2. Bundled (embedded in binary)
3. `~/.zmux/themes/iterm2/<name>` — downloaded set from `zmux init`

### Theme sync

zmux can pull your theme from another tool (read-only, never writes to their config):

```bash
zmux theme sync              # pull from default target
zmux theme pull ghostty      # read Ghostty's theme
zmux theme pull nvim         # read Neovim's colorscheme
```

## Session templates

Declarative TOML files that define window layouts:

```toml
name = "dev"
description = "Editor, server, and git"

[[windows]]
name = "editor"
command = "nvim ."

[[windows]]
name = "server"

[[windows]]
name = "git"

[options]
focus = "editor"
```

Place custom templates in `~/.zmux/templates/`. Four built-in templates
are included: dev, claude, webdev, monitor.

## Status bar presets

| Preset | Description |
|--------|-------------|
| default | Session pill, window tabs, prefix hints, clock |
| minimal | Session name and windows only |
| powerline | Angled separators, filled segments |
| blocks | Square bracket segments |

Preview them: `zmux bar`

## Agent Integration

zmux includes terminal commands (`run`, `watch`, `send`, `type`) designed for
AI agent workflows. These let agents manage long-running processes without
blocking their own shell.

**Key principles:**
- Never run dev servers in the agent's shell — use `zmux run -n server -d`
- Never use `&` for background tasks — use `zmux run`
- Wait for commands with `zmux run 'make build' -n build` (waits by default)
- Read output with `zmux watch`, don't re-run commands to check status
- For sudo/interactive commands, use `zmux type admin 'sudo ...'`

**Example workflow:**
```bash
zmux run 'npm test' -n test               # waits for completion, returns exit code
zmux run 'npm run dev' -n server -d       # detach for servers
zmux watch server --until "listening"     # wait for ready signal
zmux watch server -l 20                   # peek at output
zmux send server C-c                      # stop server
```

A Claude Code skill is installed automatically by `./install.sh` or `./dev.sh`
to `~/.claude/skills/zmux/`. Source: `skills/zmux/SKILL.md`.

## Development

```bash
make build            # build binary
make test             # run unit tests
make test-integration # run integration tests (needs tmux)
make lint             # go vet + staticcheck
make clean            # remove build output
```

## License

MIT
