---
id: "ZD-001"
title: "Route work by lifecycle and visibility"
applicability: ["claude","pi"]
---

## Invariant

Bounded non-interactive inspection may use the host shell; visible, interactive, privileged, persistent, or long-running work belongs in zmux.

## Shared instruction

Use zmux for visible, interactive, privileged, persistent, or long-running terminal work; keep only bounded non-interactive inspection in the host shell.

## Claude mechanism

CLI verbs and skill routing

## Claude enforcement

instruction

## Claude prompt guideline

_None._

## Claude caveats

_None._

## Pi mechanism

One typed zmux dispatcher plus Bash guard

## Pi enforcement

guard

## Pi prompt guideline

Use zmux instead of bash/raw tmux for runtimes, visible tabs, panes, sessions, waits, peers, and Pi lifecycle; never background long-running commands.

## Pi caveats

- The Bash guard classifies clear unsafe routes; ambiguous commands still rely on instruction.

## Verify

- skills/zmux/test/doctor.mjs
- pi-zmux/test/run.mjs
