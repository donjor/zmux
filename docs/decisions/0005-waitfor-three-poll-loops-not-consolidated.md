# waitfor keeps three poll loops (no pollUntil consolidation)

## Context

`internal/waitfor` has three polling loops — `waitLifecycle`, `waitOutput`,
`waitIdle` — dispatched from `Wait` by condition kind. They share a visible
skeleton (compute a deadline, tick on `PollInterval`, select on
`ctx.Done()`/`ticker.C`, build a timeout `Outcome` on expiry), which invites a
single `pollUntil(ctx, req, step)` helper.

Detox 055 (finding B-03) flagged the shared skeleton and asked whether T-204
should consolidate. The three loops' *ordering* is a behavioral contract, pinned
byte-for-byte by `waitfor_pollorder_test.go` (T-103). Consolidation was evaluated
against that contract.

The orderings are not the same loop with different predicates:

- **`waitLifecycle` evaluates eagerly at the top of the loop, before any wait.**
  A condition already satisfied at entry returns met with **zero ticks**
  (`TestLifecycleChecksSuccessBeforeAnyWait`, poll interval one hour, timeout one
  hour — only a pre-wait check can return). The success check precedes the
  deadline check; the deadline check precedes the `select`. On timeout it reuses
  the status it already read (`timeoutOutcomeWithStatus`), avoiding a redundant
  `ReadStatus`.
- **`waitOutput` / `waitIdle` capture a baseline before the loop but evaluate
  success only inside `ticker.C`.** They always wait at least one poll interval
  before the first success return; within a tick, success is checked before the
  deadline (`TestOutputSuccessCheckedBeforeDeadline`, `TestIdleSuccessBeforeDeadline`).
- **Capture-error policy differs.** `waitOutput` `continue`s on a capture error
  (re-selects, swallowing until the deadline, then surfaces `capture_failed` with
  the error as a warning — `TestOutputCaptureErrorSurfacesAtDeadline`).
  `waitIdle` falls through a failed capture to its own deadline check.
  `waitLifecycle` reads pane *options*, not a capture, in its predicate.
- **Predicate shapes differ.** Lifecycle classifies a `Status` (comma-set turn
  matching, `cmd`/`exit=` matching, failure-kind derivation). Output does
  occurrence-count freshness over a `CapturePane` window plus `alreadyInTail`
  tracking. Idle tracks `last`/`stableSince` stability. None reduces to the
  others' signature.

## Decision

**Keep the three loops. Do not introduce `pollUntil`.**

A helper covering all three would need to parameterize: eager-vs-lazy first
evaluation, capture-error policy (`continue` vs fall-through vs none),
timeout-`Outcome` status source (held status vs re-read vs attached tail), and a
predicate signature that sometimes captures a pane, sometimes reads options, and
sometimes needs neither. That is four orthogonal flags plus a
lowest-common-denominator callback — an abstraction less readable than three
focused ~40-line loops, and one that puts the byte-identical ordering contract a
refactor slip away from breaking. The consolidation is net-negative.

The genuine duplication that *is* worth removing was already removed:
`timeoutOutcomeWithStatus`/`timeoutOutcome`, `stateFor`, `freshnessFor`, and the
`Basis` mapping are shared helpers all three loops call. What remains
loop-local is the ordering itself, which is the behavior — not boilerplate.

## Consequences

- The ordering contract lives in one place to read (`waitfor_pollorder_test.go`)
  and one place to change (the three loop bodies). A future agent that re-derives
  the "just extract pollUntil" idea finds this record and the pinned tests first.
- **Revisit trigger:** a fourth condition kind whose loop is a genuine clone of
  an existing one (same eagerness, same capture-error policy, same predicate
  shape) — then extract a helper for *that pair*, not a universal one. Absent a
  real clone, three loops stay three loops.
