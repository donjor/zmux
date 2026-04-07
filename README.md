# zmux

An opinionated, all-in-one tmux management wrapper. Session picker, theming,
status bar presets, and a popup dashboard — in a single binary.

## Features

- **Workspace-primary picker** — single flat list of workspaces with inline session expansion, fuzzy search, ghost tab completion, matched-char underlines
- **Dashboard** — 5-tab popup (prefix+Space): Session, Workspaces, Themes, Settings, Help
- **Command palette** — spotlight-style quick actions (prefix+p)
- **Theming** — 300+ themes (iterm2-color-schemes format), semantic palette, color swatches
- **Theme sync** — pull your theme from Ghostty or Neovim
- **Workspaces** — first-class project containers grouping sessions
- **Status bar** — 9 presets with dynamic segments (git, lang, workspace, directory)
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
5. Optionally installs the Claude Code skill
6. Runs `zmux init` setup wizard

### Updating

After pulling new changes:

```bash
make install        # build + copy to ~/.local/bin/zmux
```

This only rebuilds the binary — no config prompts, no shell changes.

### Manual install

If you prefer to do it yourself:

```bash
make build          # builds ./zmux
make install        # copies to ~/.local/bin/zmux
zmux init           # interactive setup wizard (first time only)
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

### Session & Workspace Management

```
zmux                                Workspace picker (outside tmux) / dashboard (inside)
zmux <workspace>                    Attach workspace's last-active session
zmux <workspace> <session>          Attach specific session in workspace
zmux <name>                         Falls back to session if workspace not found

zmux new                            Create tmp-N session (no workspace)
zmux new <ws>                       Create workspace + 'main' session, attach
zmux new <ws> <session>             Create workspace (if needed) + session, attach
zmux new <ws> <s1> <s2> <s3>        Variadic — create workspace + multiple sessions
zmux new <ws> -t <template>         Create from template in workspace
zmux open <ws> [session]            Open workspace explicitly (alias: zmux o, attach, a)

zmux kill <name>                    Smart kill — workspace-first, then session (confirms if live)
zmux ls                             List workspaces (workspace-primary)
zmux ls <ws>                        List sessions within a workspace
zmux ls -s                          Flat session list (legacy/debug)
zmux tabs [session]                 List tabs in session (alias: zmux t)

zmux tab move <tab> <dest>          Move tab to another session
zmux tab kill <tab>                 Kill a tab
zmux session kill <session>         Kill a session
zmux workspace list                 List workspaces (alias: zmux ws)
zmux workspace kill <workspace>     Kill a workspace and all its sessions
zmux workspace show <ws>            Show workspace sessions
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
zmux bar                       List bar presets with ANSI previews (live carousel inside tmux)
zmux bar <preset>              Set preset directly
zmux bar show                  Show current preset with preview
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

### tmux prefix

| Key | Action |
|-----|--------|
| prefix + Space | Dashboard |
| prefix + p | Command palette |
| prefix + d | Detach |
| prefix + ? | Help popup |
| prefix + c | New tab |
| prefix + n / N | Next / previous tab |
| prefix + < / > | Move tab left / right |
| prefix + x | Close tab (with confirm) |
| prefix + . | Rename tab |
| prefix + , | Rename session |
| prefix + w | Workspace session picker |
| prefix + [ / ] | Prev / next session in workspace |
| prefix + r | Reload config (zmux apply) |
| prefix + v | Enter copy mode (vi keys) |
| Alt+1-9 | Switch to tab (no prefix) |
| Shift+Alt+1-9 | Switch to session N in workspace (no prefix) |
| Alt+` | Tab switcher (no prefix) |

### Picker (outside tmux)

| Key | Action |
|-----|--------|
| ↑ / ↓ | Navigate workspaces and sessions (tree traversal) |
| enter (top action) | Create tmp session (empty input) or workspace+main (typed) |
| enter (workspace) | Attach to last-active, or create main session if empty |
| enter (session) | Attach |
| tab | Accept ghost autocompletion |
| ctrl+x | Delete workspace or session under cursor (with confirm) |
| ctrl+h | Toggle hide-empty workspaces |
| ctrl+t | Open template picker |
| 1-9 | Quick-select session by index |
| esc | Clear query, or quit if empty |
| ctrl+c | Quit |

## Configuration

`~/.zmux.toml`

```toml
theme = "ayu-dark"
prefix = "C-Space"

[bar]
preset = "default"

[bar.segments]
workspace = true
git = true
lang = true
clock = true
directory = true
process = true
group = true

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
| default | Catppuccin-inspired rounded pills, icons, elevated surfaces |
| minimal | Clean, barely decorated, content-first |
| powerline | Angled separators, filled segments, directory chain |
| blocks | Square bracket segments, monospace, dense |
| rounded | Elevated pill segments, premium feel |
| hacker | Matrix-inspired, monospace, dense info |
| zen | Ultra-minimal, barely there |
| starship | Colorful prompt-inspired, each segment its own color |
| rpowerline | Rounded powerline — angled fills with rounded caps |

All presets show dynamic segments: git branch/dirty/ahead-behind, language
version, workspace with session position (e.g. `myapp 2/4`), directory,
active process, and group indicator. Segments are individually toggleable
in `[bar.segments]`.

Preview them: `zmux bar` (live carousel inside tmux, static ANSI outside)

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
