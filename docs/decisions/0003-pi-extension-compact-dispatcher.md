# Pi extension: promote the compact dispatcher without dropping cockpit guardrails

## Context

The stable `pi-zmux` Pi extension exposes 37 model-visible tools whose schemas cost roughly 8,332 tokens. The promotion campaign tested a one-tool dispatcher with 40 operations at roughly 962 schema tokens. In fresh visible Terra/medium runs, the compact surface passed all 19 scenario cards; the stable surface passed 18/19 because it reproduced the known unsafe-focus behavior on A-004.

The experimental package intentionally isolated dispatcher behavior. Stable `pi-zmux` also owns non-tool infrastructure: Bash policy enforcement, trusted project config, runtime context injection, lifecycle glyphs, peer lifecycle, callback cleanup, and reload/respawn continuations.

## Decision

Promote the validated one-tool dispatcher into the canonical `pi-zmux` package, replacing the 37 registered model-visible tools. Preserve the stable package and its non-model-visible safety, context, lifecycle, continuation, and command infrastructure rather than replacing the whole extension with the experimental package.

Keep the `zmux` CLI as the behavior source of truth. The extension remains an adapter, not an alternate implementation of terminal/session semantics.

## Consequences

- The primary Pi tool surface becomes one dispatcher with the 40 Terra-validated operations.
- Stable guardrails remain promotion requirements even though they were outside the dispatcher-only A/B matrix.
- The dispatcher schema and injected runtime context must be measured separately. Hooks and lifecycle code do not consume model tokens unless they add tool schema or prompt text.
- Old typed-tool implementation modules may be removed only after dispatcher integration and full-package tests prove they are no longer runtime dependencies.
- Luna coverage remains deferred; the accepted promotion target is the complete Terra/medium matrix.
- Promotion is incomplete until package tests, shared guard-corpus parity, repository gates, managed snapshot sync, and a fresh normal-profile Pi smoke pass.

## Narrow supersession

ADR 0004 supersedes this decision only for automatic runtime-context injection.
The stable diagnostic builder and `/zmux status` command remain; live pane, tab,
runtime, and config state is no longer appended to every model system prompt.
All other preserved cockpit infrastructure remains in force.

## Promotion closure

Release v0.13.0 completed the promotion: the sole model-visible tool is named
`zmux`, the package version is 0.4.0, and the retired 37-tool registration modules
were removed after canonical dispatcher tests and managed-profile verification.
The experiment's separate package/name is no longer a runtime or source surface.
