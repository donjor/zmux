---
id: "ZD-011"
title: "Make remote mutation legible"
applicability: ["claude","pi"]
---

## Invariant

Remote/admin work reuses one stable host target and states the decoded intended mutation before execution.

## Shared instruction

Reuse one stable admin or remote-host target, avoid numbered tab sprawl, decode opaque payloads, and state the intended host mutation before changing remote configuration.

## Claude mechanism

Skill doctrine and guard warnings

## Claude enforcement

guard

## Claude prompt guideline

_None._

## Claude caveats

_None._

## Pi mechanism

Dispatcher safety warnings and interactive routing

## Pi enforcement

guard

## Pi prompt guideline

For remote/admin runs, reuse one stable admin/remote-host tab, decode opaque payloads, and state the intended host mutation before changing remote config.

## Pi caveats

- Warnings do not infer whether a decoded mutation is authorized.

## Verify

- skills/zmux/SKILL.md
- pi-zmux/test/dispatcher.mjs
