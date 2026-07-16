# Decisions

Durable point-in-time architecture and product decisions.

| Decision | Summary |
|----------|---------|
| [0001-go-rewrite-architecture-refactor.md](0001-go-rewrite-architecture-refactor.md) | Go rewrite package boundaries, thin launcher, and interface seams. |
| [0002-session-state-priming-no-pre-action-snapshot.md](0002-session-state-priming-no-pre-action-snapshot.md) | Session priming stays at start/compact + live `run -n` resolution; no per-Bash pre-action snapshot until evidence demands it. |
| [0003-pi-extension-compact-dispatcher.md](0003-pi-extension-compact-dispatcher.md) | Promote the validated one-tool dispatcher while retaining stable cockpit guardrails. |
| [0004-pi-zmux-state-on-demand.md](0004-pi-zmux-state-on-demand.md) | Resolve live zmux state through dispatcher operations instead of automatic prompt injection. |
| [0005-waitfor-three-poll-loops-not-consolidated.md](0005-waitfor-three-poll-loops-not-consolidated.md) | Keep waitfor's three poll loops; a universal `pollUntil` would need four flags and threaten the byte-identical ordering contract. |
