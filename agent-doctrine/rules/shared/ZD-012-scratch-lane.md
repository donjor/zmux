---
id: "ZD-012"
title: "Share one scratch lane for bounded commands"
applicability: ["claude","pi"]
---

## Invariant

Bounded one-shot commands that exit on their own share a single reused scratch tab instead of minting an ad-hoc tab per command.

## Shared instruction

Run bounded checks (typecheck, test, lint, build, one-shot scripts) through the shared scratch lane; reserve a named tab for durable runtimes or work you must keep addressable.

## Claude mechanism

zmux scratch / unnamed zmux run scratch-default

## Claude enforcement

instruction

## Claude prompt guideline

For a bounded command that exits on its own, use `zmux scratch '<cmd>'` or a bare `zmux run '<cmd>'`; both claim and reuse the one scratch tab. Only pass `-n <name>` when the run is durable (a server/watcher) or you must address it later.

## Claude caveats

- The scratch lane is reused, not immortal — an idle scratch tab is still reaped and reminted on the next bounded run.

## Pi mechanism

scratch-default bounded run reuse

## Pi enforcement

instruction

## Pi prompt guideline

Route bounded one-shot checks to the shared scratch lane; name a tab only for durable runtimes or work you must re-address.

## Pi caveats

- The scratch lane is reused, not immortal — an idle scratch tab is still reaped and reminted on the next bounded run.

## Verify

- skills/zmux/references/guard-and-tab-states.md
- skills/zmux/references/run-observe.md
