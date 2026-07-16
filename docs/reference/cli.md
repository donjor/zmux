# CLI reference

This is the map for the `zmux` command surface. It is not a replacement for
`zmux --help`; it tells maintainers where commands belong and what behavior the
docs promise.

## Invocation

```bash
zmux [command] [flags]
zmux <workspace-or-query>
```

With no command, `zmux` opens the workspace picker outside tmux and the
dashboard flow inside tmux. Commands that target a session accept the normal
`workspace/session` grammar and usually fall back to the current workspace or
current tmux context. `pane list --session --target <session>` follows that
session grammar; `pane list --target <session>` without `--session` is a raw
pane/window target and may fail with tmux's raw-target error.

## Command groups

### Workspace and session

```bash
zmux new [workspace] [session...]
zmux open <workspace> [session]
zmux open <workspace>/<session>
zmux fork <new-session-label> [--dir path]
zmux kill <name>
zmux ls [workspace]
zmux workspace list|show|kill
zmux session kill <session>
zmux session run <session> -n <tab> -- <cmd>
zmux where [--json]
```

Purpose: create, attach, list, fork, inspect, and remove workspace-scoped
sessions. These commands write tmux sessions/windows and zmux workspace state.
Failures include ambiguous targets, unsupported tmux context, and live-session
confirmation refusal.

### Logical tabs and panes

```bash
zmux tabs [workspace/session]
zmux tab label [label]
zmux tab state <attention|failed|running|ready|done|clear> [tab]
zmux tab status <tab> [--json]
zmux tab inspect <tab> [--json] [-l lines]
zmux tab peer ensure <tab> [--command cmd] [--readiness regex] [--wait-turn state] [--json]
zmux tab pane <tab|N> [--into host] [--focus]
zmux tab split [--focus]
zmux tab full [tab] [--pane pane-id]
zmux tab hide <tab> [--pane pane-id]
zmux tab show <tab> [--focus]
zmux tab move <tab> <destination> [--session source]
zmux tab kill <tab> [--session source]
zmux pane open <name> [--no-focus] -- <cmd>
zmux pane toggle <name> -- <cmd>
zmux pane current|list|focus|resize|close
```

Purpose: manage pane-canonical logical tabs across full-window, joined-pane, and
hidden-dock placements. These commands mutate tmux panes/windows and pane-scoped
`@zmux_*` metadata. Missing tab targets fail closed instead of falling back to
the current pane.

`zmux pane current --json` resolves the complete row for the calling
`$TMUX_PANE`, even when that pane is in a visible tab other than the attached
client's active window. This makes it safe for background agent tabs to discover
their own session, window, cwd, and process metadata.

### Reviewable commands and output

```bash
zmux run '<cmd>' -n <tab> [-T seconds] [-d] [--no-focus] [-f] [--keep] [--scope daemon]
zmux wait <tab> --for turn:ready[,failed,attention]|cmd:done|output:<regex>|idle:<duration> [--json]
zmux watch <tab> [-l lines] [--until pattern] [--idle seconds] [-f]
zmux log start <tab> [--ansi] [--max-bytes n]
zmux log status                    # global recording view; no -s/--session
zmux log tail <tab>
zmux log stop <tab>
zmux send <tab> <keys...>
zmux type <tab> '<text>' [--wait-turn state|--wait-cmd state] [--mark-peer-running] [--json]
```

Purpose: run commands in named tmux tabs, wait/follow/tail output, record a
bounded log, or send input. `run` writes command lifecycle metadata; `-d` returns
without waiting, while `--no-focus` independently prevents a newly created tab
from being selected and may be combined with either blocking or detached execution. Detached
long-running commands can add `--until <regex>` to prove fresh startup output against a baseline
captured before command delivery; this avoids missing readiness printed immediately after launch.
`wait` is
the structured condition primitive for fresh command state, fresh turn state,
future output regex, and idle/quiet evidence; `watch` and `log tail` remain
human-friendly output readers. `send` and `type` mutate the target pane, and
`type` can now return structured wait outcomes for peer turns or shell command
completion. Use `--help` for supported flags before documenting new agent
workflows.

### Recipes

```bash
zmux run <recipe> [-y] [--dry-run] [--tab-mode run|ready]
zmux run --command '<cmd>'
zmux recipe list|show|lint|fork|edit|create
```

Purpose: launch declarative TOML plans from bundled or user recipe directories.
`--dry-run` prints the planned writes; non-dry runs create sessions/tabs and may
type commands. Recipe parsing and planning live under `internal/recipe`.

### Theme, bar, setup, and generated config

```bash
zmux init
zmux setup shell
zmux apply
zmux refresh
zmux doctor
zmux theme set|list|sync|pull
zmux bar [preset]
zmux bar show
zmux status
zmux completion <bash|zsh|fish>
zmux help
```

Purpose: configure the profile, generated tmux config, shell integration,
themes, status bar, and help/completion surfaces. `apply` and `refresh` write or
reload tmux config; theme sync is pull-only from external tools.

### Terminal evidence

```bash
zmux terminal current --json
zmux terminal capabilities
zmux snapshot [--no-png]
```

Purpose: resolve the visible desktop terminal, diagnose RGB/truecolor capability
state, and capture text/ANSI plus optional PNG evidence. These commands are used
by agents and QA to ground visual terminal behavior.

### Maintainer and hidden surfaces

```bash
zmux keys gen
zmux log-sink
./qa [ls|run|mark|status|reset|lint]
```

`keys gen` regenerates generated keybinding docs from `internal/keys`.
`log-sink` is the hidden pipe-pane sink behind `zmux log`. `./qa` is a separate
repo-local QA runner from `cmd/qa`, not a `zmux` subcommand.

## Output and failure expectations

- Human commands print plain text or open Bubble Tea UIs.
- Tooling commands support `--json` where structured state is promised.
- Mutating commands should name the target they changed or fail before mutation.
- Unknown flags, missing args, ambiguous targets, missing tabs, stale shell
  integration, and unsupported tmux contexts should exit non-zero.
- Every public command should have useful `--help` text.

## Update triggers

Update this file when `internal/cli/root.go`, command target grammar, public
flags, output format, failure behavior, or the separate `./qa` runner changes.
