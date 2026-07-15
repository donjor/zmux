---
id: "ZS-006"
title: "Open one visible sidecar"
tier: "atomic"
doctrineRefs: ["ZD-001","ZD-003","ZD-007"]
applicability: ["claude","pi"]
uatEligible: true
---

## Prompt 1

Open the app log in a right-hand sidecar. Keep me in this tab.

## Host setup

- Ensure the fixture log exists; record the current session, pane roster, placement, and focus.

## Host perturbations

_None._

## Verdict

- outcome: One visible sidecar opens on the right and shows the app log.
- orchestration: The agent opens a joined pane rather than a hidden job or unrelated tab.
- responsiveness: The original pane keeps focus and remains usable.
- presentation: Placement and content are immediately understandable without raw addressing noise.
- cleanup: The host closes the exact returned sidecar pane and restores the pane roster.

## Evidence

- Inspect pane identity, placement, content snapshot, focus, and final pane roster.

## Cleanup

- Close the exact sidecar pane by raw pane identity.

## Claude answer key

- Open one right-side pane in the current session without focusing it and retain its raw pane id.

## Pi answer key

- Use `pane_open` with right placement and `focus: false`; retain the returned raw pane id for host cleanup.
