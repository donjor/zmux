---
id: "ZD-008"
title: "Drive visible peers through fresh lifecycle"
applicability: ["claude","pi"]
---

## Invariant

A peer is a visible interactive CLI; prompts are submitted only after inspection and completion is fresh lifecycle truth when instrumentation exists.

## Shared instruction

Reuse one visible interactive peer, inspect its composer before sending, mark it running, and accept completion only from a newer ready lifecycle generation; use output or idle only for uninstrumented fallback.

## Claude mechanism

peer ensure/type/wait/status/inspect sequence

## Claude enforcement

instruction

## Claude prompt guideline

_None._

## Claude caveats

- The peer skill owns when and which peer to select.

## Pi mechanism

peer_ensure plus atomic peer_handoff

## Pi enforcement

composite

## Pi prompt guideline

_None._

## Pi caveats

- The peer skill still owns selection, roles, and engagement policy.

## Verify

- skills/zmux/references/agent-peer.md
- pi-zmux/test/dispatcher.mjs

