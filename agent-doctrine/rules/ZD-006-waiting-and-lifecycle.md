---
id: "ZD-006"
title: "Use bounded first-class evidence"
applicability: ["claude","pi"]
---

## Invariant

Completion and readiness are proven by fresh lifecycle or output evidence, never polling, elapsed time, process existence, or echoed prompt text.

## Shared instruction

Use bounded first-class lifecycle or future-output waits; do not poll, sleep as proof, treat process liveness as completion, or accept a marker already present in the prompt tail.

## Claude mechanism

zmux wait/status/watch with fresh baselines

## Claude enforcement

instruction

## Claude prompt guideline

_None._

## Claude caveats

- Idle is a fallback for uninstrumented programs, not lifecycle truth.

## Pi mechanism

wait/callback_watch plus structured command and turn state

## Pi enforcement

typed-operation

## Pi prompt guideline

For wait/callback_watch, choose exactly one of waitFor or idleSeconds with a bounded timeout; waitFor is the regex only, outgoing text must not satisfy future evidence, and deliverAs=nextTurn cannot trigger a continuation.

## Pi caveats

- deliverAs=nextTurn cannot trigger a continuation; use steer or followUp when triggerTurn is true.

## Verify

- skills/zmux/references/run-observe.md
- pi-zmux/test/dispatcher.mjs

