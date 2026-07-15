---
id: "ZS-014"
title: "Reuse one peer for a natural follow-up"
tier: "workflow"
doctrineRefs: ["ZD-008","ZD-009","ZD-010"]
applicability: ["claude","pi"]
uatEligible: false
---

## Prompt 1

Ask one visible peer for a two-sentence review of the README introduction.

## Prompt 2

Ask that same peer whether the introduction matches the architecture document.

## Host setup

- Record baseline roster/focus and ensure the test-owned peer is absent; make both documents readable.

## Host perturbations

_None._

## Verdict

- outcome: The same peer supplies a bounded initial review and a context-aware follow-up answer.
- orchestration: The retained peer is reused only while context is useful and renewed before the follow-up; no second peer is created.
- responsiveness: The host remains available and focus stays unchanged between turns.
- presentation: Both peer answers are concise and attributable without lifecycle noise.
- cleanup: Retention ends after the checkpoint and the exact peer is absent without delayed output.

## Evidence

- Compare peer identity across turns, prompt acceptance, fresh answers, retention state, focus, and final roster.

## Cleanup

- End retention and remove the exact peer after the second answer is captured.

## Claude answer key

- Reuse one visible peer for the follow-up, then explicitly remove it when the checkpoint ends.

## Pi answer key

- Retain the first `peer_handoff` only for a bounded checkpoint, renew before the second prompt, then release the exact peer.
