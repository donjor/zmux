---
id: "ZS-018"
title: "Interrupt a joined wait without cancelling accepted work"
tier: "resilience"
doctrineRefs: ["ZD-003","ZD-006","ZD-008","ZD-009","ZD-010"]
applicability: ["claude","pi"]
uatEligible: false
---

## Prompt 1

Ask three visible workers to summarize one architecture section each. Wait for all three before you continue.

## Prompt 2

What package owns logical tab state?

## Host setup

- Prepare three held interactive worker fixtures, record baseline roster/focus, and ensure test-owned workers are absent.

## Host perturbations

- Release one worker, press Escape during the explicit joined wait, then send Prompt 2 before releasing the remaining workers.

## Verdict

- outcome: Escape detaches only foreground waiting; all accepted workers survive, the host answers Prompt 2, and all three useful results settle once.
- orchestration: No accepted worker is cancelled or duplicated and no second aggregate owner is created after Escape.
- responsiveness: The host accepts Prompt 2 immediately after detachment and focus stays unchanged.
- presentation: Partial and final states are coherent, bounded, and contain useful answers rather than lifecycle noise.
- cleanup: All exact workers and owned session state are gone after final capture with no delayed duplicate aggregate.

## Evidence

- Inspect worker acceptance/liveness before and after Escape, host turn timing, result count/order, focus, roster, and canary traffic.

## Cleanup

- Capture outstanding outcomes, remove exact workers/session, and restore baseline.

## Claude answer key

- Keep accepted visible peers alive when foreground observation is interrupted; collect each result once later.

## Pi answer key

- Join three submitted `peer_ensure` workers with one foreground `wait`; Escape must detach that wait without cancelling the workers or their single ordered aggregation.
