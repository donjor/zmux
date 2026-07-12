---
id: "ZD-004"
title: "Own runtime lifecycle by stable name"
applicability: ["claude","pi"]
---

## Invariant

A server or watcher is started once in a stable target, inspected before replacement, restarted in place when requested, and stopped explicitly.

## Shared instruction

Use one stable named runtime: inspect existing output before starting another copy, restart in place when requested, and stop it explicitly.

## Claude mechanism

zmux run/watch/send lifecycle

## Claude enforcement

instruction

## Claude prompt guideline

_None._

## Claude caveats

- Readiness is output evidence, not durable health.

## Pi mechanism

runtime_ensure/runtime_logs/runtime_stop composite

## Pi enforcement

composite

## Pi prompt guideline

For servers/watchers, use runtime_ensure/logs/stop. If asked for another copy before checking logs, use runtime_logs on the existing target first and do not start duplicate processes.

## Pi caveats

- Configured runtimes require a trusted project before commands are loaded.

## Verify

- skills/zmux/references/run-observe.md
- pi-zmux/test/dispatcher.mjs

