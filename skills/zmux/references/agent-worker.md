# Agent Worker Doctrine

Drive an official agent CLI (`codex`, `claude`, `pi`, `agy`, etc.) in a zmux tab as an
**autonomous worker bound to an isolated git worktree**. Unlike a peer (prompt-scoped
reviewer), a worker *writes and runs code* — that is the job. The whole exchange
stays in a real, named terminal a human can watch, attach to, take over, or kill.

This is generic zmux doctrine: terminal mechanics + the write-capable posture only.
It does **not** define which worktrees to spawn, how to decompose work, merge order,
or who runs browser tests — that policy lives in the workflow skill above. Higher-level
skills build on this; they must not duplicate the tab-driving mechanics.

## Relationship to agent-peer

The **driving loop is identical** to `agent-peer.md` — Spawn, Wait (`watch --idle`),
Classify, Tab State, Placement, Submission Hygiene, Topic Changes, Clean Quotes all
apply unchanged. **Read `agent-peer.md` for the loop.** This doc covers only what a
worker *inverts or adds*:

| dimension | peer (agent-peer.md) | worker (this doc) |
| --- | --- | --- |
| posture | prompt-scoped reviewer | write + exec is the job |
| cwd | the project (shared) | bound to one isolated worktree |
| session | usually shares yours | **own session per worker** when >1 concurrent |
| lifetime | persists, hidden between topics | terminal: spawn → work → done/blocked → reaped |
| writes | declined | routine **inside the worktree**; surfaced outside it |
| autonomy | answers, then waits | works long unattended; raises its hand on triggers |

## Worktree / session binding

A worker is bound to a pair: **(zmux session, worktree path)**. Honor both:

- **Own session per concurrent worker, spawned focus-safe.** Use
  `zmux session run <worker-session> -n <worker-tab> -- <cli launch>` — it creates the
  session **detached** in the current workspace (or `--workspace <ws>`) with the CLI as
  its **first tab**: one call, no stolen focus, no blank shell tab. It hard-errors if the
  session already exists, and never makes the worker the workspace's default attach
  target. Then carry `-s <worker-session>` on every later `run` / `watch` / `type` /
  `send` / `tabs` / `tab state` for that worker. A single worker may instead just live as
  a tab in the current session (`zmux run … -n <tab> -d`) — separate sessions matter once
  workers run *concurrently*, so their tabs and state don't collide.
    ```bash
    # current workspace (conductor inside tmux):
    zmux session run worker-auth -n auth-worker -- codex -C ~/proj.auth -s workspace-write -a on-request
    # named workspace (conductor outside tmux, or a dedicated workers ws):
    zmux session run worker-auth --workspace dev -n auth-worker -- codex -C ~/proj.auth -s workspace-write -a on-request
    ```
  **Never** spawn an automated worker with `zmux new <ws> <worker-session>`: `new`
  attaches (steals the conductor's focus) and births a blank shell tab beside the CLI —
  the report-009 failure this primitive exists to fix.
- **Reuse joined worker panes in an existing session.** When the worker/session already
  exists (or a single worker lives in the current session) and you are about to run a
  long-running visible command with `run -n <worker-tab>`, first inspect joined logical
  panes in that session:
    ```bash
    zmux pane list --joined --session --target <worker-session> --json
    ```
  If a row matches the worker purpose/worktree, route into its resolved `tabName`:
  `zmux run '<cli launch>' -n <tabName> -d -s <worker-session>`. Do not target the raw
  `paneID` for normal work; `run -n` keeps tab state, logs, placement, and lifecycle on
  the normal zmux resolver. This is spawn discipline for the existing roster/reaping
  model, not a separate worker task manager. For first birth via `zmux session run`,
  there is no worker session to search yet.
- **Spawn the CLI in the worktree.** Launch with the CLI's cwd set to the worktree
  (`codex -C <worktree>`, or `cd <worktree> && <cli>` inside the tab). The worker stays
  there for its whole life — it never wanders to the primary checkout or a sibling worktree.
- **The worktree is the sandbox boundary** (see Permission posture). Creating and
  removing the worktree itself is the caller's job, not the worker's.

CLI launch profiles are mechanics, not selection policy. The workflow above decides which CLI
to use; this doc says how to bind it once chosen:

| CLI | worker launch |
| --- | --- |
| Codex | `codex -C <worktree> -s workspace-write -a on-request` |
| Claude Code | `cd <worktree> && claude` |
| Pi | `cd <worktree> && pi` |
| Antigravity CLI | `cd <worktree> && agy` |

Use a stricter equivalent when the CLI supports one and it still allows normal in-worktree
writes. Do not use blanket bypass flags (`codex --dangerously-bypass-approvals-and-sandbox`,
`agy --dangerously-skip-permissions`)
as the default worker launch; see Permission posture and Worker vs blanket bypass below.

**Model / variant.** Set the worker's tier at spawn with the same `--model` flag per CLI as
`agent-peer.md` → *Model / variant selection* (`claude --model …`, `codex -m …`, `agy --model …`,
`pi --model provider/id:thinking`). *Which* tier per workstream is selection policy
(`orchestrate` → Worker model tier). To bump a worker, **respawn** at the higher `--model`: the
worktree is the durable state, so a worker is cheap to relaunch and re-brief — prefer that over
the interactive `/model` fallback, which is for keeping a loaded peer tab's context.

## Permission posture — scoped write + exec

Invert peer's "decline writes." For a worker, **writes and command execution inside the
bound worktree are routine — approve them.** The boundary is the worktree directory, not
a file list. Still **surface (do not auto-approve)**:

- writes or commands that touch paths **outside** the bound worktree;
- network fetches, package installs, self-updates;
- auth changes, credential entry (never automate passwords);
- spawning further sub-agents or daemons;
- anything destructive beyond the worktree (force-push, global git config, `rm` outside it).

Pick the **most-scoped write mode the CLI offers**, with the worktree as the sandbox —
not a blanket bypass. Visible terminal state gives auditability, not prevention: a prompt
or repo file can still induce an out-of-scope action, so the surfacing rule is the real guard.

### Sandbox reality (same root cause as old sandboxed peer launches)

Scoped-write enforcement uses the CLI's OS sandbox. Peers no longer use that sandbox by
default (`agent-peer.md` → *Permission reality*), but workers still can because their
worktree boundary is load-bearing. Where the sandbox can't start (bubblewrap / no user
namespaces — containers, CI), `workspace-write` doesn't fail closed like old peer read-only —
instead, with
`-a on-request`, the CLI **surfaces every blocked write as a scoped approval** ("sandbox
could not start the command; allow `git -C <worktree>`?"). Dogfooded: one file took three
approvals (status, write, commit), each correctly scoped to the worktree path.

That surfacing *is* the boundary when the sandbox is gone — good for a short task, but a
worker is meant to run **long and unattended**, and per-write prompts break that. So when
the sandbox can't start, prefer **`-s danger-full-access -a on-request`**: in-worktree
writes run free, while out-of-worktree / network / install / auth still surface (per the
posture above). The worktree boundary then rests on the surfacing rule + the visible tab,
not the OS. Default to the strictest mode that still lets the worker work unattended.

### Worker vs blanket bypass

`codex --dangerously-bypass-approvals-and-sandbox`, `agy --dangerously-skip-permissions`, or any
blanket "approve everything" flag is **not** worker mode. Three differences:

1. **Scoped, not blanket** — the worktree bounds writes; out-of-worktree + network/install/
   auth still surface.
2. **Visible + addressable** — runs in a named session with lifecycle glyphs; a human can
   `zmux watch` / attach / take over / kill it. A yolo run in a raw shell is none of those.
3. **Surfacing discipline** — a worker raises its hand on consequential/ambiguous actions
   instead of barreling through.

Reserve blanket bypass for an explicit user acceptance of the risk on a bounded, disposable
worktree; it is not the worker default.

## Autonomy — surface vs proceed

A worker runs **long, unattended**. It must know when to stop and raise its hand vs push on.

**Surface** (set `zmux tab state attention <tab> --msg '<why>'`, then wait) on:

- ambiguity in the brief it can't resolve from the worktree;
- a permission prompt for a consequential action (per posture above);
- the **same step failing repeatedly** (don't loop forever);
- needing a forbidden resource (network/install/out-of-worktree write).

**Proceed** on everything else within the brief and the worktree — commit progress freely
(small, frequent commits are the recovery seam if the session dies; the worktree's git
history survives a lost tab). Mark `zmux tab state done <tab>` when the brief is complete,
`failed` if it gave up. Note: a worker that is *confidently wrong* will report `done`, not
`attention` — `done` means "I think I finished," not "this is correct." Verification is the
caller's job, never trust a worker's own `done` as proof.

## Names

| worker | tab name |
| --- | --- |
| task-scoped | `<task>-worker` (e.g. `auth-worker`, `migrate-worker`) |
| by CLI when generic | `codex-worker`, `claude-worker`, `pi-worker`, `agy-worker` |

Session names follow the same scope (`worker-<task>`). Keep names stable and descriptive —
they say which worktree/task the worker serves.

## Lifecycle / teardown

Workers are **terminal**, unlike peers (which you hide to keep context warm):

- spawn → `running` → `done`/`failed`/`attention` (glyphs per `agent-peer.md` → Tab State).
- Once the work has been integrated and the worktree is gone, the worker's context has no
  further value — its session is **reapable**: `zmux session kill <worker-session>`.
- Removing the **worktree** is the caller's concern (e.g. a `wt merge`), not the worker's;
  the worker only owns its tab/session lifecycle.

## Handoff to workflow skills

A workflow skill layered above zmux decides: which worktrees to create, how to brief each
worker, how many run concurrently, merge order, who runs browser/integration tests, and how
to verify a worker's output before trusting its `done`. That skill calls this doctrine for the
terminal loop and refers to workers by the stable names above. None of that fan-out policy
belongs here.
