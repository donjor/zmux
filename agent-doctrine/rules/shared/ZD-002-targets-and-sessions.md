---
id: "ZD-002"
title: "Reuse stable targets and pin ambiguous sessions"
applicability: ["claude","pi"]
---

## Invariant

Agents reuse stable descriptive targets and explicitly address the current session whenever a read or write could resolve elsewhere.

## Shared instruction

Inspect the roster before creating terminal state, reuse stable descriptive targets, and pin the current session whenever a name could resolve elsewhere.

## Claude mechanism

Session-pinned zmux CLI arguments

## Claude enforcement

instruction

## Claude prompt guideline

_None._

## Claude caveats

- Write resolution is session-scoped, but reads still require explicit pinning.

## Pi mechanism

sessions/tabs/current plus options.session

## Pi enforcement

typed-operation

## Pi prompt guideline

Start with operation=sessions or tabs when target/session is ambiguous; never operate on a generic tab name like scratch unless the prompt names the exact tab/session.

## Pi caveats

- The dispatcher cannot infer which same-named target the user intended.

## Verify

- skills/zmux/references/agent-peer.md
- pi-zmux/test/dispatcher.mjs

