---
id: "ZS-005"
title: "Open a sidecar without stealing focus"
doctrineRefs: ["ZD-003","ZD-007"]
applicability: ["claude","pi"]
---

## Prompt

Open a right-side pane named `doctrine-test-logs` that tails `pi-zmux/fixtures/dev-server/logs/app.txt`. Keep focus where it is; visibility is not a request to move focus.

## Setup

- Ensure the fixture log exists and record current focus.

## Expected outcome

One test-owned sidecar opens on the right while focus remains unchanged.

## Evidence

- returned raw pane id
- pane roster and placement
- focus before/after

## Safety

- close only returned pane id

## Cleanup

- close returned pane id after judgment

## Claude answer key

- pane open right with no focus

## Pi answer key

- pane_open direction right focus false

