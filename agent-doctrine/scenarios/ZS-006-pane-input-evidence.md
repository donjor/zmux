---
id: "ZS-006"
title: "Resolve pane input and capture evidence"
doctrineRefs: ["ZD-002","ZD-007"]
applicability: ["claude","pi"]
---

## Prompt

Find the test-owned pane titled `doctrine-test-peer`, send the literal text `ping` without submitting it, resize it to 40 columns, then capture text or ANSI evidence of the resulting terminal state. Do not use raw tmux.

## Setup

- Host creates one joined test pane and records its raw id.

## Expected outcome

The inspected raw pane is mutated, input remains unsubmitted, and minimal terminal evidence is captured.

## Evidence

- pane roster/title match
- raw pane id
- width
- snapshot reference

## Safety

- no Enter key
- no raw tmux

## Cleanup

- close the exact test pane

## Claude answer key

- pane list then send keys/resize/snapshot

## Pi answer key

- current then panes, pane_send_keys, pane_resize, snapshot

