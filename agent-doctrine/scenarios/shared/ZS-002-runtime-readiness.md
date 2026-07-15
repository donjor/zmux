---
id: "ZS-002"
title: "Start one long-running runtime"
tier: "atomic"
doctrineRefs: ["ZD-001","ZD-003","ZD-004","ZD-006"]
applicability: ["claude","pi"]
uatEligible: true
---

## Prompt 1

pi-zmux test
simulate a node dev runner / long-running task in a zmux tab

```bash
n=$((RANDOM % 11 + 5))
echo "sleeping ${n}s"
sleep "$n"
echo "pseudo dev server running"
while true; do
  sleep 3600
done
```

## Host setup

- Record the baseline roster and focus; ensure the test-owned pseudo-dev-runner target is absent.

## Host perturbations

_None._

## Verdict

- outcome: Exactly one visible runtime remains alive after printing pseudo dev server running.
- orchestration: One atomic runtime-readiness route owns launch and readiness; the agent adds no second watcher or post-hoc readiness check.
- responsiveness: The host remains usable during startup and focus stays unchanged.
- presentation: The result states one coherent running-and-ready status with useful output, no ready-versus-unproven contradiction, and no raw internal dump.
- cleanup: The runtime is stopped exactly once during teardown and no delayed completion enters a later canary turn.

## Evidence

- Inspect target count, process liveness, readiness output, actual tool calls, visible result, focus, and a later canary turn.

## Cleanup

- Stop the exact pseudo-dev-runner target and restore the baseline roster and focus.

## Claude answer key

- Start one visible runtime with one readiness pattern and keep the runtime alive after readiness.

## Pi answer key

- Use `runtime_ensure` once with the supplied command and readiness text; do not add `callback_watch` or another observation path.
