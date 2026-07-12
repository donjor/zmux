---
id: "ZD-003"
title: "Do not move focus implicitly"
applicability: ["claude","pi"]
---

## Invariant

Terminal focus moves only after an explicit user request.

## Shared instruction

Keep terminal focus unchanged unless the user explicitly asks to move it; visibility alone is not a focus request.

## Claude mechanism

Detached/default CLI flags

## Claude enforcement

instruction

## Claude prompt guideline

_None._

## Claude caveats

_None._

## Pi mechanism

focus options default false

## Pi enforcement

typed-operation

## Pi prompt guideline

Never set focus:true unless the user explicitly wants terminal focus moved; if visibility is requested without focus, keep focus false.

## Pi caveats

- Explicit tab_focus and pane_focus remain available for direct user requests.

## Verify

- skills/zmux/references/run-observe.md
- pi-zmux/test/dispatcher.mjs

