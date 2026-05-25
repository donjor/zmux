---
name: zmux
description: "Terminal/session orchestration via zmux, a tmux wrapper. TRIGGER when: managing long-running or interactive terminal processes (dev servers, watchers, queues, REPLs), the user mentions zmux/tmux sessions or tabs, sharing a terminal pane with the user, or diagnosing terminal/truecolor capabilities. Provides rules for running commands and managing tabs/panes/sessions while keeping work visible to the user."
---

# zmux — Terminal & Session Orchestration

zmux is a tmux wrapper for persistent, user-visible terminal work. Use it for any
process or terminal state that should **outlive a single command**, be **visible to
the user**, or need **interactive control**. For bounded one-shot reads, builds, and
tests whose captured output is the whole artifact, your normal shell is fine.

You are likely working inside a zmux-managed session. Drive zmux through its CLI —
every example below is a shell command.

## Am I in zmux?

```bash
[ -n "$TMUX" ] && echo inside-tmux   # inside a session — commands work without -s
zmux ls                              # workspaces / sessions (works even outside tmux)
zmux tabs                            # tabs in the current session
zmux pane current --json            # current pane/session details
```

If `$TMUX` is unset but `zmux ls` shows sessions, you can still drive them — pass
`-s <session>` on the commands that accept it (`run`, `watch`, `send`, `type`,
`tabs`); `open` takes the workspace/session positionally and `pane` uses `--target`.
**Never fall back to running processes directly just because you're not inside tmux.**

When multiple sessions/workspaces are attached, **do not guess** — inspect with `zmux
ls` / `zmux tabs` / `zmux pane current --json` and target explicitly.

## When to use zmux vs. your shell

Use your **shell directly** for:

- quick reads/searches (`rg`, `ls`, `cat`, `git diff`);
- bounded builds/tests/checks that finish within this turn;
- scripts where the captured stdout/stderr is the artifact.

Use **zmux** for:

- dev servers, file watchers, queues, REPLs — anything that keeps running;
- commands needing user input, passwords, sudo, or manual control;
- shared visibility with the user in a named tab/pane;
- stopping/restarting or inspecting an existing long-running process;
- sidecars / persistent UI panes;
- terminal capability / truecolor diagnosis.

**Never** use `&`, `nohup`, `disown`, or ad-hoc background jobs — use zmux so
processes are named, visible, and controllable. **Never** create unnamed tabs
(always `-n`). **Never** use raw `tmux` for app-level actions; zmux wrappers carry
the labels and session/workspace bookkeeping tmux doesn't.

## Run & observe (core)

### Run commands in named tabs

```bash
zmux run '<cmd>' -n <name>              # run + wait for completion (DEFAULT)
zmux run '<cmd>' -n <name> -T 180       # wait, 180s timeout (default 120)
zmux run '<cmd>' -n <name> -d           # detach — for servers that don't exit
zmux run '<cmd>' -n <name> -f           # follow output live (Ctrl+C to stop)
zmux run '<cmd>' -n <name> -s <session> # target a specific session
```

`zmux run` **waits by default**: it streams output live, then returns the command's
exit code. It injects its own completion sentinel internally — **do not add your own
`echo ":::DONE:::"` markers**. If the tab already exists, the command is sent to it
(reused, not recreated). Use `-d` **only** for processes expected to run forever.

### Read tab output

```bash
zmux watch <tab>                              # last 50 lines
zmux watch <tab> -l 200                       # last 200 lines
zmux watch <tab> -f                           # follow (tail -f style)
zmux watch <tab> --until 'ready|listening' -T 60   # wait for regex, 60s timeout
```

`zmux watch --until` snapshots the buffer when it starts and matches only **new**
output after that baseline — stale "ready" text from a prior run won't cause a false
match. Use `zmux watch` to read state instead of re-running probes that disturb it.

### Send keys / type

```bash
zmux send <tab> C-c          # raw keys (C-c, Enter, Escape, ...)
zmux type <tab> '<text>'     # type text + press Enter
```

`send`/`type` target an **existing** tab — create it first with `zmux run … -n <tab>`
(use `-d` for a persistent shell you'll drive interactively).

### Common patterns

```bash
# One-shot (anything that exits — fast or slow)
zmux run 'go test ./...' -n test -T 180

# Dev server (runs forever)
zmux run 'npm run dev' -n server -d
zmux watch server --until 'Local:|ready|listening' -T 90

# Restart
zmux send server C-c
zmux run 'npm run dev' -n server -d
zmux watch server --until 'ready|listening' -T 90

# Interactive / privileged — run it in a named tab (creates the tab), then ask the user
zmux run 'sudo apt update' -n admin -d
# → tell the user: "sudo command is in the 'admin' tab — enter your password."
```

Do not attempt to automate password entry.

## Sessions & workspaces

```bash
zmux ls [workspace]              # list workspaces, or sessions within one
zmux ls -s                       # flat list of all sessions
zmux new <workspace> [session…] # create workspace + sessions, attach (alias: n)
zmux new -t <template> <ws>     # create from a template
zmux open <ws> [session]         # open/attach a workspace session (aliases: attach, a)
zmux open <ws> <session> --hijack   # take over a session attached elsewhere (advanced)
zmux kill <name>                 # smart kill, workspace-first (alias: k)
zmux session kill <session>      # kill a single session explicitly

zmux ws list                     # workspaces and their sessions
zmux ws add <workspace> <session>   # tag a session to a workspace
zmux ws remove <session>         # untag a session
zmux ws show <workspace>         # sessions in a workspace
zmux ws kill <workspace>         # kill a workspace and all its sessions
```

## Tabs

```bash
zmux tabs [session]              # list tabs (alias: t)
zmux tab move <tab> <dest-session>  # move a tab to another session in the workspace
zmux tab label '<label>'         # set a stable zmux label for the current tab
zmux tab label ''                # clear the label
zmux tab kill <tab>              # kill a tab in the current session
```

Labels are zmux overlays (`label [auto-name]`); they don't disable tmux's automatic
window renaming.

## Panes & sidecars

```bash
zmux pane current --json         # current pane id + details
zmux pane list --json            # panes in current window (also --session, --all, -q)
zmux pane open <name> -r 35 -- <cmd>   # split right at 35%; also -l/-d/-u, --size
zmux pane open --label-tab <name> -r 35 -- <cmd>   # preserve tab label across split
zmux pane toggle <name> -r 35 -- <cmd> # open if absent, close if present (--focus/--replace)
zmux pane focus <pane>           # focus by id/title/index
zmux pane close <pane>           # close by id/title/index
zmux pane resize <pane> --size 40%
```

Split direction: `-r` right, `-l` left, `-d` down, `-u` up (each takes a size).
Use `--label-tab` for sidecars that would otherwise let tmux's auto-rename clobber
the conceptual tab label.

## Terminal capabilities

```bash
zmux terminal current --json       # the visible desktop terminal target (e.g. for screenshots)
zmux terminal capabilities --json  # diagnose RGB/truecolor path (alias: caps)
zmux terminal refresh              # reattach current client to re-resolve RGB features
```

`zmux terminal refresh` (and `zmux refresh` below) **reattaches/redraws the current
client** — it can disturb an active agent connection. Don't run it from an automated
session unless the user asked or disruption is acceptable; otherwise tell the user to
run it.

## Visual snapshots

Capture terminal/TUI evidence when the *visual* state matters — debugging a TUI,
showing a render, or grounding work on another app you're driving in a pane.

```bash
zmux snapshot                       # all panes in current window: text + ANSI + PNG
zmux snapshot --no-png              # text + ANSI only (no screenshot)
zmux snapshot --pane %5 --pane %6   # specific panes (PNG only if both are current-window)
zmux snapshot --lines 400 --json    # more scrollback; print result as JSON
zmux snapshot --out /tmp/run1       # custom output dir
```

Each run writes a bundle to `~/.zmux/snapshots/<timestamp>/` (override with `--out`):
`<pane>.txt`, `<pane>.ansi` (colour-preserving), `<pane>.meta.json`, an optional
`terminal.png`, and `snapshot.json` + `manifest.json` + `README.md`. Read `.ansi`
with `less -R`, `.txt` for plain parsing; `snapshot.json` lists every artifact,
the `modalities` captured, and any `warnings`.

The PNG only ever covers the **current** terminal window (zmux resolves its geometry
strictly and refuses hidden/ambiguous windows rather than screenshotting the wrong
one). It's captured only when every requested pane is in the current window; target
a pane elsewhere with `--pane` and the PNG is skipped (text/ANSI still captured).
Check `warnings` and report blind spots rather than trusting a missing screenshot
as evidence.

## Config & maintenance

```bash
zmux status                      # current theme, bar, prefix, sync target, session count
zmux apply                       # regenerate tmux.conf + apply theme/bar (non-disruptive)
zmux refresh                     # apply config + reattach current client (disruptive — see above)
zmux keys                        # keybinding help
```

Cosmetic/user-facing surfaces — drive these only when the user explicitly asks or
for config troubleshooting, not as part of agent ops:

```bash
zmux theme set <name>            # set theme directly (also: list, sync, pull <target>)
zmux bar [preset]                # list presets, or set one (also: bar show)
zmux init                        # interactive setup wizard — MUST be run outside tmux
```

## Naming conventions

Stable, descriptive tab names:

- `server` — dev servers
- `test` — test runners/watchers
- `build` — builds
- `logs` — log tails
- `admin` — sudo/interactive commands
- `<tool>-sidecar` — UI sidecars

## Avoid

- Starting servers/watchers in your own shell — use `zmux run -d`.
- `&`, `nohup`, `disown`, or any ad-hoc background job.
- Creating unnamed tabs — always `-n`.
- Guessing process state instead of reading it with `zmux watch` / `zmux tabs`.
- Adding your own `:::DONE:::` markers — `zmux run` handles sentinels.
- `sleep N && zmux watch` — `zmux run` already waits.
- Raw `tmux` for ordinary zmux-managed actions.
- `zmux refresh` / `zmux terminal refresh` from an agent session without weighing the
  client-reattach disruption.
- `zmux init` inside tmux — it refuses; exit the session first.
