# Claude natural-prompt zmux campaign

This is the canonical human-watchable campaign for agent-driven `zmux` usage through the Claude/CLI skill projection. A supervising host establishes an isolated lane, gives an ordinary visible Claude worker exact natural user turns, injects resilience faults out of band, inspects real terminal state, and owns judgment and cleanup.

## Ownership

- [`host-prompt.md`](host-prompt.md) — supervising-host entrypoint and lane agreement.
- [`host-flow.md`](host-flow.md) — Claude launch, tier conductor, five-lens judgment, and teardown.
- `agent-doctrine/scenarios/shared/*.md` — portable natural jobs and harness-specific host answer keys.
- `node agent-doctrine/generate.mjs --render claude-prompts --ids <id>` — exact worker turn(s) on stdout.
- `node agent-doctrine/generate.mjs --render claude-answer-key --ids <id>` — host-only setup, verdict, evidence, cleanup, and mechanics.
- `agent-doctrine/migrations/2026-natural-campaign.json` — machine-checked legacy coverage disposition.
- [`../../rules/shared/`](../../rules/shared/) — portable behavioral doctrine.

Rendered prompts and answer keys are ephemeral stdout. Edit scenario Markdown and run `make check-doctrine`; never commit a rendered campaign copy.

## Three tiers

- `atomic` — one ordinary prompt and one primary job; independently resettable.
- `workflow` — natural follow-up turns proving jobs compose.
- `resilience` — ordinary turns plus a supervising-host interruption or failure.

Select `--tier atomic|workflow|resilience` or caller-ordered `--ids`. Claude runs shared `ZS-*` rows only; Pi-only `PZ-*` rows never enter Claude output.

A single selected turn renders as exact copy/paste input. Multi-turn/tier output uses `BEGIN/END HOST TURN` HTML comments as host navigation. Send only each enclosed body.

## Five hard verdict lenses

Every row records `outcome`, `orchestration`, `responsiveness`, `presentation`, and `cleanup` as `PASS | FAIL | BLOCKED`. All five must pass. There is no `PASS*`; corrected invalid composition still fails orchestration. `BLOCKED` is reserved for unavailable required infrastructure.

The worker receives only natural prompt bodies. Lane/profile setup, target fixtures, timing, fault injection, expected commands, evidence checks, and teardown remain host-side. Self-report and echoed prompt text are not proof.

## Isolation

The supervising host must prove which binary/profile and skill source the visible worker will use. Prefer isolated `zzmux` for branch-local behavior. Launch from an allowlisted disposable sandbox that excludes `agent-doctrine/` and every host answer source. Isolation comes from launch/profile mechanics, including binding the canonical `zmux` command to the selected profile, never an extra worker instruction. If the intended code cannot be loaded safely, mark affected rows `BLOCKED` rather than coaching or silently switching lanes.

## Reporting

Return one line per selected row, then overall verdict and exact cleanup proof:

```text
PASS ZS-006 — outcome/orchestration/responsiveness/presentation/cleanup
FAIL ZS-010 [orchestration] agent-routing — visible answer arrived after a redundant wait
BLOCKED ZS-018 [outcome] infrastructure-blocked — lifecycle-instrumented peer unavailable
```

Preserve the exact prompt and first artifact for each failure. Classify `product-defect | agent-routing | harness-invalid | infrastructure-blocked`.
