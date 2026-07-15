---
id: "ZS-012"
title: "Run a small development loop"
tier: "workflow"
doctrineRefs: ["ZD-001","ZD-003","ZD-004","ZD-006","ZD-010"]
applicability: ["claude","pi"]
uatEligible: true
---

## Prompt 1

Start the fixture dev server in a zmux tab and tell me when it is ready.

## Prompt 2

While it stays running, run the fixture smoke check somewhere visible.

## Prompt 3

Show me the latest dev-server output.

## Prompt 4

Restart the dev server in the same place and tell me when it is ready again.

## Prompt 5

Stop the dev server now.

## Host setup

- Use the repository dev-server fixture, record baseline roster/focus, and ensure test-owned runtime and smoke targets are absent.

## Host perturbations

_None._

## Verdict

- outcome: One stable runtime progresses through ready, concurrent smoke, inspected output, fresh restart, and confirmed stop.
- orchestration: Each turn reuses the existing runtime and chooses one semantic route without duplicate starts or stale readiness.
- responsiveness: The host remains usable and focus stays unchanged throughout the five turns.
- presentation: Each checkpoint clearly distinguishes persistent runtime state from finite smoke work and surfaces useful output.
- cleanup: Runtime and smoke targets are absent after the final turn with no delayed completion on a canary turn.

## Evidence

- Inspect target identities/counts, lifecycle generations, smoke exit/output, focus after each turn, and final canary traffic.

## Cleanup

- Remove only the exact fixture runtime and smoke targets and restore baseline.

## Claude answer key

- Compose visible runtime start/log/restart/stop with one visible bounded smoke target and stable names.

## Pi answer key

- Compose `runtime_ensure`, `run`, `runtime_logs`, restarted `runtime_ensure`, and `runtime_stop`; never add a second readiness watcher.
