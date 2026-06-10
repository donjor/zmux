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
- writing tab lifecycle state (`running`, `done`, `attention`, `clear`);
- moving the peer between full tab, pane, and hidden dock placements;
- reading passive CLI logs only when exact quotes are needed.

Do not add:

- SDK adapters;
- hook injection into the peer CLI;
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

Reuse first:

```bash
zmux tabs
zmux watch codex-peer
```

If the peer tab exists, read the screen before sending anything. If a human has
typed partial input, or the peer is generating, stop and wait.

Spawn detached with the max-permission profile:

```bash
zmux run 'codex --dangerously-bypass-approvals-and-sandbox' -n codex-peer -d
zmux watch codex-peer --idle 3 -T 30
```

Do not start peers in OS read-only/workspace-write sandbox modes. The prompt is the
read-only boundary; the CLI profile should be permissive enough that the peer can read,
search, and inspect without permission flakiness.

Outside tmux, or when targeting another session, pass `-s <session>` to each
relevant zmux call.

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

## Prompt

Prefer pointers over payloads. The peer has its own tools and cwd:

```text
Review docs/ROADMAP.md and the current git diff.
Return the 3 strongest points, 3 weakest points, and missing risks.
```

Avoid pasting long files unless the CLI cannot access them. The peer's
exploration should render in the visible tab.

Before every send:

```bash
zmux watch codex-peer
```

Then:

```bash
zmux type codex-peer '<prompt>'
```

`zmux type` sends text and Enter separately. Do not hand-glue Enter onto pasted
text with raw sends.

## Wait

```bash
zmux watch codex-peer --idle 3 -T 300
```

Exit 0 means the screen was quiet for 3 seconds. Stable does not mean done; it
means there is a screen to classify.

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

## Tab State

The driver owns lifecycle glyphs for long-lived peer tabs:

| transition | command |
| --- | --- |
| prompt sent | `zmux tab state running codex-peer` |
| capture classifies done | `zmux tab state done codex-peer` |
| permission prompt / question / human needed | `zmux tab state attention codex-peer --msg '<why>'` |
| answer consumed and parked | `zmux tab state clear codex-peer` |

Write state once per transition. Do not spam state writes on every capture.

`tab state` is **set-only, human-facing**: it drives the glyphs a person sees in the zmux
dashboard / status bar (◐ running · ✓ done · ● attention · ✗ failed). It is **not** a status
API you can read back — `zmux tabs` lists tabs + process, not glyphs (no `--json`, no get-form).
An agent reads another tab's progress by `watch`-ing it and classifying the screen (see
*Classify*), never by scraping `zmux tabs`.

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
how `watch`, `type`, `send`, or `tab state` target the peer.

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

After every large `type`, re-capture:

```bash
zmux watch codex-peer --idle 3 -T 30
```

If the prompt remains in the input box, send Enter:

```bash
zmux send codex-peer Enter
```

If the CLI says to queue while busy, use its queue affordance if known. For
Codex this has historically been `Tab`, but verify from the screen before using
it.

## Topic Changes

When the same peer tab persists across topics, reset context human-style instead
of respawning by default:

```bash
zmux type codex-peer '/new'
```

Pause briefly before the next prompt; session reset can race input.

Quit only when the peer session is genuinely done. The shell and tab survive a
CLI exit, so a future peer can be spawned in the same named tab.

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
