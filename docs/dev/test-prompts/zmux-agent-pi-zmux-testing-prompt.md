# zmux agent Pi extension testing prompt

Use this as a copy-paste prompt for a fresh **Pi** session. It tests the canonical
one-tool dispatcher and Bash guardrails against isolated `zzmux`.

````text
You are testing the branch-local zmux Pi extension in a fresh isolated Pi session.

## Mission

Run live E2E QA for the canonical `pi-zmux` package and its single `zmux`
dispatcher. Write a structured evidence report under `.dump/test-prompts-report/`.
Do not edit source, commit, push, install live hooks, or touch the live `zmux`
profile.

This prompt authorizes four bounded real nested peer CLIs through the dispatcher
when `zzmux` isolation is proven: Pi, Claude, Codex, and Agy. A fake peer is not
a substitute. Use the weakest available model and lowest reasoning effort for
these lifecycle-only probes.

## Launch

```sh
./dev.sh zzmux
PI_ZMUX_BIN=zzmux pi -ne -e ./pi-zmux
```

`-ne` prevents the installed live extension from registering a duplicate tool.
`-e ./pi-zmux` loads this branch. If the active process is not proven to use
`zzmux`, run only read-only/source checks and mark mutating E2E checks blocked.

## Safety

1. Use `zmux` for every zmux operation; bounded Bash is only for source reads and deterministic package gates.
2. Never use `PI_ZMUX_ALLOW=1` or `# pi-zmux: allow` to turn a blocked action into a pass.
3. Inspect but do not execute `zmux_reload`, `pi_reload`, or `pi_respawn`.
4. Keep every `focus` option false. Do not call `tab_focus` or `pane_focus`.
5. Use unique `$RUN_ID` names and clean up every created tab, pane, callback, and session.
6. Do not start hidden/background processes or raw tmux app-control commands.
7. Peer checks must use real visible Pi, Claude, Codex, and Agy CLIs with atomic `peer_handoff`; report an individual row blocked if its auth/model path is unavailable.

## Read first

- `docs/domains/pi-zmux-extension.md`
- `docs/dev/agent-grounding.md`
- `skills/zmux/SKILL.md`
- `skills/zmux/references/run-observe.md`
- `skills/zmux/references/guard-and-tab-states.md`
- `skills/zmux/references/agent-peer.md`
- `pi-zmux/src/index.ts`
- `pi-zmux/src/dispatcher.ts`
- `pi-zmux/src/classify.ts`
- `pi-zmux/src/config.ts`
- `pi-zmux/test/run.mjs`
- `pi-zmux/test/dispatcher.mjs`

Do not rely on model memory; cite docs/source in the report.

## Deterministic gates

```sh
npm --prefix pi-zmux run typecheck
npm --prefix pi-zmux test
make test-agent-surfaces
```

Required results:

- exactly one registered Pi tool: `zmux`;
- exactly 40 dispatcher operations;
- shared Bash-guard corpus parity passes;
- dispatcher contracts pass 40/40;
- schema estimate stays at or below 1,200 tokens;
- automatic injected runtime context is zero tokens;
- `/zmux status` still returns the full human diagnostic snapshot;
- completed tool boxes show one consolidated zmux card, while a deliberately
  delayed foreground wait updates in place with phase and remaining time;
- scheduled callbacks show one aggregate above-tasks widget line that clears on
  completion, cancellation, session replacement, and shutdown.

## Source and guard checks

Confirm:

- `src/index.ts` registers the dispatcher without a `before_agent_start`
  context hook while retaining Bash policy, glyph lifecycle, `/zmux`, and
  reload/respawn continuations;
- trusted project runtime config is ignored when project trust is false;
- runtime/background Bash routes to `operation=runtime_ensure`;
- sudo/SSH/REPL Bash routes to `operation=interactive_type`;
- direct `zmux` and raw tmux app-control route to equivalent dispatcher operations;
- headless peer print mode (`pi -p`, `claude --print`, etc.) is rejected;
- focus stays false unless a human explicitly requests it.

## Isolated E2E

Set a unique `$RUN_ID`. Record every tool call and result.

### 1. Context and session

- Call `current`; prove `PI_ZMUX_BIN=zzmux` or mark the remaining E2E blocked.
- Call `sessions` and `tabs` to identify the isolated workspace/session.
- Use `session_run` to create `worker-$RUN_ID`, tab `worker`, with a bounded loop
  that prints `worker-ready`, accepts input, prints `worker-saw:<input>`, then exits.
- Use `runtime_logs` or `tab_inspect` with explicit `options.session` to prove `worker-ready`.

### 2. Commands and remote safety

- Use `run` in the worker session to execute `echo pi-run-ok` in stable tab `scratch`.
- Inspect it with `tab_status` and `tab_inspect`.
- Exercise the harmless remote-warning path: `run` a `printf` containing an
  encoded remote payload token in tab `remote-example2`. Confirm the result warns
  about numbered remote-admin tab names/sprawl and opaque remote mutation payloads.

### 3. Runtime reuse

- Use `runtime_ensure` for a benign `dev` runtime that prints `ready-service` then sleeps.
- Use `runtime_logs` with `options.waitFor: "ready-service"`.
- Call `runtime_ensure` again and prove it reuses the named runtime rather than creating a duplicate.
- Stop it with `runtime_stop`.

### 4. Command lifecycle, input, state, and callback

- Run `sleep 3; printf 'COMMAND_LIFECYCLE_DONE\\n'` in a visible test tab. Use
  `tab_status`/`tab_inspect` during and after the sleep to prove one fresh
  `cmdSeq` transitions from `cmdState=running` to `cmdState=done` with exit 0.
  Output or process liveness alone is not lifecycle evidence.
- Use `type_text` to send `hello-$RUN_ID` to `worker`; prove `worker-saw:hello-$RUN_ID`.
- Apply `tab_state` and `tab_peer`, then read both with `tab_status`/`tab_inspect`.
- Start `callback_watch` for a future `worker-saw:callback-$RUN_ID` marker.
  Before sending the input, confirm the scheduled tool card settles while an
  aggregate Pi-zmux activity line remains visible above tasks and counts down.
  Then send the input and confirm the widget clears and a compact top-level message with
  `customType: "pi-zmux-callback"` reports fresh evidence without leaking a
  callback handle.
- Start and cancel one additional held callback; prove cancellation clears the
  widget without delivering a completion message. Use `callback_list` and
  `callback_cancel` for cleanup.

### 5. Real peer lifecycle matrix

Create one visible peer per CLI through `peer_ensure`. Keep each interactive and
max-permission, but prompt it only to identify its CLI/model in one line:

```text
Pi      openai-codex/gpt-5.6-luna · thinking low · load pi-zmux/src/peer-lifecycle.ts
Claude  haiku
Codex   gpt-5.4-mini · model_reasoning_effort=low
Agy     Gemini 3.5 Flash (Low)
```

- Test Pi, Claude, Codex, and Agy sequentially; do not overlap callbacks.
- For each, use atomic `peer_handoff` without an output marker. Confirm it marks
  the peer running before submission, advances `turnSeq`, reaches fresh
  `turn:ready`, and delivers a follow-up that triggers the host after completion.
  On one instrumented peer, use a deliberately short timeout and a prompt that
  remains active beyond it; confirm no terminal `unproven` message is delivered,
  the above-tasks activity remains, and a replacement wait eventually reports
  the real completion. Do not sequence `type_text` then `callback_watch`.
- Prove each answer with `tab_inspect`. Output/idle-only completion is a lifecycle
  failure, not a pass. Mark only unavailable CLI/model/auth rows blocked.

### 6. Panes and evidence

- Create a throwaway finite command tab with `run`, `options.focus:false`, and
  `options.waitForExit:false`. Do not issue a second `callback_watch`. Confirm
  `run` returns immediately, the current tab does not change, the card remains
  scheduled, an aggregate line above tasks tracks `command done`, and a follow-up
  later delivers shell-lifecycle completion evidence before the widget clears.
- Create one harmless command expected not to return, detach it with
  `options.trackCompletion:false`, and confirm no automatic callback/widget is
  armed; stop and clean it explicitly. Do not set `options.state`; shell hooks
  own lifecycle.
- Create a throwaway `side` tab with `run`, then use `tab_place` to join and
  restore/clean it without focus.
- Exercise `pane_open`, `panes`, `pane_resize` with `options.axis: "auto"`, and
  `pane_close` only on throwaway panes.
- Capture text-only evidence with `snapshot`; call `terminal_current` and accept
  an explicit unsupported result when no attached desktop metadata exists.

### 7. Cleanup

- Remove created tabs with `tab_kill` and explicit `options.session`.
- Remove `worker-$RUN_ID` with `session_kill`.
- Prove cleanup with `tabs` and `sessions`.

## Report

Write:

1. environment and launch command;
2. branch/commit and dispatcher/schema/context measurements;
3. deterministic gate results;
4. Bash guard matrix;
5. each E2E step as PASS/FAIL/BLOCKED with exact evidence;
6. cleanup proof;
7. concrete findings with file paths and operation names.

Do not count source inspection as live E2E. Do not count a marker visible only in
an echoed prompt as future-output evidence.
````
