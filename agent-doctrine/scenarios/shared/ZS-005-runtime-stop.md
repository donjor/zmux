---
id: "ZS-005"
title: "Stop one existing runtime"
tier: "atomic"
doctrineRefs: ["ZD-001","ZD-003","ZD-004","ZD-010"]
applicability: ["claude","pi"]
uatEligible: false
---

## Prompt 1

Stop the `pseudo-dev-runner` and tell me when it is gone.

## Host setup

- Start one test-owned pseudo-dev-runner and record its target identity, roster, and focus.

## Host perturbations

_None._

## Verdict

- outcome: The exact runtime is stopped and its target is absent.
- orchestration: The agent stops the named runtime without killing unrelated state or inventing a substitute target.
- responsiveness: Focus remains unchanged and the host remains available.
- presentation: The answer reports confirmed absence rather than merely claiming a signal was sent.
- cleanup: No test-owned runtime or delayed completion remains after the verdict.

## Evidence

- Inspect target absence, process state, roster, focus, and one later canary turn.

## Cleanup

- Remove only any surviving test-owned pseudo-dev-runner state and restore baseline.

## Claude answer key

- Stop the exact named runtime and confirm target absence.

## Pi answer key

- Use `runtime_stop` on the exact target and verify absence host-side.
