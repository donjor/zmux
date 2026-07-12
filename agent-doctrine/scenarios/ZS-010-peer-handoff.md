---
id: "ZS-010"
title: "Complete one visible peer handoff"
doctrineRefs: ["ZD-002","ZD-006","ZD-008","ZD-009"]
applicability: ["claude","pi"]
---

## Prompt

Reuse the existing visible peer `doctrine-test-peer`. Ask it for a one-line identification of its CLI and model. Use fresh lifecycle completion when available, inspect the reply, and do not use a headless/print launch or an echoed output marker as proof.

## Setup

- Host launches one low-tier interactive lifecycle-instrumented peer in the test session.

## Expected outcome

The peer is inspected, marked running before submission, advances to a newer ready generation, and its visible reply is consumed.

## Evidence

- pre/post turn generation
- running then ready lifecycle
- visible reply

## Safety

- interactive peer
- session pinned
- focus unchanged

## Cleanup

- consume then kill unless a concrete next checkpoint exists

## Claude answer key

- peer ensure/inspect/type/wait/status sequence

## Pi answer key

- peer_ensure then atomic peer_handoff and tab_inspect

