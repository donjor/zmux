# Session-state priming: no pre-action snapshot (harden on evidence)

## Context

A recurring want: a fresh agent should have an up-to-date view of its zmux
session *before* it creates tabs/panes or fires a long-running command, so it
reuses existing tabs instead of spawning duplicates or colliding on names.

What already exists, in three layers:

- **SessionStart hook** (`skills/zmux/hooks/zmux-context.mjs`) — primes the agent
  at session start, and re-injects on resume/compact. It lists the session, cwd,
  and current `zmux tabs`. This is a **point-in-time snapshot**: captured at
  those boundaries, it goes stale between them.
- **PreToolUse:Bash guard** (`skills/zmux/hooks/zmux-guard.mjs`) — reactive. It
  blocks raw tmux / background jobs / dev servers and suggests the zmux verb. It
  injects **no** live state; it judges the command string only.
- **Binary resolution** — `zmux run -n <name>` resolves the name against the
  **live** session at exec time. Re-firing `run -n <name>` reuses the existing
  tab, and the create path is session-scoped (report 016, commit `7c18394`), so a
  same-named sibling in another session no longer blocks or misdirects the spawn.

The gap considered: there is no *just-in-time* refresh of the agent's view right
before a zmux mutation. The agent re-grounds only voluntarily (`zmux tabs`,
`zmux watch`). The proposal was a pre-action snapshot — e.g. have the guard hook
append a fresh `zmux tabs` when the command is a tab-creating verb.

## Decision

**Do not build pre-action snapshot injection now.** The determinism that matters
already lives in the binary (live name resolution + session-scoped create), and
no observed failure was actually caused by a stale snapshot.

The three reports that *resemble* this problem were something else:

- **016** — a real **binary bug** (cross-session ambiguous refusal). Fixed in code
  (`7c18394`); a snapshot would not have helped, the spawn was genuinely refused.
- **014** / **017** — the agent **already had** the tab in its primed context and
  ignored the reuse doctrine (tab-per-command; suffix-bumping `x`→`x2`). Fixed by
  hook/SKILL wording, not by more state.

A per-Bash snapshot hook would run on every command and depend on an injection
channel, to harden a failure mode with zero observed instances. That is
speculative complexity; the smallest durable change is none.

## Consequences

- Until evidence says otherwise, the agent re-grounds on demand with `zmux tabs` /
  `zmux watch`. `run -n` reuse and session-scoped create stay deterministic at
  exec time, which covers the cases that have actually bitten.
- **Revisit trigger:** a real report where the agent acts on a tab that
  **changed since session-start priming** — created, killed, or renamed
  mid-session — *not* merely ignoring data it already had. Capture it via
  `jotsmith`; that evidence picks the right mechanism (pre-action snapshot,
  auto-scoping reads to the session, or something else) instead of a guessed one.
- This record exists so a future agent does not re-derive the proposal from
  scratch and re-litigate a call that was made deliberately.
