---
id: "ZS-010"
title: "Ask one visible peer for a bounded answer"
tier: "atomic"
doctrineRefs: ["ZD-003","ZD-008","ZD-009","ZD-010"]
applicability: ["claude","pi"]
uatEligible: true
---

## Prompt 1

Ask one visible Pi peer to read `README.md` and give me its one-sentence description.

## Host setup

- Record the baseline roster and focus; ensure the test-owned peer target is absent and the intended interactive peer CLI is available.

## Host perturbations

_None._

## Verdict

- outcome: One visible peer returns one useful sentence grounded in README.md.
- orchestration: The agent creates or reuses one interactive peer, submits once, and waits for fresh lifecycle completion without hidden or print-mode delegation.
- responsiveness: The host stays usable and focus remains unchanged while the peer works.
- presentation: The final answer contains the peer's useful sentence rather than lifecycle chrome or a raw terminal tail.
- cleanup: The exact test-owned peer is released or removed and no delayed duplicate answer appears.

## Evidence

- Inspect peer visibility, prompt acceptance, fresh ready lifecycle, answer content, focus, roster, and one later canary turn.

## Cleanup

- Remove only the exact test-owned peer target and verify baseline roster.

## Claude answer key

- Launch one visible low-tier interactive peer with official lifecycle instrumentation and submit the bounded task once.

## Pi answer key

- Use `peer_ensure` followed by one atomic `peer_handoff`; do not split typing and callback registration.
