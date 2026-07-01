# Agent Peer Doctrine

Drive an official agent CLI (`codex`, `claude`, `pi`, `agy`, etc.) in a zmux tab as a
visible peer. Type prompts, wait for the screen to settle, read the answer, and
reply if needed. The whole exchange stays in a real terminal the user can watch
and take over.

This is generic zmux doctrine. It covers terminal mechanics and etiquette only.
It does not define when a personal workflow should ask for a peer, what review
ritual to use, which model is preferred, or how to manage long-running review
programs. Higher-level workflow skills may build on this; they must not
duplicate the tab-driving mechanics.

## Boundary

Use zmux for:

- spawning or reusing a real CLI in a named tab;
- typing prompts and commands;
- waiting for quiet screens with `watch --idle`;
- classifying what the visible screen shows;
- recording semantic peer lifecycle (`start`, `running`, `waiting`, `consumed`, `park`, timestamped `keep`);
- writing human-visible glyph state only as the peer lifecycle helper's presentation layer;
- moving the peer between full tab, pane, and hidden dock placements;
- reading passive CLI logs only when exact quotes are needed.

Do not add:

- SDK adapters;
- ad-hoc hook injection into the peer CLI (use official Stop/hook surfaces when the CLI already exposes them);
- config edits to the peer CLI;
- per-CLI output parsers;
- an orchestrator or manager above zmux;
- personal review doctrine.

The driving agent is the adapter. zmux provides dumb terminal primitives.

## Names

Use stable descriptive tab names:

| peer | tab name |
| --- | --- |
| Codex | `codex-peer` |
| Claude Code | `claude-peer` |
| Pi | `pi-peer` |
| Antigravity CLI | `agy-peer` |
| Task-specific | `<topic>-peer` |

Avoid legacy workflow names. The tab name should say which peer is running or
which task it is serving.

## Spawn

**Tab names are per-session unique — reuse the roster name across sessions.**
A `codex-peer` already live in another session is not your concern; never
invent a globally-unique name to dodge a "collision." Spawns and writes are
session-scoped (reports 039 / 016): a bare `zmux run -n <peer> -d` creates in
*your* session and can neither land on — nor be **blocked by** — another
session's tab, even the same roster name live in several siblings (report 016
scoped the create-path resolve to the session, so a multi-session box no longer
refuses the spawn with `ambiguous`). `send`/`type`/`kill` refuse to cross too —
an out-of-session name surfaces a clean in-session miss instead of acting on a
sibling's pane.

**Pin the current session on reads.** The read path still resolves a unique
name server-wide, so a bare `watch <peer>` (or `log tail` / `tab show`) with no
local match reads a *sibling* session's peer (a real failure: a `claude-peer`
review read against the wrong repo). Resolve where you are and pass `-s` so you
read your peer, not someone else's:

```bash
zmux pane current --json   # "Session" → the session you are in
zmux ls -s                 # how many sessions exist
```

Pin that session on the spawn and every follow-up — belt-and-suspenders for
the writes, load-bearing for the reads:
`zmux run '…' -n <peer> -d -s <session> --scope peer`, `zmux watch <peer> -s <session>`,
`zmux type <peer> -s <session> …`, `zmux tab peer … <peer> -s <session>`. In Pi,
use the equivalent `session` parameter on `zmux_run`, `zmux_runtime_logs`,
`zmux_type`, `zmux_tab_peer`, and `zmux_tab_state`. zmux prints `tab "<peer>" resolved to session
"X", outside the current session "Y"` on the read path when a bare name crosses —
seeing that means you skipped the pin.

Reuse first — but verify identity:

```bash
zmux tabs <session>
zmux pane list --joined --session --target <session> --json
zmux watch <peer> -s <session>
```

If the peer tab exists, confirm it is in *this* session and on the right
cwd/topic before sending anything; a same-named tab elsewhere is not your peer.
If a human has typed partial input, or the peer is generating, stop and wait. If
a peer already landed in the wrong session, recover with
`zmux tab move <peer> <session>` (add `-f` to pull it across workspaces) or kill
it and respawn pinned.

Before minting a fresh peer tab for long-running visible work, inspect the joined
pane list above. If it already contains the peer you need, route through that row's
`tabName` with the normal resolver:

```bash
zmux run 'codex --dangerously-bypass-approvals-and-sandbox' -n <tabName> -d -s <session> --scope peer
zmux tab peer start <tabName> -s <session> --role codex --topic '<sanitized topic>'
zmux watch <tabName> -s <session> --idle 3 -T 30
```

The raw `paneID` is diagnostic; do not target it for the peer loop. `run -n
<tabName>` preserves zmux state, logging, placement, and lifecycle behavior for
the joined tab. This does not create a new roster category or bypass tab reaping;
it is the same roster reuse check before creating another visible peer tab.

Spawn detached with the max-permission profile:

```bash
zmux run 'codex --dangerously-bypass-approvals-and-sandbox' -n codex-peer -d -s <session> --scope peer
zmux tab peer start codex-peer -s <session> --role codex --topic '<sanitized topic>'
zmux watch codex-peer -s <session> --idle 3 -T 30
```

Do not start peers in OS read-only/workspace-write sandbox modes. The prompt is the
read-only boundary; the CLI profile should be permissive enough that the peer can read,
search, and inspect without permission flakiness.

Startup interstitials are common. Self-updates, extension installs, auth
changes, and network installs are consequential; decline or ask the user.

## Launch Profiles

| profile | when |
| --- | --- |
| `codex --dangerously-bypass-approvals-and-sandbox` | default Codex peer profile |
| `claude --dangerously-skip-permissions` | default Claude Code peer profile |
| `agy --dangerously-skip-permissions` | default Antigravity CLI peer profile |
| `pi` | Pi exposes all core tools by default; use its closest full-tool / auto-approve profile if one is installed |

Peers are launched with write-capable permissions by default. A prompt or repo file can still
induce writes, so visible terminal state gives auditability, not prevention. The guard is the
review prompt plus the watched tab, not a sandbox.

### Permission reality — max profile, prompt-scoped behavior

Sandboxed peer launches are too flaky: bubblewrap/user-namespace failures, different CLI
semantics, and read failures waste the review loop. Do not try to make a review peer safe by
removing its tools. Tell it what role it is playing.

For a read-only review, put the boundary in the prompt:

```text
Stay read-only. Inspect files and the git diff. Do not edit files, install packages,
delete files, commit, push, or spawn sub-agents. Return findings only.
```

If a peer violates that boundary, treat the output as suspect, revert/ignore the write, and
restart or reset the peer. Do not respond by downgrading future peers to sandboxed profiles.

### Model / variant selection

*Which* tier a role should run at is selection policy (global **Model tiering** + the `peer` /
`orchestrate` skills). This is the mechanics: how to pin a model/variant on each CLI. Every CLI
takes a launch-time flag, so spawn directly at the wanted tier — no interactive step needed:

| CLI | spawn at tier | variant axis |
| --- | --- | --- |
| Claude Code | `claude --model opus\|sonnet\|haiku\|fable` (alias or full id) | `/fast` toggles fast-mode in-session |
| Codex | `codex -m <model>` (or `-c model="…"`) | reasoning set via model id / config |
| Antigravity | `agy --model <model>` (`agy models` lists) | — |
| Pi | `pi --model <provider/id:thinking>` | the `:<thinking>` suffix **is** the effort/reasoning variant |

Tier = model name **+** effort (reasoning/thinking level, fast-mode) — treat them as one knob.

**Mid-session bump (escalation).** When a cheap peer reviews shallowly and you want to raise its
tier: prefer a **clean respawn** at the higher `--model` (a review peer carries little durable
context, so respawn is cheap and unambiguous). Only when the tab's loaded context is worth keeping,
drive the CLI's in-session switcher instead — `zmux type <tab> '/model'` then `zmux send <tab>`
arrow keys + `Enter` (Claude `/model` menu; Pi `Ctrl+P` cycles `--models`). The `/model`+arrows
path is the fallback for CLIs without a non-interactive in-session switch.

## Prompt

Prefer pointers over payloads. The peer has its own tools and cwd:

```text
Review docs/ROADMAP.md and the current git diff.
Return the 3 strongest points, 3 weakest points, and missing risks.
```

Avoid pasting long files unless the CLI cannot access them. The peer's
exploration should render in the visible tab.

Before every send, read the visible composer:

```bash
zmux watch codex-peer -s <session>
```

Then submit through `type`:

```bash
zmux type codex-peer '<prompt>' -s <session>
```

`zmux type` delivers text and a submit key, but terminal delivery is not proof the
peer CLI accepted the prompt. For large multi-line pastes, verify submission in
*Submission Hygiene* before any long wait. Do not immediately add a separate raw
`Enter`; use an extra `Enter` only after a recapture proves the prompt is still in
the composer.

## Wait

```bash
zmux watch codex-peer -s <session> --idle 3 -T 300
```

Exit 0 means the screen was quiet for 3 seconds. Stable does not mean done; it
means there is a screen to classify.

Prefer `--idle` + classify for peer turns. If you use `watch --until`, the regex
must match future peer output, not a word in your own echoed prompt. `VERDICT`
self-matches if your prompt says "Give VERDICT"; use a discriminating answer
shape such as `VERDICT: (APPROVE|REVISE)` only when that exact text is absent from
the prompt echo.

Non-zero means timeout, interrupt, or command error. If a capture printed, use
it. If the screen is still active, wait again with a larger ceiling.

Use rough ceilings:

| ask | ceiling |
| --- | --- |
| quick question | 120s |
| review / critique | 300s |
| large plan or diff | 600s |

For long waits, run the watch as an async task and keep working. Treat task
completion as "beat ready", not "turn done"; classification decides the next
action.

## Classify

| capture shows | state | action |
| --- | --- | --- |
| answer plus fresh empty input box | done | read, synthesize, or quote |
| numbered options / approval prompt | permission prompt | apply permission policy |
| free-form question to the driver | asking you | answer like a colleague |
| submitted prompt with no answer/input | still working | wait again |
| partial input you did not send | human typing | hands off |
| startup/update/auth screen | interstitial | decline consequential actions or ask user |

You judge from the screen. Do not ask zmux for CLI-specific done detection.

## Peer lifecycle state

Use the semantic peer lifecycle surface; it writes the policy metadata and the human-facing glyph where appropriate:

| transition | command |
| --- | --- |
| peer spawned/reused | `zmux tab peer start codex-peer --role codex --topic '<sanitized topic>'` |
| prompt accepted / turn running | `zmux tab peer running codex-peer` |
| peer Stop/hook says turn ended | `zmux tab peer waiting codex-peer --source codex-stop` |
| permission prompt / question / human needed | `zmux tab peer attention codex-peer --msg '<why>'` |
| answer consumed | `zmux tab peer consumed codex-peer` |
| answer consumed but keep tab inspectable briefly | `zmux tab peer park codex-peer --ttl 30m` |
| explicit next checkpoint keep | `zmux tab peer keep codex-peer --ttl 2h` |

Write lifecycle once per transition. Do not spam writes on every capture. Prompt-scoped peers should **not** get `@zmux_keep=1` by default; use `park`/`keep --ttl` so the reaper can clean expired peers.

`tab state` remains **set-only, human-facing**: it drives glyphs a person sees in the zmux dashboard/status bar. It is not a full status API. The peer lifecycle metadata is the machine-readable policy layer; screen classification is still the fallback for CLIs without a usable Stop/hook signal.

## Placement

Peer tabs are normal logical zmux tabs. They can run full-screen, beside work as
a pane, or hidden in the dock:

```bash
zmux tab pane codex-peer
zmux tab pane codex-peer --into work --down --size 30%
zmux tab full codex-peer
zmux tab hide codex-peer
zmux tab show codex-peer
```

Hide instead of quitting when context should persist. Placement does not change
how `watch`, `type`, `send`, or `tab peer` target the peer.

## Permission Policy

Peers should normally run in a profile that does not stop for tool approvals. If the CLI still
surfaces a prompt, apply the prompt contract:

- allow routine read/search/diff work inside the workspace;
- decline installs, self-updates, auth changes, network fetches unrelated to the review,
  commands outside the workspace, daemon/sub-agent spawning, and any write outside a
  workflow-sanctioned peer artifact.

For out-of-scope prompts, decline and steer:

```bash
zmux send codex-peer Escape
zmux type codex-peer 'No. Stay read-only and review the existing files instead.'
```

Never automate password entry.

## Submission Hygiene

Treat submission as a verified step. `typed to %pane` / `sent to %pane` means the
keystrokes reached tmux; it does **not** mean the peer CLI submitted the prompt.
Large bracketed pastes can race the follow-up submit key and leave the prompt
sitting in the composer.

After every large or multi-line `type`, re-capture before waiting on the answer:

```bash
zmux watch codex-peer -s <session> --idle 1 -T 30
```

Classify the fresh screen:

| capture shows | submit state | action |
| --- | --- | --- |
| composer cleared, fresh `Working`/spinner, or new assistant text after the prompt | submitted | now wait/classify normally |
| full prompt still sitting in the composer | not submitted | send one `Enter`, then recapture |
| only old scrollback `Working`/assistant text from a prior turn | unknown | recapture or inspect live tab; do not count as submitted |
| human partial input | hands off | stop and wait/ask |

Retry only after the recapture proves the prompt is still in the input box:

```bash
zmux send codex-peer -s <session> Enter
zmux watch codex-peer -s <session> --idle 1 -T 30
```

Do not start `watch --until` or mark the peer `running` until submission is
verified. If the CLI says to queue while busy, use its queue affordance if known;
for Codex this has historically been `Tab`, but verify from the screen before
using it.

## Topic Changes

When the same peer tab persists across topics, reset context human-style instead
of respawning by default:

```bash
zmux type codex-peer '/new'
```

Pause briefly before the next prompt; session reset can race input.

Quit only when the peer session is genuinely done. After the answer is consumed, prefer `zmux tab peer park <peer> --ttl 30m`; use `keep --ttl` only for a named next checkpoint. The shell and tab may survive a CLI exit, so a future peer can be spawned in the same named tab until the parked tab expires and reaps.

## Clean Quotes

Screen capture is the working medium. For exact quotes from long answers, use
the CLI's passive session logs. Do not use logs to drive the loop.

- Codex: scrape the session id from the screen, then read the matching
  `~/.codex/sessions/**` JSONL.
- Pi: inspect `~/.pi/agent/sessions/<cwd-slug>/`.

Keep quoted excerpts short and synthesize the rest.

## Handoff To Workflow Skills

A workflow skill layered above zmux may decide:

- when a peer should be used;
- which peer CLI/profile to pick;
- what prompt template to send;
- how many review rounds are enough;
- what artifacts to write;
- how to summarize the outcome to the user.

That workflow should call this doctrine for the terminal loop and should refer
to peer tabs by the stable names above.
