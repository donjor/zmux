---
id: "ZD-012"
title: "Reload or respawn only the resolved Pi process"
applicability: ["pi"]
divergenceReason: "Claude does not expose Pi extension reload/respawn lifecycle operations."
---

## Invariant

Pi lifecycle actions target the current Pi pane safely and require a continuation to prove replacement completion.

## Shared instruction

After Pi package changes, prefer a soft reload; use hard respawn only when needed, resolve the current Pi pane rather than the desktop terminal, and require a continuation as completion proof.

## Pi mechanism

pi_reload and pi_respawn composites

## Pi enforcement

composite

## Pi prompt guideline

_None._

## Pi caveats

- terminal_current diagnoses desktop terminal attachment; it does not identify the Pi pane.

## Verify

- docs/domains/pi-zmux-extension.md
- pi-zmux/test/dispatcher.mjs

