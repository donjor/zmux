# zmux agent Pi extension testing prompt

Use this as a copy-paste prompt for a fresh **Pi** session. It tests the canonical
one-tool dispatcher and Bash guardrails against isolated `zzmux`.

````text
You are testing the branch-local zmux Pi extension in a fresh isolated Pi session.

## Mission

Run live E2E QA for the canonical `pi-zmux` package and its single `zmux_lite`
dispatcher. Write a structured evidence report under `.dump/test-prompts-report/`.
Do not edit source, commit, push, install live hooks, or touch the live `zmux`
profile.

This prompt authorizes one bounded real nested peer CLI through the dispatcher
when `zzmux` isolation is proven. A fake peer is not a substitute.

## Launch

```sh
./dev.sh zzmux
PI_ZMUX_BIN=zzmux pi -ne -e ./pi-zmux
```

`-ne` prevents the installed live extension from registering a duplicate tool.
`-e ./pi-zmux` loads this branch. If the active process is not proven to use
`zzmux`, run only read-only/source checks and mark mutating E2E checks blocked.

## Safety

1. Use `zmux_lite` for every zmux operation; bounded Bash is only for source reads and deterministic package gates.
2. Never use `PI_ZMUX_ALLOW=1` or `# pi-zmux: allow` to turn a blocked action into a pass.
3. Inspect but do not execute `zmux_reload`, `pi_reload`, or `pi_respawn`.
4. Keep every `focus` option false. Do not call `tab_focus` or `pane_focus`.
5. Use unique `$RUN_ID` names and clean up every created tab, pane, callback, and session.
6. Do not start hidden/background processes or raw tmux app-control commands.
7. A peer check must use a real visible CLI and atomic `peer_handoff`; report blocked if auth/CLI availability prevents it.

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

- exactly one registered Pi tool: `zmux_lite`;
- exactly 40 dispatcher operations;
- shared Bash-guard corpus parity passes;
- dispatcher contracts pass 40/40;
- schema estimate stays at or below 1,200 tokens;
- automatic injected runtime context is zero tokens;
- `/zmux status` still returns the full human diagnostic snapshot.

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

### 4. Input, state, and callback

- Use `type_text` to send `hello-$RUN_ID` to `worker`; prove `worker-saw:hello-$RUN_ID`.
- Apply `tab_state` and `tab_peer`, then read both with `tab_status`/`tab_inspect`.
- Start `callback_watch` for a future `worker-saw:callback-$RUN_ID` marker, then
  send the input. Confirm a top-level custom message with `customType:
  "pi-zmux-callback"`, fresh evidence basis, and no leaked callback handle.
- Use `callback_list`; cancel any remaining handle with `callback_cancel`.

### 5. Real peer

- Use `peer_ensure` to create one visible `peer-$RUN_ID` with an available
  authenticated CLI.
- Use atomic `peer_handoff`, with a future-output marker that does not appear
  verbatim in the outgoing prompt. Do not sequence `type_text` then
  `callback_watch`.
- Prove the answer with `tab_inspect` or `runtime_logs`. Output/idle is fallback
  evidence, not lifecycle readiness.

### 6. Panes and evidence

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
