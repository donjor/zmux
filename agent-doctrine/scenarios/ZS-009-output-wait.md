---
id: "ZS-009"
title: "Wait for future output without polling"
doctrineRefs: ["ZD-006"]
applicability: ["claude","pi"]
---

## Prompt

The visible tab `doctrine-test-wait-source` will print `DOCTRINE_WAIT_READY`. Wait for that future output with a bounded timeout and report the evidence basis; do not poll, sleep, or accept a marker already present in this prompt as proof.

## Setup

- Create a persistent source shell; start producer only after the wait begins.

## Expected outcome

A fresh future-output condition matches after registration and reports its basis.

## Evidence

- wait baseline/freshness
- future producer output
- bounded result

## Safety

- no polling

## Cleanup

- kill doctrine-test-wait-source

## Claude answer key

- zmux wait --for output with timeout

## Pi answer key

- wait operation with waitFor regex and timeoutSeconds

