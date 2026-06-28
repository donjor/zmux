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
workspace bookkeeping that keep tabs stably addressable — the window auto-renames out
from under you and your next command lands on the wrong slot. Reach for the zmux verb
(almost always shorter): `watch`/`send`/`type` over `capture-pane`/`send-keys`,
`tabs`/`ls` over `list-windows`/`list-sessions`, `pane open` over `split-window`,
`run -n` over `new-window`.

A `PreToolUse` guard (`hooks/zmux-guard.mjs`, symlinked into `~/.claude/hooks/`)
**blocks** raw tmux and ad-hoc background jobs — shell `&`/`nohup`, `npm run dev`,
**and the Bash tool's own `run_in_background: true`** — and prints the right verb
back, so a slip self-corrects. The full mapping table, guard
exemptions, and tab-state glyph behavior → **`references/guard-and-tab-states.md`**.

A tab also carries a lifecycle glyph in the bar (● needs-human / ◐ running / ✓ done /
✗ failed), mostly set automatically by `run`/`send`/`type` + a `Stop` hook. Set it by
hand only to flag the human: `zmux tab state attention <tab> --msg 'sudo password'`.

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
The worktree is the sandbox; zmux owns the terminal + lifecycle, while *fan-out
policy* (which worktrees, merge order, who tests) lives in the workflow skill above,
never here. Permission posture + boundaries → `references/agent-worker.md`.

## Am I in zmux?

```bash
[ -n "$TMUX" ] && echo inside-tmux   # inside a session — commands work without -s
zmux where                           # current context: workspace / session / tab / pane / cwd (alias: whoami)
zmux ls                              # all workspaces / sessions (works even outside tmux)
zmux tabs                            # tabs in the current session
```

`zmux where` is the one-shot "where am I" — the raw `zws_…` name it prints under
`session` is exactly what you pass as `-s`. (`zmux pane current --json` stays the
pane-only primitive.)

Outside tmux but `zmux ls` shows sessions? Drive them with `-s <session>` (accepted
by `run`/`watch`/`send`/`type`/`tabs`/`log`/`tab state` + `tab` placements; the three
forms and ambiguity rules → `references/cli-catalog.md`). **Never** run processes
directly just because you're outside tmux, and **never** guess between attached
sessions — list them with `zmux ls` first (`zmux where` is in-tmux only).

## When to use zmux vs. your shell

The test is **reviewability, not duration**: would a human want to *see, grab, or
re-run* this?

Your **shell** for: quick reads/searches (`rg`, `cat`, `git diff`); bounded
builds/tests/checks whose captured stdout is the whole artifact — even a slow
`go test` is fine inline.

**zmux** for: anything that keeps running (dev servers, watchers, queues, REPLs); a
command that mutates or runs the project in a way worth a glance; input/passwords/
sudo/manual control; stopping/inspecting an existing long-running process; sidecars;
terminal capability diagnosis.

**Reuse a tiny roster — never a fresh tab per task.** Address by *purpose*; `run -n
<name>` reuses an existing tab, so related work stays together:

- `claude` / `codex` — the session's agent shell (long-lived, not a task tab).
- `dev` — the project runtime: server, service, the main process you stop & restart.
- `scratch` — reviewable one-offs: mutations, manual takeover, things to inspect/re-run.
- `<x>-peer` — a review peer (peer skill); `worker-*` — orchestrate sessions.

Inventing `eval-2` / `test-run` is the slip — that's `scratch` (or `dev`). Don't add
`test` / `build` / `lint` tabs; bounded checks stay in your shell. **Re-running the
same job?** Re-fire `run -n <name>` into its existing tab — it reuses (the prior
process has exited), so never bump the name `x`→`x2`→`x3`. Full roster +
reviewability detail → **`references/guard-and-tab-states.md`**.

**Tab hygiene — spawn less, tear down after.** The guard pushes long-running work
into named tabs; the unspoken other half is not leaving sprawl behind.

- **Tear down when the task ends.** Kill task-scoped tabs (peer, worker, ad-hoc
  `run`) once their work is integrated; don't park dead shells. `zmux tab kill <name>`
  / `zmux session kill <session>` (the `zmux tabs` index number also kills).
- **The reaper backstops you, but don't lean on it.** Idle agent-task tabs auto-expire
  (~1h); `zmux reap --dry-run` shows the verdicts. Protect a long-lived tab with
  `run --keep` or `run --scope daemon` (never reaped); peers/workers are already exempt.
  Auto-reaping is housekeeping for what you forgot, not a substitute for tearing down.
- **Don't wrap an app-detached daemon in a tab.** A process the app itself backgrounds
  (its own `setsid`/session, surviving independent of any pane) needs no zmux tab — the
  tab is pure overhead. Tabs are for processes *you* babysit.

**Never:**

- `&`, `nohup`, `disown`, or the Bash tool's `run_in_background: true` — any
  ad-hoc background job — use `zmux run -d` so processes stay named, visible, and
  controllable. The harness param backgrounds work off-screen just like shell `&`;
  the guard blocks it the same way.
- unnamed tabs — always `-n`.
- raw `tmux` for app-level actions (the guard blocks it).
- your own `:::DONE:::` markers (`zmux run` handles sentinels) or `sleep N && watch`
  (`run` already waits).
- a hand-rolled poll-loop / external watcher to await one job — `zmux run -n`
  already waits and `zmux watch` reads progress. And never `pgrep -f` / `pkill -f` a
  pattern that also matches your own command line: it self-matches (false "alive")
  and `pkill` can SIGKILL your own shell.
- guessing process state — read it with `zmux watch` / `zmux tabs`.
- `zmux refresh` / `terminal refresh` from an agent session without weighing the
  client-reattach disruption.
- `zmux init` inside tmux — it refuses; exit the session first.

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

> **A long run reported "failed, exit 1" at ~120s is the wait cap, not your command.**
> That's the harness wrapper hitting its timeout while the process keeps running in
> the tab. Verify with `zmux watch <tab>` (or the log) before concluding failure or
> relaunching — and prefer `run -n <tab> -T <secs>` (or `-d` + `watch`) for jobs you
> know run long.

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

For **persistent** recording that survives detach and self-truncates, use `zmux log`
— `watch` only reads the live buffer:

```bash
zmux log start <tab>          # record output to a bounded file (background, survives detach)
zmux log start <tab> --ansi   # keep colour instead of stripping to plain
zmux log tail <tab>           # print the recorded log
zmux log status               # what's being recorded
zmux log stop <tab>           # stop
```

`log` is for line-oriented output (servers, builds, tests); a fullscreen TUI records
as escape soup. For live following use `zmux watch <tab> -f`.

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

The exhaustive verb tables live in **`references/cli-catalog.md`** — sessions/
workspaces, tabs, placements (`tab pane/full/hide/show`), panes & sidecars, terminal
capabilities, visual snapshots, output recording (`zmux log`), `-s` session-targeting
forms, and naming conventions. Read it when you need a specific verb.
