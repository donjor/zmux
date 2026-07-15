---
id: "ZD-007"
title: "Target panes structurally and capture minimal evidence"
applicability: ["claude","pi"]
---

## Invariant

Pane mutations use an inspected raw pane id; terminal evidence is captured in the least invasive useful form.

## Shared instruction

Inspect joined panes before pane mutation, use the returned raw pane id, preserve literal-vs-submit input semantics, and prefer text or ANSI evidence when an image is unnecessary.

## Claude mechanism

pane list/open/send/resize/snapshot CLI verbs

## Claude enforcement

instruction

## Claude prompt guideline

_None._

## Claude caveats

_None._

## Pi mechanism

panes and pane_* operations plus snapshot

## Pi enforcement

typed-operation

## Pi prompt guideline

_None._

## Pi caveats

- terminal_current diagnoses desktop attachment and may be unsupported.

## Verify

- skills/zmux/references/cli-catalog.md
- pi-zmux/test/dispatcher.mjs
