---
id: "ZS-001"
title: "Run one finite visible command"
tier: "atomic"
doctrineRefs: ["ZD-001","ZD-002","ZD-003","ZD-006"]
applicability: ["claude","pi"]
uatEligible: true
---

## Prompt 1

pi-zmux test
run this command in a zmux tab

```bash
n=$((RANDOM % 11 + 5)); echo "sleeping ${n}s"; sleep "$n"; echo "sleep-echo-test complete after ${n}s"
```

## Host setup

- Record the baseline roster and focus; remove only a stale test-owned finite-command target.

## Host perturbations

_None._

## Verdict

- outcome: One visible stable target runs the supplied command and reaches a truthful zero exit with its completion line available.
- orchestration: The agent uses one visible bounded-command route and one completion owner without redundant waiting or retry choreography.
- responsiveness: The host remains available and terminal focus does not move.
- presentation: Status shows meaningful progress rather than a 24-hour countdown, and the final styled result includes the decisive command output without raw diagnostics.
- cleanup: The test-owned target is absent after teardown and no delayed completion appears on a later canary turn.

## Evidence

- Inspect the target lifecycle, terminal output, visible Pi or Claude result, focus, and one later canary turn.

## Cleanup

- Remove only the finite-command target and verify the baseline roster and focus.

## Claude answer key

- Use one detached visible named run with focus disabled, then inspect its lifecycle and output after completion.

## Pi answer key

- Use `run` once with a stable target, `focus: false`, and finite automatic completion tracking; inspect the target only from the host side.
