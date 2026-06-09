---
name: zmux
description: "Terminal/session orchestration via zmux, a tmux wrapper. TRIGGER when: managing long-running or interactive terminal processes (dev servers, watchers, queues, REPLs), the user mentions zmux/tmux sessions or tabs, sharing a terminal pane with the user, diagnosing terminal/truecolor capabilities, or driving another real agent CLI (codex, claude, pi, etc.) in a visible zmux tab — as a read-only peer for review/second opinion, or as a write-capable worker bound to a git worktree. Provides rules for named tabs/panes/sessions, agent peer + agent worker doctrine, and keeping work visible to the user."
---

# zmux — Terminal & Session Orchestration

zmux is a tmux wrapper for persistent, user-visible terminal work. Use it for any
process or terminal state that should **outlive a single command**, be **visible to
the user**, or need **interactive control**. For bounded one-shot reads, builds, and
tests whose captured output is the whole artifact, your normal shell is fine.

You are likely working inside a zmux-managed session. Drive zmux through its CLI —
every example below is a shell command.

## Never reach past zmux to raw tmux

zmux **is** the tmux wrapper. Raw `tmux` drops the `@zmux_label` pin + session/
workspace bookkeeping that keep tabs stably addressable — the window can auto-rename
out from under you and your next command lands on the wrong slot. Use the zmux verb;
it's almost always shorter:

| reaching for…                       | use instead                                        |
| ----------------------------------- | -------------------------------------------------- |
| `tmux capture-pane -t X`            | `zmux watch <tab>` (read-only; `--until` baselines)|
| `tmux send-keys -t X …`             | `zmux send <tab> <keys>` / `zmux type <tab> '…'`   |
| `tmux list-windows`                 | `zmux tabs`                                         |
| `tmux list-sessions` / `tmux ls`    | `zmux ls` (`-s` for a flat list)                   |
| `tmux list-panes`                   | `zmux pane list --json`                            |
| `tmux split-window …`               | `zmux pane open <name> -r 35 -- …`                 |
| `tmux select/kill/resize-pane`      | `zmux pane focus / close / resize`                 |
| `tmux new/kill/rename/move-window`  | `zmux run -n` / `zmux tab kill / label / move`     |
| `tmux new-session` / `attach`       | `zmux new` / `zmux open`                           |

A `PreToolUse` guard (`skills/zmux/hooks/zmux-guard.mjs`, symlinked into
`~/.claude/hooks/`) **blocks** these raw calls and prints the mapping back to you —
so a slip self-corrects instead of silently targeting the wrong window. The same
guard enforces the rest of this skill's hygiene: a dev server / background job
(`npm run dev`, `&`, `nohup`) is **blocked** toward `zmux run -n <name> -d`, and an
interactive/remote command (`sudo`, `ssh`, a REPL) draws a non-blocking **warn**
nudging it into a shared tab. Genuinely need the raw command (zmux development,
socket inspection, a one-off)? Prefix `ZMUX_ALLOW=1`, append `# zmux: allow`, use an
explicit `-L <socket>`, or run from the zmux repo — all are exempt.

## Tab states (attention glyphs in the bar)

`zmux tab state <attention|running|done|failed|clear> [tab]` marks a tab's
lifecycle; the bar renders a colored glyph (● needs-human / ◐ running /
✓ done / ✗ failed) visible from any tab. Mostly automatic: `zmux run` sets
running→done/failed; `zmux send`/`type` clear a stale done/failed; focusing
a tab clears attention. Set `attention` manually when handing the human a
prompt they must act on (sudo, permission prompt): `zmux tab state attention
admin --msg 'sudo password'`. A `Stop` hook
(`skills/zmux/hooks/zmux-tab-state-stop.mjs`, symlink into
`~/.claude/hooks/` like the guard) marks the agent's own tab done/attention
when a turn ends — no transcript parsing, just "the turn ended".

## Agent peer

When asked to get a review, second opinion, or agent-to-agent discussion from a
real CLI (`codex`, `claude`, `pi`, etc.), use zmux to drive that CLI in a
visible tab. This is a generic zmux doctrine, not a personal workflow policy.
Read `skills/zmux/references/agent-peer.md` before running the loop.

Minimal loop:

```bash
zmux run 'codex -s read-only -a never' -n codex-peer -d
zmux watch codex-peer --idle 3 -T 30
zmux type codex-peer '<prompt with repo/file pointers>'
zmux watch codex-peer --idle 3 -T 300
```

Do not create a separate peer adapter or manager. zmux owns the visible terminal
mechanics; workflow skills layer above this doctrine.

## Agent worker

The peer's write-capable sibling: drive a real CLI as an **autonomous worker bound
to an isolated git worktree**, where writing + running code in that worktree *is*
the job (not declined, as in peer mode). Same visible-tab mechanics; inverted
permission posture. Generic zmux doctrine — read
`skills/zmux/references/agent-worker.md` before running the loop.

Minimal loop (one worker = one session, bound to one worktree):

```bash
zmux new dev worker-auth                                   # session per worker (when >1 concurrent)
zmux run 'codex -C /path/to/worktree-auth' -n worker-auth-cli -d -s worker-auth
zmux type worker-auth-cli '<brief: worktree path, scope, boundary>' -s worker-auth
zmux watch worker-auth-cli --idle 3 -T 600 -s worker-auth  # long, often async
```

The worktree is the sandbox: writes/exec inside it are routine; network, installs,
auth, and out-of-worktree writes still surface. zmux owns the terminal + lifecycle;
the *fan-out policy* (which worktrees, merge order, who tests in the browser) lives
in the workflow skill above, never here.

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
`echo ":::DONE:::"` markers**. If a tab with that name already exists, the command is
sent to it (reused, not recreated). `-d` creates the tab **without stealing your
focus**. Use `-d` **only** for processes expected to run forever.

> **Tab names are stable.** The first time you address a tab by name with `run`,
> `send`, or `type` and it matches, zmux pins that name as a stable label — so the tab
> stays reachable as `<name>` even after a running process makes tmux rename the
> window (a dev server's window often drifts to `node`/`vite`, or back to the shell
> after you `C-c` it). `watch` resolves the same label but is read-only — it never
> pins. **Caveat:** a tab that already drifted *before* you ever addressed it by name
> has nothing pinned — run `zmux tabs`, then pin it by addressing its **current** name
> with `send`/`type`, or label it explicitly.

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

# Restart in place (reuses the same 'server' tab, even if tmux renamed it)
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
zmux run <recipe>               # recipe form with cwd/workspace/session defaults
zmux run <recipe> -y            # run recipe defaults without prompting
zmux run <recipe> --dry-run     # print the exact recipe plan
zmux recipe list                # list bundled and user recipes
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
zmux tabs [session]              # list tabs — riders nested under hosts, hidden marked ~ (alias: t)
zmux tab move <tab> <dest-session>  # move a tab to another session in the workspace
zmux tab label '<label>'         # set a stable zmux label for the current tab
zmux tab label ''                # clear the label
zmux tab kill <tab>              # kill a tab in the current session
```

Labels are zmux overlays (`label [auto-name]`); they don't disable tmux's automatic
window renaming.

### Placements (pane / full / hide / show)

A zmux tab is a **stable logical unit** (id + label + state), not a window slot. It
can live as a full window, as a pane inside another tab, or hidden in the dock —
and `send`/`type`/`watch`/`run -n` keep reaching it by name in every placement.

```bash
zmux tab pane <tab>                      # join <tab> as a pane beside your current tab
zmux tab pane <tab> --into <host> --down --size 30%   # explicit host + geometry
zmux tab full <tab>                      # promote a pane back to its own tab (--after: next to old host)
zmux tab hide <tab>                      # park it off the bar — process keeps running
zmux tab show <tab>                      # bring it back to the session it was hidden from
```

States and labels ride along: a `running` glyph set on a full tab is still there
when it's a pane or hidden. Placement verbs refuse while grouped viewports
(`-b`/`-c` clones) are attached — detach the extra clients first. `tab show` never
auto-joins; follow with `tab pane` if you want it back as a pane.

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
