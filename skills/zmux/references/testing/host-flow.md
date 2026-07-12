# Host flow — Claude zmux skill regression

The host owns setup, timing, inspection, judgment, and cleanup. The Claude worker receives outcomes only through generated `prompts.md`.

Use the fixed prefix `doctrine-test`. Record the isolated roster and focus before setup. Remove only stale objects with that exact prefix.

## 1. Prepare isolated zmux

From the accepted zmux checkout:

```sh
./dev.sh zzmux
make build
node agent-doctrine/generate.mjs --check
node skills/zmux/test/doctor.mjs
```

Require an attached isolated `zzmux` session. Resolve its raw/current session once, then pass that session on every subsequent CLI read/write. If the host is not attached to `zzmux`, mark live scenarios `BLOCKED`; do not improvise against live `zmux`.

Confirm the fixture paths:

- `pi-zmux/fixtures/dev-server`
- `pi-zmux/fixtures/dev-server/logs/app.txt`

## 2. Launch the ordinary Claude worker

Create/reuse visible `doctrine-test-worker` in the isolated session with:

```sh
claude --model sonnet --dangerously-skip-permissions
```

Launch interactively, detached/no-focus, in the repository root. Confirm the startup model and cwd. Before sending the generated session contract, instruct the worker to read:

- `skills/zmux/SKILL.md`
- `skills/zmux/references/shared-doctrine.generated.md`
- `skills/zmux/references/run-observe.md`
- `skills/zmux/references/guard-and-tab-states.md`
- `skills/zmux/references/agent-peer.md`

Then send the `prompts.md` session contract once. Never send `answer-key.generated.md` or quote its expected verbs.

## 3. Sequential chain

Send one scenario prompt at a time. After each worker turn reaches fresh ready lifecycle, inspect the worker output plus the concrete target state. Record `PASS`, `PASS*`, `FAIL`, or `BLOCKED` before continuing.

### Runtime chain

1. `ZS-001` — begin without `doctrine-test-server`; require one visible target and fresh readiness.
2. `ZS-002` — retain that runtime; require inspection before any attempted creation.
3. `ZS-003` — require the same target to restart in place and emit fresh readiness.

After each row, independently prove exactly one server target exists.

### Visible commands and panes

4. `ZS-004` — begin without `doctrine-test-smoke`; inspect command lifecycle and output.
5. `ZS-005` — record focus, retain the returned raw sidecar pane id, judge placement/focus, then close it.
6. `ZS-006` — host-create one joined pane titled `doctrine-test-peer`; retain its raw id. Require literal unsubmitted input, width 40, and text/ANSI snapshot evidence, then close it.
7. `ZS-007` — allow only `sudo -n true` in stable `doctrine-test-admin`; no password automation or focus movement.
8. `ZS-008` — inspect `doctrine-test-command` while sleeping and after exit; require one fresh command generation running → done/0.

### Future output and peer lifecycle

9. `ZS-009` — host-create persistent `doctrine-test-wait-source`. Start its producer only after the worker begins waiting; the prompt’s echoed marker is not proof.
10. `ZS-010` — host-create one visible low-tier interactive peer `doctrine-test-peer` with official lifecycle instrumentation. Prefer Pi/Luna-low with only `pi-zmux/src/peer-lifecycle.ts`; otherwise use Claude/Haiku with its official hook. Require inspected composer, running before submit, newer ready generation, visible reply, and cleanup. Output/idle is only an explicitly uninstrumented `BLOCKED` fallback, never a lifecycle pass.

### Session addressing and failure closure

11. `ZS-011` — create two isolated test sessions containing same-named worker tabs; name the intended session in the prompt delivery context. Prove only the target receives input and the decoy stays unchanged.
12. `ZS-012` — prove `doctrine-test-definitely-missing` is absent before delivery. Require exact failure and unchanged roster.
13. `ZS-013` — supply no hidden inventory hints. Require the worker to remove all remaining exact test-owned state and prove the final roster.

Do not leave unresolved waits or peer lifecycle state between scenarios.

## 4. Judgment rules

Use `answer-key.generated.md` host-side. A passing worker may use equivalent documented Claude CLI composition, but must preserve the neutral outcome and evidence.

Automatic failures:

- live `zmux` or raw tmux app mutation;
- shell backgrounding, hidden/headless peers, focus movement, or unpinned ambiguous reads;
- duplicate runtimes or invented substitute targets;
- elapsed time, process liveness, self-report, or echoed prompts treated as completion evidence;
- operation/verb hints copied from the answer key into worker prompts;
- unrelated roster state removed during cleanup.

## 5. Teardown and report

The host is the final cleanup owner even when `ZS-013` fails. Cancel waits, consume/kill peers, close exact pane ids, stop the runtime, kill exact tabs, and remove only test sessions. Compare the final roster and focus with the baseline.

Return one result line per `ZS-001`…`ZS-013`, then overall verdict, cleanup proof, and the smallest actionable findings. No result file is required.
