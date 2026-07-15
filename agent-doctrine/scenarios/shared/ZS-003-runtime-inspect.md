---
id: "ZS-003"
title: "Inspect and reuse an existing runtime"
tier: "atomic"
doctrineRefs: ["ZD-001","ZD-002","ZD-003","ZD-004"]
applicability: ["claude","pi"]
uatEligible: false
---

## Prompt 1

The `pseudo-dev-runner` should already be running. Show me its latest output; don't start another copy.

## Host setup

- Start one test-owned pseudo-dev-runner with recognizable output and record its target identity, roster, and focus.

## Host perturbations

_None._

## Verdict

- outcome: The latest output from the existing runtime is shown and the original runtime remains alive.
- orchestration: The agent inspects or reuses the existing target before creation and never starts a duplicate.
- responsiveness: Focus remains unchanged and the host stays available.
- presentation: The answer distinguishes existing runtime state from newly started work and surfaces useful output concisely.
- cleanup: Exactly the original test-owned runtime is stopped during teardown with no unrelated target mutation.

## Evidence

- Compare target identity and count before and after, inspect output, and record focus.

## Cleanup

- Stop the original pseudo-dev-runner and restore the baseline roster.

## Claude answer key

- Inspect existing logs/status before any creation path and report the retained target.

## Pi answer key

- Use `runtime_logs` on the existing target; do not call `runtime_ensure` for another copy.
