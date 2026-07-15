<!-- GENERATED FILE — edit agent-doctrine/ and run `make gen-doctrine`. -->

# Agent doctrine capability matrix

| Rule | Outcome | Harness | Enforcement | Mechanism | Caveat / divergence |
|---|---|---|---|---|---|
| ZD-001 | Route work by lifecycle and visibility | claude | instruction | CLI verbs and skill routing | — |
| ZD-001 | Route work by lifecycle and visibility | pi | guard | One typed zmux dispatcher plus Bash guard | The Bash guard classifies clear unsafe routes; ambiguous commands still rely on instruction. |
| ZD-002 | Reuse stable targets and pin ambiguous sessions | claude | instruction | Session-pinned zmux CLI arguments | Write resolution is session-scoped, but reads still require explicit pinning. |
| ZD-002 | Reuse stable targets and pin ambiguous sessions | pi | typed-operation | sessions/tabs/current plus options.session | The dispatcher cannot infer which same-named target the user intended. |
| ZD-003 | Do not move focus implicitly | claude | instruction | Detached/default CLI flags | — |
| ZD-003 | Do not move focus implicitly | pi | typed-operation | focus options default false | Explicit tab_focus and pane_focus remain available for direct user requests. |
| ZD-004 | Own runtime lifecycle by stable name | claude | instruction | zmux run/watch/send lifecycle | Readiness is output evidence, not durable health. |
| ZD-004 | Own runtime lifecycle by stable name | pi | composite | runtime_ensure/runtime_logs/runtime_stop composite | Configured runtimes require a trusted project before commands are loaded. |
| ZD-005 | Route manual input through a visible shared terminal | claude | instruction | Visible tab plus type/watch/status CLI sequence | — |
| ZD-005 | Route manual input through a visible shared terminal | pi | composite | interactive_type composite | A password prompt may return needsUserInput rather than moving focus automatically. |
| ZD-006 | Use bounded first-class evidence | claude | instruction | zmux wait/status/watch with fresh baselines | Idle is a fallback for uninstrumented programs, not lifecycle truth. |
| ZD-006 | Use bounded first-class evidence | pi | typed-operation | wait/callback_watch plus structured command and turn state | deliverAs=nextTurn cannot trigger a continuation; use steer or followUp when triggerTurn is true. A timeout is diagnostic: before escalating, inspect current lifecycle and ask whether the event already happened, the evidence channel is stale, or the chosen mechanism is wrong. |
| ZD-007 | Target panes structurally and capture minimal evidence | claude | instruction | pane list/open/send/resize/snapshot CLI verbs | — |
| ZD-007 | Target panes structurally and capture minimal evidence | pi | typed-operation | panes and pane_* operations plus snapshot | terminal_current diagnoses desktop attachment and may be unsupported. |
| ZD-008 | Drive visible peers through fresh lifecycle | claude | instruction | peer ensure/type/wait/status/inspect sequence | The peer skill owns when and which peer to select. |
| ZD-008 | Drive visible peers through fresh lifecycle | pi | composite | peer_ensure plus atomic peer_handoff | The peer skill still owns selection, roles, and engagement policy. |
| ZD-009 | Keep peers interactive and clean their lifecycle | claude | guard | Launch profiles plus tab peer state/kill | Prompt scope, not OS sandboxing, defines a read-only review. |
| ZD-009 | Keep peers interactive and clean their lifecycle | pi | guard | Headless launch rejection plus tab_peer/tab_kill | Immediate teardown may omit an intermediate consumed state. |
| ZD-010 | Fail closed and clean exact owned state | claude | instruction | Resolver failures plus explicit kill/session cleanup | — |
| ZD-010 | Fail closed and clean exact owned state | pi | typed-operation | Typed target resolution plus tab_kill/session_kill | — |
| ZD-011 | Make remote mutation legible | claude | guard | Skill doctrine and guard warnings | — |
| ZD-011 | Make remote mutation legible | pi | guard | Dispatcher safety warnings and interactive routing | Warnings do not infer whether a decoded mutation is authorized. |
