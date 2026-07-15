---
id: "ZD-009"
title: "Keep peers interactive and clean their lifecycle"
applicability: ["claude","pi"]
---

## Invariant

Peers never use headless print mode and are consumed, parked with a reason, or killed when no concrete next checkpoint exists.

## Shared instruction

Launch peers as visible interactive CLIs, never print/headless one-shots; after consuming the answer, retain only for a concrete next checkpoint and otherwise clean up the tab and lifecycle state.

## Claude mechanism

Launch profiles plus tab peer state/kill

## Claude enforcement

guard

## Claude prompt guideline

_None._

## Claude caveats

- Prompt scope, not OS sandboxing, defines a read-only review.

## Pi mechanism

Headless launch rejection plus tab_peer/tab_kill

## Pi enforcement

guard

## Pi prompt guideline

_None._

## Pi caveats

- Immediate teardown may omit an intermediate consumed state.

## Verify

- skills/zmux/references/agent-peer.md
- skills/zmux/test/doctor.mjs

