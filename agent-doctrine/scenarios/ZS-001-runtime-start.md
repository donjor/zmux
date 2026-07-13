---
id: "ZS-001"
title: "Start one visible runtime"
doctrineRefs: ["ZD-001","ZD-003","ZD-004"]
applicability: ["claude","pi"]
---

## Prompt

In `pi-zmux/fixtures/dev-server`, start the dev script in one visible stable place named `doctrine-test-server`, wait until it prints ready or localhost, keep focus unchanged, and do not leave a hidden shell job behind.

## Setup

- Remove only a stale test-owned doctrine-test-server target.

## Expected outcome

Exactly one visible named runtime exists and fresh output proves readiness without focus movement.

## Evidence

- roster contains one target
- fresh readiness output
- focus before/after

## Safety

- test-owned doctrine-test state only
- no hidden process

## Cleanup

- retain runtime for ZS-002 and ZS-003

## Claude answer key

- run/watch with stable name and pinned session

## Pi answer key

- runtime_ensure with readiness and focus false

