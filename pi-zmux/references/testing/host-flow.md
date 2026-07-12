# Host flow — Pi zmux regression

The host owns setup, timing, inspection, judgment, and cleanup. The Pi worker receives outcomes only through generated `prompts.md`.

Use the fixed prefix `doctrine-test`. Record the isolated roster and focus before setup. Remove only stale objects with that exact prefix.

## 1. Prepare isolated package and profile

From the accepted zmux checkout:

```sh
./dev.sh zzmux
node agent-doctrine/generate.mjs --check
npm --prefix pi-zmux run typecheck
npm --prefix pi-zmux test
node skills/zmux/test/doctor.mjs
```

Require an attached isolated `zzmux` session. Confirm fixtures:

- `pi-zmux/fixtures/dev-server`
- `pi-zmux/fixtures/dev-server/logs/app.txt`
- `pi-zmux/fixtures/config-project`

## 2. Launch the ordinary Pi worker

Launch visible `doctrine-test-worker` in the isolated session and repository root with:

```sh
env PI_ZMUX_BIN=zzmux pi --model openai-codex/gpt-5.6-terra --thinking medium -ne -e ./pi-zmux
```

Use detached/no-focus launch mechanics and pin the isolated session. Confirm one `zmux` tool, Terra/medium, branch-local package, and correct cwd. Send the generated session contract once. Never send `answer-key.generated.md` or quote its operation hints.

## 3. Shared sequential chain

Send one scenario prompt at a time. Wait for fresh worker lifecycle and inspect the tool call plus concrete state before continuing.

1. `ZS-001` — one visible `doctrine-test-server`, fresh readiness, no focus movement.
2. `ZS-002` — inspect existing runtime before any duplicate action; roster remains one.
3. `ZS-003` — restart the same target in place and prove fresh readiness.
4. `ZS-004` — visible `doctrine-test-smoke` command with lifecycle/exit/output.
5. `ZS-005` — right sidecar, returned raw pane id, unchanged focus; host closes after judgment.
6. `ZS-006` — host creates titled joined test pane; require literal input, width 40, and text/ANSI snapshot; host closes exact id.
7. `ZS-007` — allow only visible `sudo -n true`; bounded exit or `needsUserInput`, no focus movement.
8. `ZS-008` — one fresh command generation transitions running → done/0 around the three-second sleep.
9. `ZS-009` — persistent source producer starts only after wait registration; echoed prompt marker is not evidence.
10. `ZS-010` — host creates visible low-tier Pi/Luna peer with only `pi-zmux/src/peer-lifecycle.ts`. Require `peer_ensure`/atomic handoff semantics, running before submit, newer ready generation, automatic follow-up, visible reply, and cleanup.
11. `ZS-011` — host creates target and decoy same-named workers in two isolated sessions; require explicit session targeting and unchanged decoy.
12. `ZS-012` — exact target absent; require structured failure and unchanged roster.
13. `ZS-013` — worker removes remaining exact test-owned state and proves baseline roster.

Do not leave unresolved callbacks or peer lifecycle state between rows.

## 4. Pi-only scenarios

Run these in controlled state after shared cleanup setup is re-established as needed.

14. `ZS-014` — create persistent callback source, register callback before producer output, confirm scheduled card settles, aggregate footer remains active, then compact fresh completion arrives and footer/list clear. Cancel on timeout.
15. `ZS-015` — in the disposable branch-local Pi with no unsent input, require soft reload of its own pane and continuation proof; a safe resolution blocker passes, touching another pane fails.
16. `ZS-016` — run one finite detached command with focus unchanged and no manual callback; prove automatic fresh completion and activity cleanup. Then run one harmless no-return command with `trackCompletion:false`, prove no callback was armed, and stop/kill both exact targets.

Adapter-local checks not duplicated in live prompts remain mandatory through package tests: trusted config, Bash guard, schema/rendering, non-TUI ticker suppression, session replacement cleanup, and hard respawn continuation.

## 5. Judgment and teardown

Use `answer-key.generated.md` host-side. Automatic failures include Bash `zmux`/raw tmux, hidden work, focus movement, duplicate runtimes, ambiguous unpinned reads, output/idle used as instrumented lifecycle truth, answer-key leakage, or unrelated cleanup.

The host is final cleanup owner even after `ZS-013`. Cancel callbacks, consume/kill peers, close exact panes, stop runtimes, kill exact tabs/sessions, and close disposable Pi. Compare final roster/focus to baseline.

Return one result line per `ZS-001`…`ZS-016`, overall verdict, cleanup proof, and smallest actionable findings. No result file is required.
