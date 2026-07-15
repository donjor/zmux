---
id: "ZS-007"
title: "Type literal input into a visible pane"
tier: "atomic"
doctrineRefs: ["ZD-002","ZD-003","ZD-005","ZD-007"]
applicability: ["claude","pi"]
uatEligible: false
---

## Prompt 1

In the pane titled `test-console`, type `ping` but don't press Enter. Then show me what is in that pane.

## Host setup

- Create one joined pane titled test-console, record its raw pane id, contents, dimensions, and focus.

## Host perturbations

_None._

## Verdict

- outcome: Literal ping appears unsubmitted in the intended pane and the pane snapshot is shown.
- orchestration: The agent resolves the named joined pane to its raw identity and sends literal keys without submitting them.
- responsiveness: Focus remains on the original pane.
- presentation: The snapshot is readable and clearly tied to test-console.
- cleanup: The host closes the exact test-console pane and restores the baseline pane roster.

## Evidence

- Inspect literal pane contents before and after, snapshot text, raw pane identity, and focus.

## Cleanup

- Close the exact host-created test-console pane.

## Claude answer key

- Resolve the named pane in the current session, send literal keys without Enter, then capture its contents.

## Pi answer key

- Resolve the raw pane via `current` and `panes`, use `pane_send_keys` with literal keys, then `snapshot`; do not use `pane_type`.
