# Pi extension: resolve live zmux state on demand

## Context

ADR 0003 promoted the compact dispatcher by grafting it into the stable
`pi-zmux` package. That integration deliberately retained the stable package's
`before_agent_start` runtime-context injection: binary/version, policy/config,
current pane, configured runtimes, visible tabs, and a routing reminder. The
bounded worst-case fixture measured approximately 430 tokens in addition to the
approximately 995-token dispatcher schema.

The injection was inherited infrastructure, not a fix for compact-dispatcher
failures. The isolated compact candidate registered only the dispatcher, had no
context-injection hook, and passed the complete Terra/medium matrix 19/19. Its
evidence-driven fixes were operation behavior and targeted tool guidance:
readiness handling, atomic peer handoff, callback freshness, lifecycle
continuations, validation, and focus safety.

Pi runs `before_agent_start` for each agent run and carries the resulting system
prompt through provider calls in that run. The snapshot is paid even when zmux
is irrelevant and may become stale after external terminal activity or the
first mutating operation. Persisting or watching live state would not make it
model-visible without another prompt/message/tool result, and would add
session/socket lifecycle and invalidation complexity.

## Decision

Do not inject live zmux state into the model system prompt.

Keep one model-visible `zmux` dispatcher. Resolve state on demand:

- agents call `current`, `tabs`, `sessions`, `panes`, `tab_status`, or
  `tab_inspect` when a request actually needs broad inspection;
- explicit-target operations execute directly;
- `current` resolves the complete row for the caller's `$TMUX_PANE`, independent
  of which window the attached client is viewing;
- operations that require implicit live state resolve only that state inside
  their implementation;
- tool results return the relevant observation or postcondition for the next
  model call.

Keep the full `buildContext()` diagnostic behind the human `/zmux status`
command. Do not add a watcher, general cross-call cache, persistent state
message, or second model-visible state tool. Cheap live CLI reads remain the
correctness source. A narrow cache may be considered later only from measured
latency evidence, never as authoritative terminal state.

This decision narrows ADR 0003: preserve the stable context diagnostic
infrastructure, but not its automatic model-prompt injection.

## Consequences

- Automatic pi-zmux runtime-context overhead becomes zero tokens.
- The fixed model-visible surface remains one tool at approximately 995 schema
  tokens.
- State/result tokens are incurred only on turns that use zmux.
- External terminal changes are observed by the next relevant live operation
  rather than reconciled through cache invalidation.
- `/zmux status`, Bash policy, trusted config, lifecycle glyphs, callbacks, and
  reload/respawn continuations remain available.
- Production tests must reject registration of a `before_agent_start` handler,
  keep `/zmux status` diagnostic coverage, and preserve all 40 operation
  contracts.
