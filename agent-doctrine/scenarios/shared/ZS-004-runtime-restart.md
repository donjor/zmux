---
id: "ZS-004"
title: "Restart one runtime in place"
tier: "atomic"
doctrineRefs: ["ZD-001","ZD-003","ZD-004","ZD-006"]
applicability: ["claude","pi"]
uatEligible: false
---

## Prompt 1

Restart the `pseudo-dev-runner` in the same place and tell me when it is ready again.

## Host setup

- Start one test-owned pseudo-dev-runner and record its stable target, current lifecycle generation, roster, and focus.

## Host perturbations

_None._

## Verdict

- outcome: The same stable target restarts and emits fresh readiness from the new process generation.
- orchestration: The restart is explicit and in place; no duplicate runtime or stale readiness is accepted.
- responsiveness: The host remains usable and focus stays unchanged.
- presentation: The answer clearly distinguishes restart from initial start and cites fresh useful output.
- cleanup: The restarted runtime is stopped exactly once and the roster returns to baseline.

## Evidence

- Compare target identity and lifecycle generation, inspect fresh output, count targets, and record focus.

## Cleanup

- Stop the exact restarted runtime and restore the baseline roster.

## Claude answer key

- Restart the existing named runtime in place and require readiness newer than the baseline.

## Pi answer key

- Use `runtime_ensure` with `restart: true` on the same target and one atomic readiness condition.
