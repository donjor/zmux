<!-- GENERATED FILE — edit agent-doctrine/ and run `make gen-doctrine`. -->

# Pi host answer key

Host-only expected mechanics and evidence. Never send this file or its operation/verb hints to the worker.

### ZS-001 · Start one visible runtime

- **Expected outcome:** Exactly one visible named runtime exists and fresh output proves readiness without focus movement.
- **Pi mechanics:** runtime_ensure with readiness and focus false
- **Evidence:** roster contains one target; fresh readiness output; focus before/after
- **Safety:** isolated zzmux profile; no hidden process
- **Cleanup:** retain runtime for ZS-002 and ZS-003

### ZS-002 · Inspect before duplicating a runtime

- **Expected outcome:** The agent inspects existing output and leaves exactly one runtime target.
- **Pi mechanics:** runtime_logs before runtime_ensure
- **Evidence:** existing output capture; single-target roster
- **Safety:** no duplicate process
- **Cleanup:** retain runtime for ZS-003

### ZS-003 · Restart a runtime in place

- **Expected outcome:** The same target is stopped/restarted and fresh evidence proves the new generation is ready.
- **Pi mechanics:** runtime_ensure with restart true
- **Evidence:** single-target roster; fresh post-restart output
- **Safety:** bounded stop and readiness wait
- **Cleanup:** retain runtime until final teardown

### ZS-004 · Run an inspectable one-shot

- **Expected outcome:** A visible stable target owns the bounded command and preserves its exit/output evidence.
- **Pi mechanics:** run operation in named tab
- **Evidence:** target roster; command exit state; captured output
- **Safety:** focus unchanged
- **Cleanup:** kill doctrine-test-smoke during teardown

### ZS-005 · Open a sidecar without stealing focus

- **Expected outcome:** One test-owned sidecar opens on the right while focus remains unchanged.
- **Pi mechanics:** pane_open direction right focus false
- **Evidence:** returned raw pane id; pane roster and placement; focus before/after
- **Safety:** close only returned pane id
- **Cleanup:** close returned pane id after judgment

### ZS-006 · Resolve pane input and capture evidence

- **Expected outcome:** The inspected raw pane is mutated, input remains unsubmitted, and minimal terminal evidence is captured.
- **Pi mechanics:** current then panes, pane_send_keys, pane_resize, snapshot
- **Evidence:** pane roster/title match; raw pane id; width; snapshot reference
- **Safety:** no Enter key; no raw tmux
- **Cleanup:** close the exact test pane

### ZS-007 · Route privileged input visibly

- **Expected outcome:** The harmless privileged probe runs visibly in one stable admin target and returns bounded exit or user-input evidence.
- **Pi mechanics:** interactive_type with waitForExit and focus false
- **Evidence:** target roster; command lifecycle/exit; focus before/after
- **Safety:** non-mutating sudo -n true only
- **Cleanup:** kill doctrine-test-admin

### ZS-008 · Prove command lifecycle structurally

- **Expected outcome:** One fresh command generation transitions from running to done with exit zero.
- **Pi mechanics:** run then tab_status/tab_inspect
- **Evidence:** fresh command generation; running state; done state and exit code
- **Safety:** bounded command
- **Cleanup:** kill doctrine-test-command

### ZS-009 · Wait for future output without polling

- **Expected outcome:** A fresh future-output condition matches after registration and reports its basis.
- **Pi mechanics:** wait operation with waitFor regex and timeoutSeconds
- **Evidence:** wait baseline/freshness; future producer output; bounded result
- **Safety:** no polling
- **Cleanup:** kill doctrine-test-wait-source

### ZS-010 · Complete one visible peer handoff

- **Expected outcome:** The peer is inspected, marked running before submission, advances to a newer ready generation, and its visible reply is consumed.
- **Pi mechanics:** peer_ensure then atomic peer_handoff and tab_inspect
- **Evidence:** pre/post turn generation; running then ready lifecycle; visible reply
- **Safety:** interactive peer; session pinned; focus unchanged
- **Cleanup:** consume then kill unless a concrete next checkpoint exists

### ZS-011 · Address a worker in an explicit session

- **Expected outcome:** Only the worker in the named session receives input and produces the expected response.
- **Pi mechanics:** type_text and tab_inspect with options.session
- **Evidence:** session roster; target response; decoy unchanged
- **Safety:** explicit session on reads and writes
- **Cleanup:** remove both test sessions

### ZS-012 · Fail closed on a missing target

- **Expected outcome:** The operation fails clearly and no substitute target appears.
- **Pi mechanics:** tab_status/tab_inspect then stop
- **Evidence:** structured missing-target result; unchanged roster
- **Safety:** no creation or fuzzy fallback
- **Cleanup:** none

### ZS-013 · Tear down exact owned state

- **Expected outcome:** All and only test-owned state is gone, with no active callbacks or duplicate runtimes.
- **Pi mechanics:** callback_cancel, runtime_stop, pane_close, tab_kill, session_kill, then inventory
- **Evidence:** callback inventory; pane/tab/session roster diff; peer lifecycle cleanup
- **Safety:** exact ids/prefix only
- **Cleanup:** this scenario is the terminal cleanup

### ZS-014 · Schedule a Pi callback without blocking

- **Expected outcome:** The callback schedules, remains visibly active, then delivers one compact fresh completion and clears its activity state.
- **Pi mechanics:** callback_watch with followUp/nextTurn semantics appropriate to triggerTurn
- **Evidence:** scheduled result; active footer; fresh callback message; empty callback list
- **Safety:** bounded timeout; no callback handle leakage
- **Cleanup:** cancel callback if still active; kill source tab
- **Divergence:** Claude skill mechanics have no in-process Pi follow-up delivery channel.

### ZS-015 · Resolve Pi lifecycle safely

- **Expected outcome:** The current Pi pane reloads and a continuation proves completion, or the operation fails closed without touching another pane.
- **Pi mechanics:** pi_reload with continuationPrompt and omitted target
- **Evidence:** resolved pane; scheduled lifecycle result; post-reload continuation
- **Safety:** disposable process; no explicit foreign target
- **Cleanup:** close disposable Pi tab
- **Divergence:** Claude does not host Pi extension reload/respawn operations.

### ZS-016 · Track detached Pi commands automatically

- **Expected outcome:** The finite run returns immediately, automatically tracks one fresh command generation, delivers a completion follow-up, and clears its activity; the explicit no-return opt-out arms no callback and is cleaned manually.
- **Pi mechanics:** run with focus false and waitForExit false; automatic completion callback; run with trackCompletion false; callback_list and exact tab cleanup
- **Evidence:** unchanged focus; automatic scheduled callback/activity; fresh cmd completion follow-up; no callback for opted-out command; final callback and tab roster
- **Safety:** visible targets; no duplicate callback_watch; trackCompletion false only for no-return work; exact cleanup
- **Cleanup:** cancel any unresolved automatic callback; stop and kill both exact tabs; prove baseline callback roster
- **Divergence:** Automatic in-process completion callbacks are specific to the Pi dispatcher; Claude uses explicit visible command observation.
