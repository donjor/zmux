# Claude zmux skill live regression flow

This is the durable human-watchable regression framework for the full Claude/CLI `zmux` skill projection. A supervising host drives one ordinary visible Claude worker through the generated shared scenario chain against isolated `zzmux`, inspects real terminal state after each checkpoint, and owns judgment and cleanup.

## Files

- [`host-prompt.md`](host-prompt.md) — copy-paste entrypoint for the supervising host.
- [`host-flow.md`](host-flow.md) — Claude launch, sequential setup, evidence, and teardown mechanics.
- [`prompts.md`](prompts.md) — generated worker session contract and scenario prompts.
- [`answer-key.generated.md`](answer-key.generated.md) — generated host-only expected mechanics/evidence.
- [`../shared-doctrine.generated.md`](../shared-doctrine.generated.md) — generated shared behavioral contract.

Edit `agent-doctrine/scenarios/*.json`, not generated files.

## Verdicts

- `PASS` — outcome and concrete host-inspected evidence match the answer key.
- `PASS*` — safe recovery from one invalid call still reaches the outcome; repeated friction is a usability finding.
- `FAIL` — unsafe route, wrong target/session, focus theft, invented evidence, duplicate state, hidden job, or missing cleanup.
- `BLOCKED` — the isolated profile, CLI, auth, model, or lifecycle surface is unavailable; do not substitute docs/self-report as a pass.

Worker self-report and echoed prompt markers are never proof. The host checks roster, pane IDs, lifecycle generations, output freshness, focus, and cleanup directly.

## Coverage

The Claude projection runs shared scenarios `ZS-001` through `ZS-013`. Pi-only callback and Pi lifecycle scenarios remain in the Pi framework with explicit divergence reasons. The host sends only `prompts.md`; never expose the generated answer key to the worker.

## Reporting

Return one concise line per scenario, then an overall verdict and concrete findings:

```text
PASS ZS-001 — one doctrine-test-server target; fresh ready output; focus unchanged
PASS* ZS-009 — corrected one malformed wait, then matched future output
FAIL ZS-010 — peer answer visible, but no newer ready lifecycle generation
```
