---
id: "ZS-014"
title: "Schedule a Pi callback without blocking"
doctrineRefs: ["ZD-006"]
applicability: ["pi"]
divergenceReason: "Claude skill mechanics have no in-process Pi follow-up delivery channel."
---

## Prompt

Arrange for this Pi process to be notified when `doctrine-test-callback-source` prints `DOCTRINE_CALLBACK_DONE`. Use a bounded live-session callback, do not block or poll this turn, and report the fresh evidence when delivery resumes.

## Setup

- Create a persistent source shell; produce output only after callback registration.

## Expected outcome

The callback schedules, remains visibly active, then delivers one compact fresh completion and clears its activity state.

## Evidence

- scheduled result
- active footer
- fresh callback message
- empty callback list

## Safety

- bounded timeout
- no callback handle leakage

## Cleanup

- cancel callback if still active
- kill source tab

## Pi answer key

- callback_watch with followUp/nextTurn semantics appropriate to triggerTurn

