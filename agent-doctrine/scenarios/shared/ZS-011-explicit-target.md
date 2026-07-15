---
id: "ZS-011"
title: "Address one same-named target explicitly"
tier: "atomic"
doctrineRefs: ["ZD-002","ZD-003","ZD-008","ZD-010"]
applicability: ["claude","pi"]
uatEligible: false
---

## Prompt 1

Ask the `same-worker` peer in the `target` session for `TARGET_OK`. Do not use the one in `decoy`.

## Host setup

- Create target and decoy test sessions with same-named visible peers; record both contents, identities, rosters, and focus.

## Host perturbations

_None._

## Verdict

- outcome: Only the peer in target receives the request and returns TARGET_OK.
- orchestration: The agent pins the explicit session and never guesses from the same-named decoy.
- responsiveness: Focus remains unchanged and the host stays available.
- presentation: The answer clearly identifies the intended session without exposing raw addressing noise.
- cleanup: Both exact test sessions are removed and unrelated sessions remain untouched.

## Evidence

- Compare both peer inputs/outputs, session identities, focus, and final roster.

## Cleanup

- Remove the exact target and decoy test sessions only.

## Claude answer key

- Resolve and address same-worker through the explicitly named target session on every read and write.

## Pi answer key

- Pin the target session in the peer operation; verify the decoy pane remains unchanged.
