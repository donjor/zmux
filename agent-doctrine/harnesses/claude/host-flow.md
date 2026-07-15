# Host flow — Claude natural-prompt zmux campaign

The supervising host owns lane selection, setup, timing, perturbation, inspection, judgment, and teardown. The visible Claude worker receives only exact natural prompt bodies from `claude-prompts`.

Use an exact test prefix. Record the approved profile's roster and focus before setup; remove only stale objects with that prefix.

## 1. Confirm and prepare the execution lane

Agree and record one concrete lane before launch:

- native `zmux` or isolated `zzmux`;
- installed behavior or checkout-local binary;
- installed Claude skill source;
- any explicitly approved install/sync mutation.

Prefer isolated `zzmux` for branch-local CLI behavior. Claude cannot load a checkout skill through a direct extension flag, so prove the intended skill source rather than describing it to the worker. If the intended code cannot be loaded safely, mark affected rows `BLOCKED infrastructure-blocked`.

Create a disposable worker sandbox outside the checkout containing only selected-row subject files: `README.md`, `docs/{architecture,ROADMAP}.md`, and `pi-zmux/fixtures/**`. Never expose `agent-doctrine/`, `.dump/`, harness files, answer keys, or migration/failure ledgers. A worker search that discovers its prompt or host mechanics invalidates the row.

Bind the canonical `zmux` command to the selected profile in the launch environment. In an isolated lane, an exported shell function or equivalent launcher must route `zmux …` to the real `zzmux` executable while preserving argv0 as `zzmux`; a symlink named `zmux` is insufficient because profile selection is argv0-based. Prove the binding before the first row without telling the worker. If it crosses profiles, classify the row `harness-invalid` and relaunch cleanly.

Run applicable checks:

```sh
# Edge lane only when approved:
./dev.sh zzmux

make build
make check-doctrine
node skills/zmux/test/doctor.mjs
```

Require an attached session in the selected profile. Resolve its raw session once and pin it on every host read/write. Never fall back to another profile. Confirm the fixture dev server and log paths.

## 2. Launch one ordinary Claude worker

Create one visible interactive Claude worker in the recorded session and disposable worker sandbox root, detached/no-focus. Confirm model, sandbox cwd, intended installed skill, profile binding, focus, and editor state through host inspection.

Do not instruct it to read doctrine/mechanics files. Do not send a session contract, profile hint, operation list, evidence rule, or cleanup procedure. The normal installed skill is the product under test.

Choose work host-side:

```sh
node agent-doctrine/generate.mjs --render claude-answer-key --tier atomic
node agent-doctrine/generate.mjs --render claude-answer-key --ids ZS-012,ZS-018
```

Render each selected row with the matching command:

```sh
node agent-doctrine/generate.mjs --render claude-prompts --ids ZS-001
```

A one-turn row is exact stdout. For multi-turn output, send only bodies inside each `BEGIN/END HOST TURN` boundary, in order.

## Scenario roster (shared)

Every shared scenario applies to the Claude harness. Select rows from this roster; never invent cases outside it.

Atomic:

1. `ZS-001` — run one finite visible command; require a truthful running → done/exit lifecycle.
2. `ZS-002` — start one long-running runtime and require fresh readiness before use.
3. `ZS-003` — inspect an existing runtime before creating; reuse it rather than duplicating.
4. `ZS-004` — restart the same runtime in place and emit fresh readiness.
5. `ZS-005` — stop the exact existing runtime and confirm it is gone.
6. `ZS-006` — open one visible sidecar; retain its raw pane id, judge placement/focus, then close it.
7. `ZS-007` — type literal unsubmitted input into a visible pane and prove it with a snapshot.
8. `ZS-008` — run one visible admin check with no password automation or focus movement.
9. `ZS-009` — notify on output that only appears after the wait begins; the echoed marker is not proof.
10. `ZS-010` — ask one visible instrumented peer for a bounded answer; submit once and return its useful reply.
11. `ZS-011` — address one same-named target explicitly; prove only the intended target receives input.

Workflow:

12. `ZS-012` — run a small development loop in visible tabs without sprawl.
13. `ZS-013` — delegate parallel reading to peers while the host stays available and focus holds.
14. `ZS-014` — reuse one peer for a natural follow-up without relaunching.
15. `ZS-015` — continue work in one explicit named session end to end.
16. `ZS-016` — complete sequential work and leave no residual test-owned tabs or sessions.

Resilience:

17. `ZS-017` — fail closed on a missing target; prove absence before delivery and keep the roster unchanged.
18. `ZS-018` — interrupt a joined wait without cancelling already-accepted work.
19. `ZS-019` — preserve accepted work when one launch in a batch fails.
20. `ZS-020` — report one peer process death truthfully rather than as a false pass.

## 3. Drive one selected row

1. Read the host answer key and establish the exact disposable baseline without telling the worker.
2. Record roster, target/session/pane/process identity, lifecycle/output state, and focus.
3. Send Prompt 1 exactly, without id/title or appended hints.
4. Send workflow follow-ups only at their natural checkpoint.
5. Inject resilience faults only from `Host perturbations`, after their declared boundary and entirely out of band.
6. Inspect real Claude tool/terminal state. Self-report, elapsed time, idle, and echoed markers are not completion proof.
7. Record all five lenses and preserve the first failure.
8. Perform exact cleanup and restore baseline before the next row; use a canary turn when delayed delivery is in scope.

Atomic rows never depend on prior rows. Use fresh disposable sessions when interruption, peer death, or ownership state could contaminate another case.

## 4. Judgment rules

- **outcome** — requested result or truthful failure is concrete.
- **orchestration** — smallest documented CLI composition, no raw tmux, shell backgrounding, hidden/headless peers, duplicate targets, ambiguous reads, or invented substitutes.
- **responsiveness** — host remains usable and focus stays unless requested.
- **presentation** — timing, useful output, and explanation are clear and non-contradictory.
- **cleanup** — exact test-owned state returns to baseline with no delayed traffic.

All five pass or the row fails. Safe recovery from an invalid command remains `FAIL orchestration`; there is no `PASS*`. Use `BLOCKED` only for unavailable required infrastructure.

## 5. Teardown and report

The host owns final cleanup after every result. Cancel exact waits, capture accepted peer answers, remove exact peers, close exact panes, stop runtimes, kill exact tabs/test sessions, and compare final roster/focus with baseline. Never remove unrelated state.

Report one line per selected row:

```text
PASS ZS-006 — outcome/orchestration/responsiveness/presentation/cleanup
FAIL ZS-010 [presentation] agent-routing — useful peer answer replaced by terminal chrome
BLOCKED ZS-018 [outcome] infrastructure-blocked — lifecycle-instrumented peer unavailable
```

Return overall verdict, exact prompt and first artifact for failures, adapter divergence findings, and per-row plus lane-final cleanup proof. No durable report file is required.
