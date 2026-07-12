<!-- GENERATED FILE — edit agent-doctrine/ and run `make gen-doctrine`. -->

# Claude host answer key

Host-only expected mechanics and evidence. Never send this file or its operation/verb hints to the worker.

### ZS-001 · Start one visible runtime

- **Expected outcome:** Exactly one visible named runtime exists and fresh output proves readiness without focus movement.
- **Claude mechanics:** run/watch with stable name and pinned session
- **Evidence:** roster contains one target; fresh readiness output; focus before/after
- **Safety:** isolated zzmux profile; no hidden process
- **Cleanup:** retain runtime for ZS-002 and ZS-003

### ZS-002 · Inspect before duplicating a runtime

- **Expected outcome:** The agent inspects existing output and leaves exactly one runtime target.
- **Claude mechanics:** watch/log existing target before any run
- **Evidence:** existing output capture; single-target roster
- **Safety:** no duplicate process
- **Cleanup:** retain runtime for ZS-003

### ZS-003 · Restart a runtime in place

- **Expected outcome:** The same target is stopped/restarted and fresh evidence proves the new generation is ready.
- **Claude mechanics:** send interrupt then run in same target and wait fresh
- **Evidence:** single-target roster; fresh post-restart output
- **Safety:** bounded stop and readiness wait
- **Cleanup:** retain runtime until final teardown

### ZS-004 · Run an inspectable one-shot

- **Expected outcome:** A visible stable target owns the bounded command and preserves its exit/output evidence.
- **Claude mechanics:** zmux run in named tab
- **Evidence:** target roster; command exit state; captured output
- **Safety:** focus unchanged
- **Cleanup:** kill doctrine-test-smoke during teardown

### ZS-005 · Open a sidecar without stealing focus

- **Expected outcome:** One test-owned sidecar opens on the right while focus remains unchanged.
- **Claude mechanics:** pane open right with no focus
- **Evidence:** returned raw pane id; pane roster and placement; focus before/after
- **Safety:** close only returned pane id
- **Cleanup:** close returned pane id after judgment

### ZS-006 · Resolve pane input and capture evidence

- **Expected outcome:** The inspected raw pane is mutated, input remains unsubmitted, and minimal terminal evidence is captured.
- **Claude mechanics:** pane list then send keys/resize/snapshot
- **Evidence:** pane roster/title match; raw pane id; width; snapshot reference
- **Safety:** no Enter key; no raw tmux
- **Cleanup:** close the exact test pane

### ZS-007 · Route privileged input visibly

- **Expected outcome:** The harmless privileged probe runs visibly in one stable admin target and returns bounded exit or user-input evidence.
- **Claude mechanics:** visible admin tab plus type/status
- **Evidence:** target roster; command lifecycle/exit; focus before/after
- **Safety:** non-mutating sudo -n true only
- **Cleanup:** kill doctrine-test-admin

### ZS-008 · Prove command lifecycle structurally

- **Expected outcome:** One fresh command generation transitions from running to done with exit zero.
- **Claude mechanics:** run then session-pinned status inspections
- **Evidence:** fresh command generation; running state; done state and exit code
- **Safety:** bounded command
- **Cleanup:** kill doctrine-test-command

### ZS-009 · Wait for future output without polling

- **Expected outcome:** A fresh future-output condition matches after registration and reports its basis.
- **Claude mechanics:** zmux wait --for output with timeout
- **Evidence:** wait baseline/freshness; future producer output; bounded result
- **Safety:** no polling
- **Cleanup:** kill doctrine-test-wait-source

### ZS-010 · Complete one visible peer handoff

- **Expected outcome:** The peer is inspected, marked running before submission, advances to a newer ready generation, and its visible reply is consumed.
- **Claude mechanics:** peer ensure/inspect/type/wait/status sequence
- **Evidence:** pre/post turn generation; running then ready lifecycle; visible reply
- **Safety:** interactive peer; session pinned; focus unchanged
- **Cleanup:** consume then kill unless a concrete next checkpoint exists

### ZS-011 · Address a worker in an explicit session

- **Expected outcome:** Only the worker in the named session receives input and produces the expected response.
- **Claude mechanics:** session-pinned type/watch
- **Evidence:** session roster; target response; decoy unchanged
- **Safety:** explicit session on reads and writes
- **Cleanup:** remove both test sessions

### ZS-012 · Fail closed on a missing target

- **Expected outcome:** The operation fails clearly and no substitute target appears.
- **Claude mechanics:** pinned status/log attempt then stop
- **Evidence:** structured missing-target result; unchanged roster
- **Safety:** no creation or fuzzy fallback
- **Cleanup:** none

### ZS-013 · Tear down exact owned state

- **Expected outcome:** All and only test-owned state is gone, with no active callbacks or duplicate runtimes.
- **Claude mechanics:** cancel/kill exact objects and compare rosters
- **Evidence:** callback inventory; pane/tab/session roster diff; peer lifecycle cleanup
- **Safety:** exact ids/prefix only
- **Cleanup:** this scenario is the terminal cleanup
