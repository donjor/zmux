---
id: "ZS-013"
title: "Delegate parallel reading while the host stays available"
tier: "workflow"
doctrineRefs: ["ZD-003","ZD-008","ZD-009","ZD-010"]
applicability: ["claude","pi"]
uatEligible: true
---

## Prompt 1

Ask three visible workers to read `README.md`, `docs/architecture.md`, and `docs/ROADMAP.md`, one file each, then give me one ordered summary.

## Prompt 2

While they work, tell me which Go package owns logical tab state.

## Host setup

- Record baseline roster and focus; ensure no test-owned workers session or worker targets exist and all three files are readable.

## Host perturbations

_None._

## Verdict

- outcome: The host answers the package question promptly and later presents one ordered useful three-file aggregate.
- orchestration: One semantic parallel delegation owns exactly three accepted tasks; no hidden workers, duplicate aggregate, or unnecessary per-worker waiting is added.
- responsiveness: The second turn is accepted and answered while workers remain active; focus stays unchanged.
- presentation: Background activity stays compact and readable, and the aggregate contains answers rather than lifecycle metadata or terminal chrome.
- cleanup: Exact workers and any owned workers session are gone after settlement with no duplicate or delayed aggregate.

## Evidence

- Inspect worker visibility/acceptance, host turn timing, activity presentation, ordered answers, focus, roster, and a later canary turn.

## Cleanup

- Remove only exact test-owned workers and their owned workers session after outcomes are captured.

## Claude answer key

- Create three visible interactive peers, keep the host responsive, collect results once in requested order, then remove exact peers.

## Pi answer key

- Launch three asynchronous `peer_ensure` workers for the requested reads and aggregate their answers once in requested order without per-worker joins, then settle before exact cleanup.
