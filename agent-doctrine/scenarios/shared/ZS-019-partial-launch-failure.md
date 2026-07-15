---
id: "ZS-019"
title: "Preserve accepted work when one launch fails"
tier: "resilience"
doctrineRefs: ["ZD-003","ZD-008","ZD-009","ZD-010"]
applicability: ["claude","pi"]
uatEligible: false
---

## Prompt 1

Ask three visible workers to check the README, architecture document, and roadmap, then summarize what each says.

## Host setup

- Prepare three worker launch slots, hold their completion, record baseline roster/focus, and ensure no test-owned workers exist.

## Host perturbations

- Fail one worker launch before its prompt is accepted; allow the other two to accept their tasks.

## Verdict

- outcome: The two accepted tasks remain valid and the failed logical item is retried or reported without duplicating accepted work.
- orchestration: The agent distinguishes pre-acceptance failure from accepted work and retries at most the failed item.
- responsiveness: The host remains available and focus unchanged during recovery.
- presentation: The final result clearly distinguishes recovered, completed, and failed items and preserves requested order.
- cleanup: All exact workers and any owned session are removed after outcomes settle with no duplicate delivery.

## Evidence

- Inspect launch/acceptance records, worker identities, prompts, result order, focus, roster, and canary traffic.

## Cleanup

- Capture accepted outcomes and remove only exact test-owned workers/session.

## Claude answer key

- Preserve accepted visible peers and retry only the logical item whose prompt was never accepted.

## Pi answer key

- Fail one `peer_ensure` launch before its prompt is accepted; the two accepted workers must keep their identities and feed one ordered aggregation.
