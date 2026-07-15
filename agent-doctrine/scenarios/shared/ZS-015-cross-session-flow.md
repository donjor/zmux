---
id: "ZS-015"
title: "Continue work in one explicit session"
tier: "workflow"
doctrineRefs: ["ZD-002","ZD-003","ZD-008","ZD-010"]
applicability: ["claude","pi"]
uatEligible: false
---

## Prompt 1

Ask `same-worker` in the `target` session to answer `FIRST_TARGET`.

## Prompt 2

Ask that same target-session worker to answer `SECOND_TARGET`. Leave the one in `decoy` alone.

## Host setup

- Create target and decoy test sessions with same-named visible workers and record both identities, contents, rosters, and focus.

## Host perturbations

_None._

## Verdict

- outcome: Both replies come from the same worker in target and the decoy receives nothing.
- orchestration: Every turn pins the explicit session and reuses the intended worker without ambiguous same-name fallback.
- responsiveness: The host remains available and focus unchanged across turns.
- presentation: Replies are ordered and clearly associated with target rather than raw session identifiers.
- cleanup: Both exact test sessions are removed; unrelated sessions and focus remain unchanged.

## Evidence

- Inspect both worker inputs/outputs after each turn, identities, focus, and final session roster.

## Cleanup

- Remove the exact target and decoy test sessions only.

## Claude answer key

- Pin the target session on every address and prove decoy contents remain unchanged.

## Pi answer key

- Supply the target session on every peer operation and reuse the same target identity for the follow-up.
