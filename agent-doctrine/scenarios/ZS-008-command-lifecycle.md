---
id: "ZS-008"
title: "Prove command lifecycle structurally"
doctrineRefs: ["ZD-001","ZD-006"]
applicability: ["claude","pi"]
---

## Prompt

Run `sleep 3; printf 'DOCTRINE_COMMAND_DONE\n'` in a visible tab named `doctrine-test-command`. Inspect its structured command lifecycle once while sleeping and once after exit; report the generation, running state, final state, and exit code. Output or process liveness alone is insufficient.

## Setup

- Remove stale doctrine-test-command target.

## Expected outcome

One fresh command generation transitions from running to done with exit zero.

## Evidence

- fresh command generation
- running state
- done state and exit code

## Safety

- bounded command

## Cleanup

- kill doctrine-test-command

## Claude answer key

- run then session-pinned status inspections

## Pi answer key

- run then tab_status/tab_inspect

