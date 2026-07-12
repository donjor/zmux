---
id: "ZD-005"
title: "Route manual input through a visible shared terminal"
applicability: ["claude","pi"]
---

## Invariant

Sudo, SSH, password prompts, REPLs, database shells, and similar manual input never run as hidden host-shell jobs.

## Shared instruction

Run manual-input commands in one visible stable admin or remote target, keep focus unchanged by default, and wait only with bounded lifecycle or prompt evidence.

## Claude mechanism

Visible tab plus type/watch/status CLI sequence

## Claude enforcement

instruction

## Claude prompt guideline

_None._

## Claude caveats

_None._

## Pi mechanism

interactive_type composite

## Pi enforcement

composite

## Pi prompt guideline

For sudo, ssh, passwords, REPLs, database shells, and other manual input, use interactive_type and never generic run; target one stable admin/remote tab, keep focus false, and use bounded waitForExit when appropriate.

## Pi caveats

- A password prompt may return needsUserInput rather than moving focus automatically.

## Verify

- skills/zmux/references/run-observe.md
- pi-zmux/test/dispatcher.mjs

