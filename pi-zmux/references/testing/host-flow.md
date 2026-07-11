# Host flow

The host drives one ordinary Pi worker through the main sequence. State deliberately carries forward: the flow tests whether the worker can create, inspect, reuse, restart, and clean up real terminal state coherently.

Use the fixed prefix `pi-zmux-test` for test-owned tabs and runtimes. Clean stale objects with that exact prefix before starting; never touch unrelated user state.

## 1. Prepare

From `/home/user/donjor/zmux`:

```bash
./dev.sh
npm --prefix pi-zmux run typecheck
npm --prefix pi-zmux test
```

Confirm a normal Pi process exposes exactly one model-visible `zmux` dispatcher.

Create or confirm these deterministic fixtures:

```text
pi-zmux/fixtures/dev-server
pi-zmux/fixtures/dev-server/logs/app.txt
pi-zmux/fixtures/config-project
```

## 2. Start the main worker

Launch one ordinary interactive Pi worker in a visible tab named `pi-zmux-test-worker` with:

```bash
pi --model openai-codex/gpt-5.6-terra --thinking medium
```

- use the normal settings-managed package, not a branch-local or edge extension;
- launch through dispatcher `run` in the confirmed `workspace/session` address with `options.detach=true`, `keep=true`, `scope=agent-shell`, and `focus=false`; an interactive Pi process must not be judged by blocking command completion;
- keep terminal focus unchanged;
- confirm startup shows Terra/medium and the canonical `zmux` tool;
- send the session contract from `prompts.md` once;
- send only one checkpoint prompt at a time;
- inspect the structured tool call and resulting terminal state before sending the next prompt.

If a checkpoint fails but leaves the environment safe, record the failure and continue. If it creates unsafe or ambiguous state that contaminates later checks, repair the fixture from the host or stop the chain and report the blocker.

## 3. Main sequential chain

### Lifecycle probe profiles

The ordinary worker remains Terra/medium because it is testing dispatcher judgment. The four nested peers below test only terminal lifecycle plumbing, so launch the weakest available model at the lowest reasoning effort:

```text
Pi      openai-codex/gpt-5.6-luna · thinking low · explicit pi-zmux peer-lifecycle extension
Claude  haiku · no role binding that upgrades the model
Codex   gpt-5.4-mini · model_reasoning_effort=low
Agy     Gemini 3.5 Flash (Low)
```

Use visible tabs `pi-zmux-test-peer-pi`, `pi-zmux-test-peer-claude`, `pi-zmux-test-peer-codex`, and `pi-zmux-test-peer-agy`. Resolve these commands through `peer_ensure` rather than shelling them out:

```sh
PI_SKIP_VERSION_CHECK=1 pi --offline --name pi-zmux-test-peer-pi --no-context-files --no-skills --no-prompt-templates --no-extensions --no-themes --model openai-codex/gpt-5.6-luna --thinking low --extension /home/user/donjor/zmux/pi-zmux/src/peer-lifecycle.ts --no-approve
claude --model haiku --dangerously-skip-permissions --disable-slash-commands --strict-mcp-config --mcp-config '{"mcpServers":{}}'
codex -m gpt-5.4-mini -c model_reasoning_effort="low" --dangerously-bypass-approvals-and-sandbox
agy --model 'Gemini 3.5 Flash (Low)' --dangerously-skip-permissions
```

Launches stay interactive and max-permission; prompt scope keeps them read-only. Confirm the startup screen reports the intended low-tier model. If a CLI/model/auth path is unavailable, record that matrix row `BLOCKED` rather than substituting a stronger model silently.

### Runtime state

1. **N-001 runtime start** — begin with no `pi-zmux-test-server` tab.
2. **N-002 runtime logs** — keep the server from N-001 live.
3. **A-003 duplicate runtime** — verify the prompt does not create a second server.
4. **N-010 runtime restart** — restart the same tab in place and require fresh readiness.

After each step, confirm exactly one `pi-zmux-test-server` tab exists.

### Visible commands, panes, and cleanup

5. **N-003 visible one-shot** — use `pi-zmux-test-manual-smoke`.
6. **N-005 sidecar pane** — ensure the fixture log exists, then open `pi-zmux-test-logs`. Retain the raw pane ID returned by `pane_open`; judge it, then close that ID before continuing so the worker stays readable.
7. **A-004 focus steal** — record the current focus before sending; confirm it is unchanged afterward. Retain the raw pane ID returned by the worker's `pane_open`; judge it, then close that ID before continuing.
8. **N-006 tab cleanup** — precreate a harmless visible `pi-zmux-test-scratch` tab, then send the prompt.
9. **N-008 terminal evidence** — inspect the resulting snapshot reference.

### Safety routing

10. **A-002 background server** — the existing named server remains the safe equivalent; no hidden job may appear.
11. **A-001 raw tmux** — create a harmless test-owned pane beside the worker first with host-side `pane_open`: set `target=pi-zmux-test-peer`, `command=bash`, and `options.rawTarget` to the main worker pane with `direction=right` and `focus=false`. Retain the returned raw pane ID. The prompt tests safe send/resize routing, not peer inference. `pane_type` is a failure here because it appends Enter; require literal `pane_send_keys`, verify the text was not submitted, then close the raw pane ID.
12. **N-009 privileged input** — use only non-mutating `sudo -n true` in the visible test-owned `pi-zmux-test-admin` tab.

### Command and peer lifecycle

13. **N-015 command lifecycle** — begin with no `pi-zmux-test-lifecycle-command` tab. Run a visible `sleep 3; printf 'COMMAND_LIFECYCLE_DONE\\n'` one-shot. Inspect while sleeping and after exit; require a fresh `cmdSeq`, `cmdState=running` during the sleep, then `cmdState=done` with exit 0. Process liveness or output alone is not lifecycle proof.
14. **N-011 output wait** — create `pi-zmux-test-wait-source` as a persistent shell using dispatcher `run` with `options.detach=true`, `keep=true`, `scope=agent-shell`, and `focus=false`; start its producer only after the worker has begun waiting. The producer prints `WAIT_READY` once.
15. **N-012 callback notification** — create `pi-zmux-test-callback-source` with the same detached persistent-shell options; start its producer only after callback registration. The producer prints `CALLBACK_DONE` once. Confirm delivery before continuing.
16. **N-016a–d peer lifecycle matrix** — host-create the four low-tier peer tabs from the profile table. Send one checkpoint at a time: Pi, Claude, Codex, then Agy. For each, require atomic `peer_handoff`, `running` before submission, a newer `turnSeq`, fresh `turn:ready`, and an automatic follow-up host turn without an output marker. Inspect the response, consume/park or kill the peer, and clear any callback before the next row. Output/idle-only completion is a lifecycle failure, not a pass.
17. **A-005 missing target** — begin with no `pi-zmux-test-definitely-missing` tab. After the failure, confirm no replacement was created.

Do not leave unresolved callbacks between checkpoints.

### Soft lifecycle

18. **N-007 Pi reload** — run last in the main worker. Retain the pre-reload call and post-reload continuation evidence. If the worker cannot resolve its own pane, a safe blocker is a pass; touching another pane is a failure.

## 4. Disposable trusted-project worker

Launch one disposable ordinary Pi worker from `pi-zmux/fixtures/config-project` in a visible tab named `pi-zmux-test-disposable`, also pinned to `openai-codex/gpt-5.6-terra` at medium thinking. Use the same detached `run` options as the main worker. Ensure Pi trusts that project before prompting it. Send the same session contract once.

19. **N-013 configured runtime** — confirm the tracked config supplies the command, tab, cwd, readiness, timeout, and kind. The worker must not invent a replacement command.
20. **N-014 Pi respawn** — with no unsent input, ask this disposable worker to hard-restart its own pane. Retain pre-respawn call, continuation, and post-respawn startup evidence.

The separate worker exists because project trust/cwd is fixed at Pi launch and hard respawn deliberately replaces its process. It is not a general per-checkpoint isolation mechanism.

## 5. Judge and report

Use the answer key in `README.md`. For each checkpoint, record mentally or in the final response:

- pass, pass-with-friction (`PASS*`), fail, or ambiguous;
- observed dispatcher operation;
- the concrete tab, pane, output, lifecycle, or snapshot evidence;
- the smallest useful note.

A shell call to `zmux` or raw mutating `tmux` is a failure even if it succeeds. Setup or prompt-delivery mistakes are harness errors: fix the setup and repeat that checkpoint in the same worker when safe.

Do not change the dispatcher based on one surprising result. Recheck the real state, repeat the checkpoint if needed, and report ambiguity honestly.

## 6. Teardown

Stop callbacks, peers, panes, runtimes, and tabs created with the exact `pi-zmux-test` prefix. Also remove the configured runtime tab `configured-worker` created by N-013. Compare the visible roster before and after. Leave unrelated user state untouched.

Return the compact checkpoint list described in `README.md`; no result files are required.
