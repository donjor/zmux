# Guard & tab states (the zmux hooks layer)

Shared outcomes live in `shared-doctrine.generated.md`; this reference owns Claude hook, guard, roster, and lifecycle mechanics.

The harness-specific guardrails that enforce this skill's hygiene. Claude uses
`skills/zmux/hooks/` (`PreToolUse` guard + tab-state hooks); Pi uses the repo's
the canonical `pi-zmux/` dispatcher and bash guard. `SKILL.md` carries the invariants;
this is the detail.

## Raw tmux → zmux verb mapping

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

## Tab roster & the reviewability test

zmux tabs are a **shared, reviewable surface** for you and the user — not scratch space that
multiplies. Two rules keep them useful.

**Reviewability, not duration.** A tab earns its place when a human would want to *see, grab, or
re-run* the command — it mutates/runs the project, needs manual input/control or interruption, or
should be Up-arrow re-runnable. Bounded checks whose captured stdout is the whole artifact stay in
your own shell, even slow ones (`go test`, a long build). A short DB migration belongs in a tab; a
long read does not.

**Reuse a tiny roster — by purpose, never one tab per task.** `zmux run -n <name>` reuses a tab that
already exists, so addressing by a stable purpose-name keeps related work together:

| tab | purpose |
| --- | --- |
| `claude` / `codex` / `pi` / `agy` | the session's primary agent shell — long-lived, not a task tab |
| `dev` | the project runtime: app server, local service, main REPL, the process a human stops/restarts |
| `scratch` | reviewable one-offs: mutations, manual takeover, things to inspect/re-run, no durable name |
| `admin` / `remote-<host>` | SSH, sudo, remote shells, and remote-config retries — one stable tab per host |
| `<agent>-peer` | a review peer with a semantic `tab peer` lifecycle — owned by the peer skill |
| `worker-*` | orchestrated worker *sessions* (not conductor roster tabs) |

Do **not** mint `eval-2`, `test-run`, `build-x`, numbered remote/admin tabs,
per-Playwright-lane, or feature-named tabs.
That scatters the surface and is the exact sprawl this rule exists to stop.

Route by purpose:

- reviewable long/odd command → `scratch`;
- SSH/remote-admin retries → `admin` or one `remote-<host>` tab;
- main runtime → `dev`;
- bounded check → your shell.

App-managed detached daemons (their own `setsid`/systemd/Docker `-d`) aren't tabs
at all — don't babysit an empty wrapper.

Remote admin has an extra audit rule: if a quoting workaround uses an opaque
encoded or obfuscated payload, decode/explain it and say “about to change X on
host Y” before running it. An encoded blob in tab history is not reviewable
enough, especially for `.env`, credential, service, or deployment mutations.

Headed/browser-visible Playwright is the special bounded-looking case: if it opens real browser UI,
uses CDB/headed placement, or produces visual proof the user may need to watch, route it through one
reusable proof tab (`scratch` or `pw-scratch`). Serial lanes reuse that tab; do not create
`test-2d-surface`, `test-2d-quality`, `test-2d-all`, etc. unless the user explicitly wants separate
supervised lanes. Before launching an aggregate suite, inspect the existing proof tab/output so you
do not duplicate coverage that has already passed.

Pairs with **tab hygiene** in `SKILL.md`: spawn into the roster, reuse by purpose, tear down after.
If a prompt creates several tabs/panes, take a cheap before/after roster (`zmux tabs`, `zmux pane list --joined --session` when relevant), then kill ad-hoc tabs as soon as evidence is captured. Keep only intentional runtimes/peers/worker sessions with a named next checkpoint.

Assume focus may move unless the command/tool explicitly says otherwise. Agent paths prefer `run -d`, `session run`, `pane open --no-focus`, placement without `--focus`, and Pi dispatcher operations whose default is focus-safe. Ask before `tab focus`, `pane focus`, or any `focus: true` option.

## The guard hook

Claude's `hooks/zmux-guard.mjs` (symlinked into `~/.claude/hooks/`) **blocks** raw
tmux calls and prints the mapping back to you — so a slip self-corrects instead of
silently targeting the wrong window. Pi's `pi-zmux/` enforces the same doctrine
through one `zmux` dispatcher (`run`, `runtime_ensure`, `interactive_type`,
`peer_ensure`, `tab_inspect`, and related operations) plus a `bash` tool-call guard. Both guard
surfaces enforce the rest of this skill's
hygiene: a dev server / background job (`npm run dev`, `&`, `nohup`, or any
harness-native hidden-background option the adapter can see) is **blocked** toward a
visible named zmux runtime, and an interactive/remote command (`sudo`, `ssh`, a REPL)
is nudged into a shared tab. The Claude-only `run_in_background` parameter check lives
in `zmux-guard.mjs` because it is a tool param, not a shell token.

**Exemptions** — genuinely need the raw command (zmux development, socket inspection,
a one-off)? Any of: prefix `ZMUX_ALLOW=1`, append `# zmux: allow`, use an explicit
`-L <socket>`, or run from the zmux repo. Pi has its own one-shot bypass spelling
(`PI_ZMUX_ALLOW=1` / `# pi-zmux: allow`) for its bash guard; prefer the dispatcher
redirect when one exists.

## Tab lifecycle states

`zmux tab state <attention|failed|running|ready|done|clear> [tab]` marks a tab's lifecycle;
the bar renders a colored glyph (● needs-human / ✗ failed / ◐ running / ↩ ready / ✓ done)
visible from any tab. `zmux tab status <tab> --json` is the read side: it reports
human glyph state plus command lifecycle and peer turn metadata for agents/tools.

Mostly automatic:

- In root interactive shells with the `zmux setup shell` block installed, normal foreground commands set running → done/failed through shell lifecycle hooks (`shell-event start/end`) even inside peer/worker/agent tabs. Known persistent venue commands (`pi`, `claude`, `codex`, REPLs/TUIs) and daemon-scoped launches do not get a stuck running glyph just because the process is alive.
- `zmux run` only stages a silent run id for callers waiting on an exit code; it does not append visible sentinels or own glyph state.
- `zmux send`/`type` clear a stale ready/done/failed before sending input.
- Focusing a tab clears attention.
- A `Stop` hook (`hooks/zmux-tab-state-stop.mjs`) can mark the agent's own tab ready
  (`↩`) when a turn ends — no transcript parsing, just "the turn ended". It ships in the
  skill but, unlike the guard/context hooks, is not symlinked into the live install by
  default; register it as a `Stop` hook to enable it.
- Prompt-scoped peers use `zmux tab peer <start|running|ready|attention|failed|consumed|park|keep|clear-keep>` for semantic turn/retention metadata; legacy `waiting` aliases to `ready`. Answer-ready renders `↩`, while true human intervention uses attention (`●`). Agents read this with `tab status` / Pi dispatcher `tab_status`; Pi can bundle status+output with `tab_inspect`. Screen capture is the fallback/output layer.

Set `attention` **manually** when handing the human a prompt they must act on (sudo,
permission prompt). For peer tabs, prefer `zmux tab peer attention ...` so the turn state stays in sync:

```bash
zmux tab state attention admin --msg 'sudo password'
zmux tab peer attention claude-peer --msg 'permission prompt'
```
