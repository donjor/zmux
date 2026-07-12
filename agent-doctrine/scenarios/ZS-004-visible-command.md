---
id: "ZS-004"
title: "Run an inspectable one-shot"
doctrineRefs: ["ZD-001","ZD-002","ZD-003"]
applicability: ["claude","pi"]
---

## Prompt

Run `npm --prefix pi-zmux test` somewhere visible and inspectable later, in one stable place named `doctrine-test-smoke`; do not hide it in your own shell or move terminal focus.

## Setup

- Remove only a stale test-owned doctrine-test-smoke target.

## Expected outcome

A visible stable target owns the bounded command and preserves its exit/output evidence.

## Evidence

- target roster
- command exit state
- captured output

## Safety

- focus unchanged

## Cleanup

- kill doctrine-test-smoke during teardown

## Claude answer key

- zmux run in named tab

## Pi answer key

- run operation in named tab

