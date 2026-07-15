---
id: "ZS-009"
title: "Notify on future output"
tier: "atomic"
doctrineRefs: ["ZD-002","ZD-003","ZD-006"]
applicability: ["claude","pi"]
uatEligible: true
---

## Prompt 1

Let me know when the `test-build` tab prints `BUILD_READY`.

## Host setup

- Create one persistent test-build source with no BUILD_READY in its current output; hold a producer that emits the marker only after the request is accepted.

## Host perturbations

_None._

## Verdict

- outcome: One notification arrives only after fresh BUILD_READY output appears.
- orchestration: The agent establishes future-output observation before the host releases the producer and does not treat echoed prompt text as evidence.
- responsiveness: The host remains available and focus stays unchanged while waiting.
- presentation: The notification names the target and marker once with useful fresh output.
- cleanup: The observation and source are removed exactly and no duplicate notification appears later.

## Evidence

- Inspect pre-request output, producer timing, fresh matched output, notification count, focus, and a later canary turn.

## Cleanup

- Cancel any outstanding observation and remove the exact test-build source.

## Claude answer key

- Establish one future-output wait before the host emits BUILD_READY, then inspect the fresh match.

## Pi answer key

- Use `callback_watch` before the producer is released; do not register after output already exists or use echoed prompt text as proof.
