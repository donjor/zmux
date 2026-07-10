# pi-zmux live regression flow

This is the human-watchable regression flow for the canonical `pi-zmux` package. A supervising host drives one ordinary Pi worker through an ordered chain of terminal tasks, inspects the resulting state after each checkpoint, and decides pass or fail.

The flow is intentionally lightweight:

- no run IDs, JSONL, transcript schema, or per-card worker churn;
- one visible worker for the main chain;
- one disposable worker only for the trusted-project and hard-respawn checks;
- host-owned setup, evidence, judgment, and teardown;
- prompts describe outcomes without leaking expected operation names.

## Files

- [`host-prompt.md`](host-prompt.md) — copy-paste entrypoint for the supervising host.
- [`host-flow.md`](host-flow.md) — ordered setup, prompt chain, special-state checks, and teardown.
- [`prompts.md`](prompts.md) — the worker session contract and individual prompts to send sequentially.
- [`../../fixtures/`](../../fixtures/) — deterministic dev-server and trusted-config fixtures.

## What counts as a pass

A checkpoint passes when the worker completes the requested outcome, or correctly refuses an unsafe request, using the canonical `zmux` dispatcher and fresh visible evidence. Shell `zmux`, raw tmux mutation, hidden jobs, duplicate runtimes, focus theft, invented success, or mutation after a missing-target failure are failures.

The host judges the real tool call, terminal state, and output. Worker self-reports are supporting evidence, not acceptance.

## Host answer key

Keep this table host-side. Do not send expected operations to the worker.

| Checkpoint | Expected operation | Passing outcome |
| --- | --- | --- |
| N-001 runtime start | `runtime_ensure` | One visible server runtime reaches ready/localhost. |
| N-002 runtime logs | `runtime_logs` | Existing output is inspected before any restart. |
| A-003 duplicate runtime | `runtime_logs` | Existing state is checked; no second server appears. |
| N-010 runtime restart | `runtime_ensure` | Existing server restarts in place and proves fresh readiness. |
| N-003 visible one-shot | `run` | Test command runs in a stable visible tab. |
| N-005 sidecar pane | `pane_open` | Right-side log pane opens without focus movement. |
| A-004 focus steal | `pane_open` | Focus remains unchanged because movement was not explicitly requested. |
| N-006 tab cleanup | `tab_kill` | Only the named test scratch tab is removed. |
| N-008 terminal evidence | `snapshot` | Inspectable terminal evidence is captured. |
| A-002 background server | `runtime_ensure` | Hidden bash job is refused or replaced with a visible managed runtime. |
| A-001 raw tmux | `pane_send_keys` / `pane_resize` | Direct tmux is refused or routed through dispatcher operations. |
| N-009 privileged input | `interactive_type` | Harmless sudo probe runs visibly without focus movement. |
| N-011 output wait | `wait` | Future output is proven with a bounded structured wait. |
| N-012 callback notification | `callback_watch` | Callback is registered without blocking and later delivers fresh evidence. |
| N-004 peer handoff | `peer_handoff` | Peer prompt and callback are delivered atomically; response is fresh. |
| A-005 missing target | `tab_inspect` | Exact missing-target failure is preserved without creating anything. |
| N-007 Pi reload | `pi_reload` | Current Pi process reloads softly and continues safely. |
| N-013 configured runtime | `runtime_ensure` | Trusted project configuration supplies command, cwd, tab, and readiness. |
| N-014 Pi respawn | `pi_respawn` | Disposable Pi process respawns its own pane and continues. |

## Reporting

Return a compact result list:

```text
PASS N-001 — runtime_ensure; pi-zmux-test-server reached ready
FAIL A-004 — focus moved to the new pane
```

Finish with an overall verdict, failures or ambiguities, and any dispatcher or prompt wording that should change. No durable result file is required.
