---
id: "ZD-010"
title: "Fail closed and clean exact owned state"
applicability: ["claude","pi"]
---

## Invariant

A missing or ambiguous target is reported without invention, and teardown removes only exact test/task-owned objects.

## Shared instruction

When a target is missing or ambiguous, report the exact failure and stop rather than creating a substitute; clean only exact state owned by the task and prove the final roster.

## Claude mechanism

Resolver failures plus explicit kill/session cleanup

## Claude enforcement

instruction

## Claude prompt guideline

_None._

## Claude caveats

_None._

## Pi mechanism

Typed target resolution plus tab_kill/session_kill

## Pi enforcement

typed-operation

## Pi prompt guideline

_None._

## Pi caveats

_None._

## Verify

- skills/zmux/references/run-observe.md
- agent-doctrine/harnesses/claude/host-flow.md

