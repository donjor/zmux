<!-- GENERATED FILE — edit agent-doctrine/ and run `make gen-doctrine`. -->

# Shared zmux doctrine

These are harness-neutral outcomes projected for the Claude skill. Claude-specific command sequences and hooks remain in the handwritten references.

### ZD-001 · Route work by lifecycle and visibility

- **Invariant:** Bounded non-interactive inspection may use the host shell; visible, interactive, privileged, persistent, or long-running work belongs in zmux.
- **Instruction:** Use zmux for visible, interactive, privileged, persistent, or long-running terminal work; keep only bounded non-interactive inspection in the host shell.
- **Claude mechanism:** CLI verbs and skill routing (instruction)
  - Caveat: none.
- **Verify:** `skills/zmux/test/doctor.mjs`, `pi-zmux/test/run.mjs`

### ZD-002 · Reuse stable targets and pin ambiguous sessions

- **Invariant:** Agents reuse stable descriptive targets and explicitly address the current session whenever a read or write could resolve elsewhere.
- **Instruction:** Inspect the roster before creating terminal state, reuse stable descriptive targets, and pin the current session whenever a name could resolve elsewhere.
- **Claude mechanism:** Session-pinned zmux CLI arguments (instruction)
  - Caveat: Write resolution is session-scoped, but reads still require explicit pinning.
- **Verify:** `skills/zmux/references/agent-peer.md`, `pi-zmux/test/dispatcher.mjs`

### ZD-003 · Do not move focus implicitly

- **Invariant:** Terminal focus moves only after an explicit user request.
- **Instruction:** Keep terminal focus unchanged unless the user explicitly asks to move it; visibility alone is not a focus request.
- **Claude mechanism:** Detached/default CLI flags (instruction)
  - Caveat: none.
- **Verify:** `skills/zmux/references/run-observe.md`, `pi-zmux/test/dispatcher.mjs`

### ZD-004 · Own runtime lifecycle by stable name

- **Invariant:** A server or watcher is started once in a stable target, inspected before replacement, restarted in place when requested, and stopped explicitly.
- **Instruction:** Use one stable named runtime: inspect existing output before starting another copy, restart in place when requested, and stop it explicitly.
- **Claude mechanism:** zmux run/watch/send lifecycle (instruction)
  - Caveat: Readiness is output evidence, not durable health.
- **Verify:** `skills/zmux/references/run-observe.md`, `pi-zmux/test/dispatcher.mjs`

### ZD-005 · Route manual input through a visible shared terminal

- **Invariant:** Sudo, SSH, password prompts, REPLs, database shells, and similar manual input never run as hidden host-shell jobs.
- **Instruction:** Run manual-input commands in one visible stable admin or remote target, keep focus unchanged by default, and wait only with bounded lifecycle or prompt evidence.
- **Claude mechanism:** Visible tab plus type/watch/status CLI sequence (instruction)
  - Caveat: none.
- **Verify:** `skills/zmux/references/run-observe.md`, `pi-zmux/test/dispatcher.mjs`

### ZD-006 · Use bounded first-class evidence

- **Invariant:** Completion and readiness are proven by fresh lifecycle or output evidence, never polling, elapsed time, process existence, or echoed prompt text.
- **Instruction:** Use bounded first-class lifecycle or future-output evidence; register callbacks before expected events and never add a post-hoc blind wait when lifecycle or callback evidence exists. Blind waiting is the last resort: try 10 seconds, inspect and reassess whether the expectation or mechanism is wrong, then escalate only to 30 and 60 seconds with the same reassessment. Do not poll, sleep as proof, treat process liveness as completion, or accept a marker already present in the prompt tail.
- **Claude mechanism:** zmux wait/status/watch with fresh baselines (instruction)
  - Caveat: Idle is a fallback for uninstrumented programs, not lifecycle truth.
- **Verify:** `skills/zmux/references/run-observe.md`, `pi-zmux/test/dispatcher.mjs`

### ZD-007 · Target panes structurally and capture minimal evidence

- **Invariant:** Pane mutations use an inspected raw pane id; terminal evidence is captured in the least invasive useful form.
- **Instruction:** Inspect joined panes before pane mutation, use the returned raw pane id, preserve literal-vs-submit input semantics, and prefer text or ANSI evidence when an image is unnecessary.
- **Claude mechanism:** pane list/open/send/resize/snapshot CLI verbs (instruction)
  - Caveat: none.
- **Verify:** `skills/zmux/references/cli-catalog.md`, `pi-zmux/test/dispatcher.mjs`

### ZD-008 · Drive visible peers through fresh lifecycle

- **Invariant:** A peer is a visible interactive CLI; prompts are submitted only after inspection and completion is fresh lifecycle truth when instrumentation exists.
- **Instruction:** Reuse one visible interactive peer, inspect its composer before sending, mark it running, and accept completion only from a newer ready lifecycle generation; use output or idle only for uninstrumented fallback.
- **Claude mechanism:** peer ensure/type/wait/status/inspect sequence (instruction)
  - Caveat: The peer skill owns when and which peer to select.
- **Verify:** `skills/zmux/references/agent-peer.md`, `pi-zmux/test/dispatcher.mjs`

### ZD-009 · Keep peers interactive and clean their lifecycle

- **Invariant:** Peers never use headless print mode and are consumed, parked with a reason, or killed when no concrete next checkpoint exists.
- **Instruction:** Launch peers as visible interactive CLIs, never print/headless one-shots; after consuming the answer, retain only for a concrete next checkpoint and otherwise clean up the tab and lifecycle state.
- **Claude mechanism:** Launch profiles plus tab peer state/kill (guard)
  - Caveat: Prompt scope, not OS sandboxing, defines a read-only review.
- **Verify:** `skills/zmux/references/agent-peer.md`, `skills/zmux/test/doctor.mjs`

### ZD-010 · Fail closed and clean exact owned state

- **Invariant:** A missing or ambiguous target is reported without invention, and teardown removes only exact test/task-owned objects.
- **Instruction:** When a target is missing or ambiguous, report the exact failure and stop rather than creating a substitute; clean only exact state owned by the task and prove the final roster.
- **Claude mechanism:** Resolver failures plus explicit kill/session cleanup (instruction)
  - Caveat: none.
- **Verify:** `skills/zmux/references/run-observe.md`, `agent-doctrine/harnesses/claude/host-flow.md`

### ZD-011 · Make remote mutation legible

- **Invariant:** Remote/admin work reuses one stable host target and states the decoded intended mutation before execution.
- **Instruction:** Reuse one stable admin or remote-host target, avoid numbered tab sprawl, decode opaque payloads, and state the intended host mutation before changing remote configuration.
- **Claude mechanism:** Skill doctrine and guard warnings (guard)
  - Caveat: none.
- **Verify:** `skills/zmux/SKILL.md`, `pi-zmux/test/dispatcher.mjs`

### ZD-012 · Share one scratch lane for bounded commands

- **Invariant:** Bounded one-shot commands that exit on their own share a single reused scratch tab instead of minting an ad-hoc tab per command.
- **Instruction:** Run bounded checks (typecheck, test, lint, build, one-shot scripts) through the shared scratch lane; reserve a named tab for durable runtimes or work you must keep addressable.
- **Claude mechanism:** zmux scratch / unnamed zmux run scratch-default (instruction)
  - Caveat: The scratch lane is reused, not immortal — an idle scratch tab is still reaped and reminted on the next bounded run.
- **Verify:** `skills/zmux/references/guard-and-tab-states.md`, `skills/zmux/references/run-observe.md`
