---
id: "ZS-011"
title: "Address a worker in an explicit session"
doctrineRefs: ["ZD-002","ZD-010"]
applicability: ["claude","pi"]
---

## Prompt

In session `zmux/doctrine-test-target`, send `hello-doctrine` to the existing `doctrine-test-same-worker` tab and prove its response. Do not act on the same-named worker in `zmux/doctrine-test-decoy`, and do not create a replacement if the target is absent.

## Setup

- Host creates two sessions with same-named decoy and target worker tabs.

## Expected outcome

Only the worker in the named session receives input and produces the expected response.

## Evidence

- session roster
- target response
- decoy unchanged

## Safety

- explicit session on reads and writes

## Cleanup

- remove both test sessions

## Claude answer key

- session-pinned type/watch

## Pi answer key

- type_text and tab_inspect with options.session

