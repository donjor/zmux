---
name: zmux
description: "Terminal/session orchestration through zmux, a tmux wrapper. Use when managing long-running or interactive processes, shared visible tabs/panes, terminal/TUI evidence, zmux/tmux sessions, or driving real agent CLIs in visible zmux tabs. Does NOT fire for ordinary bounded shell checks, peer/orchestrate policy decisions without terminal driving, or app work unrelated to terminal/session control."
---

# zmux — Terminal & Session Orchestration

zmux is a tmux wrapper for persistent, user-visible terminal work. Use it for any
process or terminal state that should **outlive a single command**, be **visible to
the user**, or need **interactive control**. For bounded one-shot reads, builds,
and tests whose captured output is the whole artifact, your normal shell is fine.

You are likely already inside a zmux-managed session. In Pi, prefer the typed
`zmux_*` tools for the same operations; in other harnesses, use `zmux` CLI verbs.

## Route

| Need | Read / use |
| --- | --- |
| Concrete run/watch/send/type examples | `references/run-observe.md` |
| Full CLI catalog, `-s` forms, snapshots, panes, config | `references/cli-catalog.md` |
| Guard mapping, roster details, lifecycle glyphs | `references/guard-and-tab-states.md` |
| Real CLI as prompt-scoped review peer | `references/agent-peer.md` |
| Real CLI as write-capable worktree worker | `references/agent-worker.md` |

## Core invariants

### Never reach past zmux to raw tmux

zmux **is** the tmux wrapper. Raw `tmux` drops the `@zmux_label` pin and
session/workspace bookkeeping that keep tabs stably addressable. Reach for the
zmux verb instead: `watch`/`send`/`type` over `capture-pane`/`send-keys`,
`tabs`/`ls` over `list-windows`/`list-sessions`, `pane open` over
`split-window`, `run -n` over `new-window`.

Harness guardrails backstop this doctrine. Claude has hook-backed blocking; Pi
has typed tools plus a bash guard. The full mapping table, bypasses, and lifecycle
state details live in `references/guard-and-tab-states.md`.

### Use zmux for reviewability, not duration

Use your **shell** for quick reads/searches and bounded checks whose captured
stdout is the whole artifact — even a slow `go test` can stay inline.

Use **zmux** for software that keeps running (servers, watchers, queues, REPLs),
commands a human may want to see/grab/re-run, input/passwords/sudo/manual
control, existing long-running process inspection, sidecars, terminal capability
diagnosis, and visual/TUI evidence.

### Reuse a tiny roster

Address tabs by purpose. `zmux run -n <name>` reuses an existing tab, so related
work stays together:

- `dev` — project runtime: server, service, main process, REPL.
- `scratch` — reviewable one-offs, manual takeover, things to inspect/re-run.
- `<agent>-peer` — a review peer; `worker-*` — orchestrated worker sessions.
- `claude` / `codex` / `pi` / `agy` — long-lived primary agent shells, not task tabs.

Do **not** mint `eval-2`, `test-run`, `build-x`, or feature-named tabs. Bounded
checks stay in your shell; odd reviewable commands go to `scratch`; the main
runtime goes to `dev`. Re-running the same job means re-fire the same `run -n
<name>` target, never suffix-bump `x` → `x2` → `x3`.

### Joined panes are roster tabs too

Before minting a fresh visible long-running tab, check whether this session
already has a joined logical tab for that purpose:

```bash
zmux pane list --joined --session --json
```

When targeting another session, add `--target <session>`. If a row matches, route
through its `tabName` with the normal resolver: `zmux run '<cmd>' -n <tabName>
-s <session>`. Do not target raw `paneID` for normal work; keep state, logging,
placement, and lifecycle on the logical tab.

### Tear down after the task

Kill task-scoped peer, worker, or ad-hoc `run` tabs once their work is integrated.
The reaper backstops forgotten agent-task tabs, but it is housekeeping, not a
substitute for cleanup. Protect intentional long-lived tabs with `run --keep` or
`run --scope daemon`.

Do not wrap app-detached daemons in a tab. If the app backgrounds itself and
survives independently (systemd, Docker `-d`, its own `setsid`), a zmux wrapper
tab is pure overhead.

## Pi typed tools

In Pi, use typed tools instead of shelling out for common zmux operations:

- `zmux_current` — inspect current session/tab/pane, binary/profile, project trust, config.
- `zmux_run` — reviewable command-in-tab one-shots.
- `zmux_runtime_ensure` / `zmux_runtime_logs` / `zmux_runtime_stop` — persistent runtimes.
- `zmux_interactive_type` — sudo, SSH, REPLs, database shells, manual input.
- `zmux_tabs`, `zmux_sessions`, `zmux_session_run`, `zmux_session_kill` — tab/session control.
- `zmux_tab_*`, `zmux_pane_*`, `zmux_log`, `zmux_snapshot`, `zmux_terminal_current` — layout, lifecycle, logging, evidence.
- `zmux_pi_reload` after Pi extension/skill/prompt/theme changes; `zmux_reload` only for zmux config/key/theme changes; `zmux_pi_respawn` only as hard fallback.

If the Pi bash guard says to use a typed tool, do that instead of bypassing into
the CLI.

## Outside tmux / ambiguous session

Inside tmux, current-session targeting is implicit. Outside tmux, or whenever a
name could mean multiple sessions, list first and pass an explicit session target:

```bash
zmux ls -s
zmux tabs -s <session>
zmux run '<cmd>' -n scratch -s <session>
```

Never run processes directly just because you are outside tmux, and never guess
between attached sessions. `references/run-observe.md` covers `zmux where`, `-s`
forms, and common command examples.

## Agent peer

When asked for a review, second opinion, or agent-to-agent discussion from a real
CLI (`codex`, `claude`, `pi`, `agy`, etc.), drive that CLI in a visible zmux tab.
This skill owns terminal mechanics only; workflow skills decide when a peer is
needed. Read `references/agent-peer.md` before running the loop.

Minimal shape:

```bash
zmux run 'codex --dangerously-bypass-approvals-and-sandbox' -n codex-peer -d -s <session>
zmux watch codex-peer -s <session> --idle 3 -T 30
zmux type codex-peer '<prompt with repo/file pointers>' -s <session>
zmux watch codex-peer -s <session> --idle 3 -T 300
```

In Pi, use `zmux_run`, `zmux_runtime_logs`, `zmux_type`, and `zmux_tab_state` with
the `session` parameter.

## Agent worker

Workers are the peer loop with an inverted permission posture: a real CLI writes
and runs code inside an isolated git worktree. The worktree is the sandbox; zmux
owns terminal visibility and lifecycle. Fan-out policy belongs to the workflow
skill above this one. Read `references/agent-worker.md` before spawning workers.

Use `zmux session run`, not `zmux new`, for automated worker birth: it creates a
detached session with the CLI as the first tab, without focus steal or a blank
shell tab.

## Never

- `&`, `nohup`, `disown`, or harness-native hidden-background options — use `zmux run -d` / Pi `zmux_runtime_ensure`.
- unnamed tabs — always give reviewable work a stable purpose name.
- raw tmux for app-level actions.
- your own terminal sentinels, done markers, wrapper scripts, or `sleep && watch`; zmux/Pi tools own completion tracking.
- hand-rolled poll loops or `pgrep -f` / `pkill -f` self-matching patterns to await one job.
- guessing process state — read it with `zmux watch`, `zmux log`, or typed Pi log tools.
- `zmux refresh` / `zmux terminal refresh` from an agent session unless the user asked or disruption is acceptable.
- `zmux init` inside tmux — it intentionally refuses.
