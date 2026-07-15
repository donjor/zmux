---
id: "ZD-006"
title: "Use bounded first-class evidence"
applicability: ["claude","pi"]
---

## Invariant

Completion and readiness are proven by fresh lifecycle or output evidence, never polling, elapsed time, process existence, or echoed prompt text.

## Shared instruction

Use bounded first-class lifecycle or future-output evidence; register callbacks before expected events and never add a post-hoc blind wait when lifecycle or callback evidence exists. Blind waiting is the last resort: try 10 seconds, inspect and reassess whether the expectation or mechanism is wrong, then escalate only to 30 and 60 seconds with the same reassessment. Do not poll, sleep as proof, treat process liveness as completion, or accept a marker already present in the prompt tail.

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

Before expected events, register callback_watch; never post-hoc blind wait when lifecycle/callback evidence exists. Blind wait last: 10s, inspect/reassess; then 30s, 60s max. One condition; echoed output is not evidence; nextTurn cannot trigger.

## Pi caveats

- deliverAs=nextTurn cannot trigger a continuation; use steer or followUp when triggerTurn is true.
- A timeout is diagnostic: before escalating, inspect current lifecycle and ask whether the event already happened, the evidence channel is stale, or the chosen mechanism is wrong.

## Verify

- skills/zmux/references/run-observe.md
- pi-zmux/test/dispatcher.mjs

