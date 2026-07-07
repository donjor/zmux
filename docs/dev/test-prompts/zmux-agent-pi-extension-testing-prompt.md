# zmux agent Pi extension testing prompt

Use this file as a copy-paste prompt for a fresh **Pi** session. It tests the branch-local Pi extension's typed `zmux_*` tools and bash guardrails against an isolated `zzmux` profile. It complements `zmux-agent-skill-testing-prompt.md`, which tests shared skill/CLI doctrine.

````text
You are testing the zmux Pi extension in a fresh isolated Pi session.

## Mission

Run full live E2E QA for the branch-local Pi extension's `zmux_*` tools against the isolated `zzmux` profile. Write a structured report with evidence to `.dump/test-prompts-report/`; creating that report file is allowed and required. This prompt is the explicit authorization for **one bounded nested peer CLI** through the typed Pi tools when isolation is confirmed, so the test proves real peer spawning/reuse, typed prompt delivery, fresh output observation, lifecycle metadata, worker/session targeting, and cleanup. Do not edit source files, commit, push, install live-profile hooks, run destructive live-profile commands, or touch the live `zmux` profile.

## Preferred launch

Best signal comes from a Pi process launched against this repo's extension and isolated `zzmux` profile:

```sh
./dev.sh zzmux
PI_ZMUX_BIN=zzmux pi -ne -e ./pi-extension
```

`./dev.sh zzmux` installs only the edge binary/profile; it does not refresh active Pi tools, shared skill mirrors, or global package settings. `-ne` disables globally discovered extensions so the already-installed live `zmux/pi-extension` cannot register duplicate `zmux_*` tools. The explicit `-e ./pi-extension` still loads this branch's extension for the fresh Pi process, and `PI_ZMUX_BIN=zzmux` routes those tools to the isolated binary/profile.

If you are already inside Pi and cannot relaunch with `PI_ZMUX_BIN=zzmux`, still run read-only/source checks and deterministic tests, but do not run mutating typed-tool smoke tests against the live `zmux` profile. Mark those smoke checks `BLOCKED: active Pi is not isolated on zzmux`.

## Safety rules

1. Prefer native Pi `zmux_*` tools for zmux operations. Use bounded shell only for source reads, grep inventory, and package test commands.
2. Do not bypass the bash guard with `PI_ZMUX_ALLOW=1` or `# pi-zmux: allow` unless the test is specifically about documenting that bypass exists. Do not use the bypass to turn a blocked action into a passing check.
3. Do not invoke `zmux_reload`, `zmux_pi_reload`, or `zmux_pi_respawn`; inspect their descriptions/docs only. These affect live config or the active Pi pane.
4. Do not call focus-moving tools (`zmux_tab_focus`, `zmux_pane_focus`, or any `focus: true`) unless the human explicitly asks to move terminal focus.
5. Peer/worker checks must use typed tools, not docs-only validation. This prompt authorizes exactly one bounded real nested `claude`/`codex`/`pi`/`agy` peer when `PI_ZMUX_BIN=zzmux` isolation is confirmed. If no peer CLI is available or auth blocks it, report `BLOCKED`; do not substitute a fake peer and call it pass.
6. Worker checks must prove command completion/interaction, not just detached-session spawn.
7. Create only uniquely named test sessions/tabs and clean them up.

## Read first

Read these files before judging behavior:

- `docs/dev/test-prompts/README.md`
- `docs/domains/pi-zmux-extension.md`
- `skills/zmux/SKILL.md`
- `skills/zmux/references/run-observe.md`
- `skills/zmux/references/guard-and-tab-states.md`
- `skills/zmux/references/agent-peer.md`
- `skills/zmux/references/agent-worker.md`
- `docs/dev/agent-grounding.md`
- `pi-extension/src/tools/index.ts`
- `pi-extension/src/tools/core.ts`
- `pi-extension/src/tools/tabs.ts`
- `pi-extension/src/tools/panes.ts`
- `pi-extension/src/tools/runtimes.ts`
- `pi-extension/src/classify.ts`
- `pi-extension/test/run.mjs`

Do not rely on model memory for expected behavior; cite docs/source in the report.

## Deterministic gates

Run:

```sh
cd pi-extension && npm run typecheck && npm test
make test-agent-surfaces
```

If dependencies are missing or the command cannot run, report `BLOCKED` with stdout/stderr summary.

## Live tool inventory gate

Enumerate the currently expected tool names from source:

```sh
grep -Rho 'name: "zmux_[a-z_]*"' pi-extension/src/tools pi-extension/src/zmux.ts 2>/dev/null \
  | sed 's/name: "//;s/"//' \
  | sort -u
```

Also enumerate the **active Pi tool registry** from the tools visible in your session. If you cannot programmatically list loaded tools, manually list the available `zmux_*` tool names from the tool registry/system prompt.

Current expected source inventory as of this prompt:

- `zmux_callback`
- `zmux_current`
- `zmux_interactive_type`
- `zmux_log`
- `zmux_pane_close`
- `zmux_pane_focus`
- `zmux_pane_list`
- `zmux_pane_open`
- `zmux_pane_resize`
- `zmux_pane_send_keys`
- `zmux_pane_type`
- `zmux_peer_ensure`
- `zmux_peer_handoff`
- `zmux_pi_reload`
- `zmux_pi_respawn`
- `zmux_reload`
- `zmux_run`
- `zmux_runtime_ensure`
- `zmux_runtime_logs`
- `zmux_runtime_stop`
- `zmux_send_keys`
- `zmux_session_kill`
- `zmux_session_run`
- `zmux_sessions`
- `zmux_snapshot`
- `zmux_tab_focus`
- `zmux_tab_inspect`
- `zmux_tab_kill`
- `zmux_tab_label`
- `zmux_tab_move`
- `zmux_tab_peer`
- `zmux_tab_place`
- `zmux_tabs`
- `zmux_tab_state`
- `zmux_tab_status`
- `zmux_terminal_current`
- `zmux_type`

Report any mismatch:

- source tool missing from active Pi registry → `FAIL` if this session was launched with the repo extension; `BLOCKED` if active Pi is not using this branch/package.
- active `zmux_*` tool not represented in the checklist below → `FAIL: uncovered tool surface`.

## Isolated typed-tool E2E checks

Run these only when `PI_ZMUX_BIN=zzmux` isolation is confirmed by `zmux_current` or by the launch environment. Use unique names, for example `pi-ext-<timestamp>`. This section is not satisfied by source inspection or command inventory; it must use the native Pi `zmux_*` tools.

Before mutating anything:

1. Call `zmux_current` and confirm the binary is `zzmux` or the environment clearly routes to `zzmux`. If not, mark all mutating E2E checks `BLOCKED: active Pi is not isolated on zzmux`.
2. Call `zmux_sessions` and/or `zmux_current` to identify the current workspace. If no current workspace is available, mark worker/session E2E `BLOCKED` instead of inventing one.
3. Set one unique run id in your notes at the start, e.g. `RUN_ID=pi-ext-<timestamp>-<short-random>`. Use the same value for every tab/session; changing it mid-test is a prompt failure.

Required typed-tool path:

1. **Worker session birth and targeting**
   - Use `zmux_session_run` to create `worker-$RUN_ID` with tab `worker` in the current workspace. Command: `bash -lc 'sleep 3; echo worker-ready; while IFS= read -r line; do sleep 3; echo "worker-saw:$line"; done'`.
   - Use `zmux_runtime_logs` (name/tab `worker`) or `zmux_tab_inspect` with session `<workspace>/worker-$RUN_ID` to wait for `worker-ready`. The wait-backed tools should report an evidence `basis` such as `outputRegex` or `idleFallback`; if a wait times out but the captured tail already contains the marker, record already-in-tail evidence and use `zmux_tab_inspect`/logs for proof; do not retry blindly.
   - Use `zmux_tabs` and `zmux_pane_list` with session `<workspace>/worker-$RUN_ID` to prove the branch's `workspace/session` targeting path works through the typed wrapper.

2. **Reviewable command, status, inspect, and logs**
   - Use `zmux_run` in the worker session to run `echo pi-run-ok` in a stable `scratch` tab.
   - Use `zmux_tab_status` and `zmux_tab_inspect` on `scratch` with session `<workspace>/worker-$RUN_ID` and include the evidence.
   - Use `zmux_log` start/tail/stop on `scratch` or `worker` in the worker session. Use `zmux_log status` only as the global recording view; do not pass `session` or `tab` to `status`.
   - Verify the remote-admin/tab-sprawl warning path without touching a real host: call `zmux_run` with a harmless `printf` command containing an encoded remote payload token and a numbered remote tab name such as `remote-example2`, then report whether the tool result warns to reuse the unsuffixed stable tab and decode/explain opaque payloads before remote mutation.

3. **Runtime lifecycle**
   - Use `zmux_runtime_ensure` in the worker session for a benign long-lived runtime tab `dev`: `sh -c 'sleep 3; echo ready-service; sleep 3600'`.
   - Use `zmux_runtime_logs` (name/tab `dev`) with `waitFor: "ready-service"`.
   - Use `zmux_runtime_stop` and confirm the runtime stops or becomes idle.

4. **Typed input and fresh output**
   - Use `zmux_type` on tab `worker` with session `<workspace>/worker-$RUN_ID` to send `hello-worker-$RUN_ID`.
   - Use `zmux_runtime_logs` (name/tab `worker`) or `zmux_tab_inspect` to prove `worker-saw:hello-worker-$RUN_ID` appears.
   - Use `zmux_send_keys` only on throwaway tabs/panes if a raw key path is needed; never send keys to live user tabs.

5. **Lifecycle metadata through session labels**
   - Use `zmux_tab_state` on `worker` with session `<workspace>/worker-$RUN_ID` and a message like `state via workspace/session`.
   - Use `zmux_tab_peer` on `worker` with the same session and report `zmux_tab_status` / `zmux_tab_inspect` evidence.

6. **Callback and peer-handoff notifications**
   - Use `zmux_callback` with action `watch` on `worker` in session `<workspace>/worker-$RUN_ID`, `waitFor: "worker-saw:callback-$RUN_ID"`, and a short timeout. Then use `zmux_type` to send `callback-$RUN_ID`; the Pi session should receive a callback message with output evidence and `basis: outputRegex` before the next model call because default delivery is `steer`. In JSONL, delivered callback messages are top-level `custom_message` records with `customType: "pi-zmux-callback"`, not wrapped `message` records. Report whether the callback message arrived or mark `FAIL/BLOCKED` with evidence.
   - Use `zmux_callback` action `list` before/after callback completion to prove callback handles are visible and not leaked; the list text should include active callbacks and recent completions. If a callback remains active after the proof, use action `cancel` and report it.
   - If a real peer is available in the next step, use `zmux_peer_handoff` for one prompt instead of raw `zmux_type` when possible. It should type the prompt and deliver a wait-backed callback/handoff with an explicit basis when the peer output matches or goes idle.

7. **Real peer composite**
   - Determine whether a supported peer CLI is available with bounded shell (`command -v claude`, then `codex`, `pi`, `agy`). This shell probe is allowed because Pi has no typed command-inventory tool.
   - If available, use `zmux_peer_ensure` to create one peer tab `peer-$RUN_ID` in the isolated current session. Prefer command `claude --dangerously-skip-permissions` when `claude` exists; otherwise use the available CLI. Use a readiness regex such as `Claude Code|Codex|bypass permissions|❯|›`.
   - Use `zmux_type` to send: `Reply with prefix PI_PEER_E2E_ immediately followed by this run id, and no other text. Run id: $RUN_ID`. If you pass `waitForTurnState: "ready"` and it returns `unproven` while output proves the peer answered, record that as the expected uninstrumented-peer fallback, not a fake failure; nested CLIs do not necessarily have stop-hook readiness installed.
   - Use `zmux_runtime_logs` or `zmux_tab_inspect` to wait for and prove the concatenated marker `PI_PEER_E2E_$RUN_ID` appears after the prompt has actually submitted. Do not count a marker that appears only in the typed prompt/composer. If the peer cannot authenticate/respond, report `BLOCKED` with output evidence.
   - Use `zmux_tab_peer` / `zmux_tab_status` / `zmux_tab_inspect` to manually mark and inspect peer readiness/consumption after output proof when automatic lifecycle readiness is unproven.

8. **Pane, placement, and evidence**
   - First create a throwaway `side` tab in the worker session with `zmux_run` (for example `sleep 3; echo side-ready; sleep 60`) and wait for `side-ready`.
   - Use `zmux_tab_place` to join `side` in the worker session as a pane and then promote it back/full or clean it up. This specifically tests placement `--session` through the typed wrapper.
   - Use `zmux_pane_open`, `zmux_pane_list`, `zmux_pane_resize`, and `zmux_pane_close` only on throwaway panes. For `zmux_pane_resize`, first use default `axis: "auto"` and confirm geometry changes; if testing a specific split direction, pass `axis: "width"` or `axis: "height"` explicitly. If the tool cannot safely target the worker session, use a throwaway tab in the current isolated session and state that scope.
   - Use `zmux_snapshot` with `noPng: true` and `zmux_terminal_current` when safe, and include artifact/evidence references. If `zmux_terminal_current` reports `unsupported` because the test is headless/no attached client metadata exists, record that explicit unsupported result as environment evidence rather than a wrong-window failure.

9. **Session kill cleanup proof**
   - Create a second throwaway session `kill-$RUN_ID` with `zmux_session_run` using a `sleep 3; echo kill-ready; sleep 60` marker command, then wait for the marker.
   - Remove it using `zmux_session_kill` with `<workspace>/kill-$RUN_ID`.
   - Prove it is gone with `zmux_tabs`/`zmux_sessions`; this specifically tests the branch's `session kill workspace/session` fix.

Always clean up `worker-$RUN_ID`, `kill-$RUN_ID`, `peer-$RUN_ID`, and any throwaway tabs/panes. Use `zmux_tab_kill` with its `session` parameter for throwaway tabs in non-current sessions. Do not invoke focus-moving, reload, respawn, or live kill/move tools. For focus/reload/respawn, verify docs/schema and report `PASS (inspect-only)` or a finding.

## Coverage checklist

For each item, report `PASS`, `FAIL`, or `BLOCKED`, with evidence.

### Tool registration and docs

- Active tool list matches source inventory, or mismatch is explained.
- Each registered `zmux_*` tool has a clear description and maps to skill doctrine.
- New tools are represented in `docs/domains/pi-zmux-extension.md` and relevant skill docs.
- `zmux_tab_inspect`, `zmux_peer_ensure`, `zmux_callback`, `zmux_peer_handoff`, and `zmux_type` peer wait fields are documented.

### Bash guardrails

- Runtime/background commands are nudged or blocked toward `zmux_runtime_ensure` / `zmux_run`.
- Interactive/manual/sudo/SSH/REPL commands route to `zmux_interactive_type`.
- Direct raw tmux app-control paths are blocked when typed tools exist.
- Direct zmux CLI usage is nudged toward typed Pi tools where appropriate.
- Bounded reads/tests remain allowed in normal shell.
- Explicit bypass is documented but not used to fake success.

### Runtime and reviewable command tools

- `zmux_run` handles reviewable one-shots in a stable named tab.
- `zmux_run` warns on numbered remote-admin tab names and on opaque encoded/obfuscated remote payloads, so this failure class is covered by deterministic tests and fresh-session QA.
- `zmux_runtime_ensure/logs/stop` handle persistent runtimes without duplicate hidden jobs.
- Output waits return evidence and do not require hand-written sleeps/sentinels.
- Logs can wait briefly for regex/idle output when documented.

### Sessions, tabs, panes, and evidence

- Session tools support explicit targeting and do not steal focus unexpectedly.
- Tab status/inspect/state/peer/place/label/move/kill/focus behavior is documented; mutating/focus/destructive variants are tested only on throwaway resources or inspect-only.
- Pane tools are focus-safe by default and tested only on throwaway panes.
- Snapshot/terminal-current behavior is safe and evidence-oriented.

### Interactive and Pi lifecycle tools

- `zmux_interactive_type` reports manual prompts/password needs without automating secrets.
- `zmux_reload` is only for zmux config/key/theme changes.
- `zmux_pi_reload` is the soft path after Pi extension/skill/prompt/theme changes but is not invoked during this test.
- `zmux_pi_respawn` is documented as a destructive hard fallback and is not invoked during this test.

### Callback and peer composites

- `zmux_callback` proves long-running completion handoff without agent-side sleeps/poll loops and leaves no leaked callback handles.
- `zmux_peer_handoff` types the prompt and delivers a wait-backed callback/handoff with explicit basis, or reports `BLOCKED` when no real peer CLI/auth is available.
- `zmux_peer_ensure` returns core peer ensure spawn/reuse/readiness/status/output evidence or an explicit unproven/blocked result.
- `zmux_type` peer wait uses fresh generation (`turnSeq`, with `turnAt` as supporting evidence), not stale `ready` state; uninstrumented nested CLIs may return `unproven` even when output proves a response.
- `zmux_tab_inspect` is useful for one-call status+output diagnosis.
- Unavailable real peer CLIs are `BLOCKED`, not silent passes.

## Final report format

Write the report to `.dump/test-prompts-report/zmux-agent-pi-extension-testing-report-<date-or-run-id>.md` (create the directory if needed), then return a short final message with the report path and verdict. The report file must contain exactly these sections:

1. `Verdict` — one of `PASS`, `PASS WITH FINDINGS`, `FAIL`, or `BLOCKED`.
2. `Environment` — repo path, branch/commit, Pi launch mode, `zmux_current` summary, `PI_ZMUX_BIN` status.
3. `Tool inventory` — source list count, active Pi list count, mismatches/uncovered tools.
4. `Commands/tools run` — concise list with pass/fail/block notes.
5. `Coverage matrix` — grouped by the checklist above, with docs/source/live evidence.
6. `Findings` — concrete defects or drift, with file paths/tool names. If worker interaction, real peer marker output, pane open/resize/close proof, placement `session` targeting, or `zmux_session_kill <workspace/session>` proof is missing, report it here as `FAIL` or `BLOCKED`. Do not count a marker that appears only in the typed prompt/composer.
7. `Cleanup` — sessions/tabs/panes created and confirmation they were removed, or what remains.
8. `Recommended follow-ups` — prioritized, bounded actions.
````
