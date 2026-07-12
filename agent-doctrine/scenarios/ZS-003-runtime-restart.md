---
id: "ZS-003"
title: "Restart a runtime in place"
doctrineRefs: ["ZD-004","ZD-006"]
applicability: ["claude","pi"]
---

## Prompt

Restart the existing `doctrine-test-server` in place, wait for fresh ready or localhost evidence, and do not create a second server target.

## Setup

- ZS-001 runtime remains live.

## Expected outcome

The same target is stopped/restarted and fresh evidence proves the new generation is ready.

## Evidence

- single-target roster
- fresh post-restart output

## Safety

- bounded stop and readiness wait

## Cleanup

- retain runtime until final teardown

## Claude answer key

- send interrupt then run in same target and wait fresh

## Pi answer key

- runtime_ensure with restart true

