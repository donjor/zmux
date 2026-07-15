---
id: "ZS-016"
title: "Complete sequential work without accumulation"
tier: "workflow"
doctrineRefs: ["ZD-001","ZD-003","ZD-008","ZD-009","ZD-010"]
applicability: ["claude","pi"]
uatEligible: false
---

## Prompt 1

Run `printf 'FIRST_DONE\n'` somewhere visible and tell me when it finishes.

## Prompt 2

Ask one visible peer for the title of `README.md`.

## Prompt 3

Run `printf 'LAST_DONE\n'` somewhere visible and tell me when it finishes.

## Host setup

- Record baseline roster/focus and ensure all test-owned finite and peer targets are absent.

## Host perturbations

_None._

## Verdict

- outcome: Both finite outputs and the peer title arrive once in request order.
- orchestration: Each job gets one exact owner and settles before the next logical job begins; no prior target is silently adopted.
- responsiveness: The host remains usable and focus unchanged at every checkpoint.
- presentation: Each result is concise, useful, and clearly associated with its request.
- cleanup: Exact test-owned state returns to baseline after every settled checkpoint, not only at final teardown, with no delayed traffic.

## Evidence

- Record roster, lifecycle, output, and focus after each of the three checkpoints plus a final canary turn.

## Cleanup

- Remove only any surviving exact finite or peer target and restore baseline.

## Claude answer key

- Use stable visible finite/peer targets and prove exact absence after each result before starting the next.

## Pi answer key

- Use tracked finite `run` operations and one atomic peer handoff; capture each outcome before exact release and confirm absence at every checkpoint.
