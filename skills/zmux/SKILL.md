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
| Testing the shipped agent/Pi surface while developing zmux | repo-root `docs/dev/test-prompts/` |

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

Use **zmux** for work that needs a shared terminal surface:

- software that keeps running: servers, watchers, queues, REPLs;
- commands a human may want to see, grab, interrupt, or re-run;
- input/passwords/sudo/manual control;
- existing long-running process inspection, sidecars, and terminal diagnostics;
- visual/TUI evidence.

SSH/remote-admin retry loops use one stable visible tab:

- Use `admin` or `remote-<host>`.
- Never suffix-bump one host into `remote-<host>2`, `remote-<host>3`, etc.
- If quoting forces an opaque encoded/admin payload, decode/explain it first.
- State the intended host/config mutation before running it.

Headed/browser-visible Playwright or Chrome proof counts as visual evidence even
when the command exits. Run those batches through one reusable scratch/proof tab,
not direct bash and not one tab per lane.

### Reuse a tiny roster

Address tabs by purpose. `zmux run -n <name>` reuses an existing tab, so related
work stays together:

- `dev` — project runtime: server, service, main process, REPL.
- `scratch` — reviewable one-offs, manual takeover, things to inspect/re-run.
- `<agent>-peer` — a review peer with semantic `tab peer` lifecycle; `worker-*` — orchestrated worker sessions.
- `claude` / `codex` / `pi` / `agy` — long-lived primary agent shells, not task tabs.

Do **not** mint `eval-2`, `test-run`, `build-x`, per-Playwright-lane,
numbered remote/admin tabs, or feature-named tabs. Bounded checks stay in your shell; odd
reviewable commands and serial browser-proof batches go to `scratch`/`pw-scratch`;
remote/manual admin goes to `admin` or one `remote-<host>` tab; the main runtime
goes to `dev`. Re-running the same job means re-fire the same `run -n <name>`
target, never suffix-bump `x` → `x2` → `x3`.

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

Kill task-scoped peer, worker, or ad-hoc `run` tabs once their evidence is
captured or their work is integrated — not at some vague session closeout. If
you create several tabs/panes, compare the roster before/after and remove what
you created. The reaper backstops forgotten agent-task tabs, but it is
housekeeping, not a substitute for cleanup. Protect intentional long-lived tabs
with `run --keep` or `run --scope daemon`.

Do not wrap app-detached daemons in a tab. If the app backgrounds itself and
survives independently (systemd, Docker `-d`, its own `setsid`), a zmux wrapper
tab is pure overhead.

## Pi dispatcher

Pi exposes one compact `zmux_lite` tool. Select its `operation` instead of
shelling out:

- inspect/control: `current`, `tabs`, `sessions`, `panes`, `run`, `session_run`, `session_kill`;
- persistent work: `runtime_ensure`, `runtime_logs`, `runtime_stop`;
- peers/tabs: `peer_ensure`, `peer_handoff`, `type_text`, `tab_inspect`, `tab_status`, `tab_state`, `tab_peer`, `tab_place`, `tab_kill`;
- panes/input: `pane_open`, `pane_resize`, `pane_close`, `interactive_type`;
- waits/evidence: `wait`, `callback_watch`, `log`, `snapshot`, `terminal_current`;
- lifecycle: `pi_reload` after Pi extension/skill/prompt/theme changes, `zmux_reload` only for zmux config/key/theme changes, and `pi_respawn` only as a hard fallback.

Keep focus false unless the user explicitly requests focus. If the Pi bash guard
redirects to a dispatcher operation, use it instead of bypassing into the CLI.

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

Minimal status-first shape:

```bash
zmux tab peer ensure codex-peer -s <session> --command 'codex --dangerously-bypass-approvals-and-sandbox' --role codex --topic '<sanitized topic>' --json
zmux type codex-peer '<prompt with repo/file pointers>' -s <session> --mark-peer-running --wait-turn ready --json
zmux tab inspect codex-peer -s <session> --json  # status + output after state says ready, or fallback evidence
```

In Pi, use `zmux_lite` operations `peer_ensure` for spawn/reuse, `type_text` with `options.markPeerRunning`/`waitForTurnState` for peer prompts, and `tab_inspect` for status+output diagnosis. Pass `options.session` on every peer call.

## Agent worker

Workers are the peer loop with an inverted permission posture: a real CLI writes
and runs code inside an isolated git worktree. The worktree is the sandbox; zmux
owns terminal visibility and lifecycle. Fan-out policy belongs to the workflow
skill above this one. Read `references/agent-worker.md` before spawning workers.

Use `zmux session run`, not `zmux new`, for automated worker birth: it creates a
detached session with the CLI as the first tab, without focus steal or a blank
shell tab.

## Never

- `&`, `nohup`, `disown`, or harness-native hidden-background options — use `zmux run -d` / Pi `zmux_lite operation=runtime_ensure`.
- unnamed tabs — always give reviewable work a stable purpose name.
- raw tmux for app-level actions.
- your own terminal sentinels, done markers, wrapper scripts, or `sleep && watch`; zmux-managed shells own command lifecycle glyphs and `zmux wait` owns structured condition waits.
- hand-rolled poll loops or `pgrep -f` / `pkill -f` self-matching patterns to await one job.
- guessing process state — read lifecycle/command/peer state with `zmux tab status` / Pi `zmux_lite operation=tab_status`, or use `zmux wait` for fresh state/output/idle conditions; use `zmux watch`, `zmux log`, or dispatcher log operations for output only.
- `zmux refresh` / `zmux terminal refresh` from an agent session unless the user asked or disruption is acceptable.
- `zmux init` inside tmux — it intentionally refuses.
