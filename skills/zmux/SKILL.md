---
name: zmux
description: "Terminal/session orchestration via zmux, a tmux wrapper. TRIGGER when: managing long-running or interactive terminal processes (dev servers, watchers, queues, REPLs), the user mentions zmux/tmux sessions or tabs, sharing a terminal pane with the user, diagnosing terminal/truecolor capabilities, or driving another real agent CLI (codex, claude, pi, etc.) in a visible zmux tab — as a max-permission prompt-scoped review peer, or as a write-capable worker bound to a git worktree. Provides rules for named tabs/panes/sessions, agent peer + agent worker doctrine, and keeping work visible to the user."
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
zmux run 'codex --dangerously-bypass-approvals-and-sandbox' -n codex-peer -d
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
# session run = one call: detached session + the CLI as its first tab, no focus
# steal, no blank tab, tagged into the current workspace (--workspace to target another).
zmux session run worker-auth -n worker-auth-cli -- codex -C /path/to/worktree-auth -s workspace-write -a on-request
zmux type worker-auth-cli '<brief: worktree path, scope, boundary>' -s worker-auth
zmux watch worker-auth-cli --idle 3 -T 600 -s worker-auth  # long, often async
```

Use `zmux session run`, **never** `zmux new <ws> <worker-session>` for automated
spawn — `new` attaches (steals the conductor's focus) and births a blank shell tab.

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

## Logical tabs (why addressing by name works)

A zmux tab is a **stable logical unit** (id + label + state), not a window slot —
it can be a full window, a pane inside another tab, or hidden in the dock, and
`run -n`/`send`/`type`/`watch` keep reaching it **by name** in every placement.
Address tabs by their zmux name, never a tmux window index.

## Full command catalog → `references/cli-catalog.md`

Sessions/workspaces, tabs, placements (`tab pane/full/hide/show`), panes &
sidecars, terminal capabilities, visual snapshots, and config/maintenance — the
exhaustive verb tables live in **`references/cli-catalog.md`**. Read it when you
need a specific verb. The ones you'll reach for most:

```bash
zmux ls -s                             # flat list of all sessions
zmux session kill <session>            # kill a session
zmux tab move <tab> <dest>             # move a tab to another session
zmux tab pane <tab> / full / hide / show   # placements (pane / promote / dock)
zmux pane open <name> -r 35 -- <cmd>   # split a sidecar pane
zmux snapshot                          # capture text + ANSI + PNG of the current window
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
