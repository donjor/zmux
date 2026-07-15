# Pi zmux extension

The repo owns a Pi extension plus the shared zmux agent skill. The skill teaches
terminal/session doctrine; the extension adds one compact `zmux` dispatcher,
on-demand runtime inspection, and bash guardrails so agents use visible
zmux-managed tabs instead of hidden shell jobs or raw tmux.

## Owned paths

- `agent-doctrine/**` — neutral shared rules/scenarios and deterministic generator.
- `docs/reference/agent-doctrine-matrix.generated.md` — generated capability/caveat review matrix.
- `pi-zmux/index.ts` — package entry for Pi extension loading.
- `pi-zmux/src/**` — dispatcher registration, on-demand diagnostics, bash classification, and zmux adapters.
- `pi-zmux/test/**`, `pi-zmux/package.json`, `pi-zmux/tsconfig.json` — TypeScript validation surface.
- `agent-doctrine/harnesses/claude/**` — handwritten harness launch, inspection, judgment, and teardown mechanics. The parallel `agent-doctrine/harnesses/pi/**` is deferred with the Pi extension reintegration and not present on this shared branch.
- `agent-doctrine/generate.mjs --render <artifact>` — stdout-only generated prompts and host answer keys for live maintainer runs.
- `pi-zmux/fixtures/**` — deterministic live fixtures.
- `skills/zmux/SKILL.md`, `skills/zmux/references/**`, `skills/zmux/hooks/**`, `skills/zmux/test/**` — Claude mechanics/hooks, the committed runtime doctrine projection, and the single doctrine doctor.
- `docs/dev/agent-grounding.md` — live `zzmux` grounding protocol for agents.
- `docs/dev/test-prompts/zmux-agent-*-testing-prompt.md` — prompt-driven exploratory QA for fresh supervised sessions testing the whole agent-facing skill/Pi surface.

## Invariants

- Long-running, interactive, sudo/password, watcher, server, and TUI commands belong in zmux tabs or panes, not the agent shell.
- Dispatcher operations are focus-safe by default. They move terminal focus only when the user explicitly asks or after a focused confirmation.
- Direct raw tmux app-control paths are blocked when an equivalent dispatcher operation exists.
- `zmux operation=pi_reload` is the soft path after Pi extension, skill, prompt, or theme changes; `operation=pi_respawn` is a destructive fallback.
- Persistent Pi process liveness is not a running signal. Only an active Pi turn should publish the running glyph.
- Project config containing commands is read only when Pi marks the project trusted.

## Reusable primitives

- `src/dispatcher.ts` owns the one-tool schema and its 40 operation mappings.
- `src/rendering.ts` owns the pending/partial/final native tool-card lifecycle, compact/expanded presentation, callback-message rendering, destination trees, lifecycle evidence, narrow-width wrapping, and sensitive-input redaction.
- `src/exec.ts` and `src/interactive.ts` are focused dispatcher adapters over the `zmux` CLI and tmux socket.
- `src/classify.ts` shares the guard vocabulary with the zmux skill and Claude hook tests.
- `src/zmux/**` retains context, lifecycle, and continuation primitives used outside the model-visible schema.
- `src/config.ts` loads trusted project config and configured runtimes.
- `src/reload-continuation.ts` and `src/respawn-continuation.ts` build safe continuation prompts for Pi lifecycle operations.
- `skills/zmux/references/agent-peer.md` and `agent-worker.md` own visible peer/worker terminal doctrine.

## Split-logic warnings

- Do not duplicate shell lifecycle waiting with temp sentinels or wrapper scripts; read `zmux tab status --json` or use first-class `zmux wait` / `zmux type --wait-*` condition results.
- Do not let the Pi dispatcher silently normalize opaque remote-admin behavior: numbered `remote-<host>N` tab sprawl and encoded/obfuscated remote payloads need deterministic warnings/tests, not just prose doctrine.
- Do not hand-edit committed runtime projections. Edit `agent-doctrine/`, run `make gen-doctrine`, commit changed runtime projections, and render maintainer test artifacts on demand.
- Do not add a dispatcher operation without updating its contract test, neutral mechanism records, and the guard redirect map when the workflow should be tool-preferred.
- Keep package loading settings-managed. A retired global `~/.pi/agent/extensions/pi-zmux` symlink can mask the local package.
- Keep `zzmux` grounding isolated from live `zmux`; edge profile QA must not mutate live shell startup or agent integration links.
- If a tool shells out to zmux, preserve structural non-zero results instead of crashing the extension process.

## Update triggers

Update this doc when Pi package loading, dispatcher operations, bash guard policy,
project config shape, lifecycle reporting, `zzmux` grounding, skill doctrine
paths, or agent-surface testing prompts change.

## Maintainer setup

The active shared-skill source is `~/donjor/skills`. In the maintainer setup:

- `~/donjor/skills/skills/zmux` symlinks to this repo's `skills/zmux`.
- Pi consumes the generated mirror at `~/.pi/agent/skills/donjor/zmux`.
- `./dev.sh zmux` symlinks this repo's `pi-zmux/` package into `~/donjor/skills/pi/extensions/pi-zmux`.
- Pi sync loads that settings-managed package through the skills repo registry, not directly from this repo.
- That registry classifies `pi-zmux` as the Pi-only replacement for `zmux`, keeps `peer` backed, and validates the shape and generated-rule consistency of `doctrine-manifest.generated.json` before suppressing the full skill. The operation inventory remains owned here: `make check-doctrine` compares the frozen manifest with `pi-zmux/src/operations.ts` without imposing a zmux-specific count in the skills repo. Claude still installs the full skill.

Refresh generated outputs explicitly, then mirrors and package diagnostics:

```bash
make gen-doctrine
./dev.sh zmux
```

`./dev.sh zmux` performs only a non-mutating freshness check and aborts before live mutation when outputs are stale. It never regenerates them. `./dev.sh zzmux` remains binary-only and skips this live-sync gate.

`./dev.sh zmux` refreshes shared skill mirrors, links `pi-zmux/` into the
skills repo's Pi extension registry source directory, removes the retired global
extension symlink if present, and warns when global Pi settings still disable
the package. It does not rewrite global Pi settings.

One-off package smoke:

```bash
pi -e ./pi-zmux --help
```

## Package/API baseline

The extension targets current Pi `0.80.x` era APIs and `@earendil-works/*`
package names. Runtime Pi core packages are peer dependencies; local development
uses dev dependencies from `pi-zmux/package.json`.

```bash
npm --prefix pi-zmux install
npm --prefix pi-zmux run typecheck
npm --prefix pi-zmux test
make test-agent-surfaces
```

`make test-agent-surfaces` first validates Markdown doctrine records, runs generator tests and checks committed projection freshness, then extension typecheck/tests, QA lint, and the shipped zmux skill doctrine doctor. Live-test prompts and answer keys are stdout-only `--render` outputs; they are not package files.

## Agent-surface test prompts

Deterministic checks are not enough to catch every agent-routing and fresh-session
failure. Thin activation wrappers live under `docs/dev/test-prompts/`:

- `zmux-agent-skill-testing-prompt.md` — shared skill/CLI doctrine, `zzmux`
  smoke, raw-tmux avoidance, roster/session/lifecycle/peer-worker coverage.
- `zmux-agent-pi-zmux-testing-prompt.md` — active Pi dispatcher operation
  inventory, bash guardrails, operation smoke, peer composites, and Pi lifecycle
  safety.

Use these prompts after material agent-facing changes, especially new dispatcher
operations, guard classifications, peer/worker flow changes, or edits to shipped
skill doctrine. The wrappers route to durable Claude and Pi frameworks. Shared prompt/outcome bodies
come from `agent-doctrine/scenarios/shared/*.md`; generated host answer keys stay hidden
from workers, while harness-specific launch, inspection, and teardown stay handwritten.
`make test-agent-surfaces` remains the deterministic gate.

The handwritten Claude framework lives at `agent-doctrine/harnesses/claude/`; the parallel Pi framework at `agent-doctrine/harnesses/pi/` is deferred with the Pi extension reintegration and not present on this shared branch. Their worker prompts and host-only answer keys are rendered to stdout by `agent-doctrine/generate.mjs --render`.

- The Claude framework drives one visible Sonnet worker; the Pi framework drives one visible Terra/medium worker.
- Both consume the same 20 shared scenario prompts; package tests retain trusted-config, guard, renderer, and hard-respawn coverage. Pi-only callback and soft-lifecycle scenarios return with the deferred Pi harness.
- Hosts inspect real state, use low-tier lifecycle peers, distinguish clean passes from safe recovered friction, and own exact teardown.

Historical promotion artifacts remain in the skills repo.

## Dispatcher

Pi exposes one model-visible tool, `zmux`, with 40 validated operations.
The canonical package and tool are both named for zmux; this is the sole model-visible tool surface.

Operation groups:

- Context/config: `current`, `tabs`, `sessions`, `panes`, `zmux_reload`.
- Commands/runtimes: `run`, `session_run`, `session_kill`, `runtime_ensure`,
  `runtime_logs`, `runtime_stop`.
- Tabs/peers: `tab_state`, `tab_peer`, `tab_status`, `tab_inspect`, `tab_label`,
  `tab_move`, `tab_place`, `tab_kill`, `tab_focus`, `send_keys`, `type_text`,
  `peer_ensure`, `peer_handoff`.
- Panes/input: `pane_open`, `pane_close`, `pane_resize`, `pane_focus`,
  `pane_send_keys`, `pane_type`, `interactive_type`.
- Evidence/lifecycle: `log`, `snapshot`, `wait`, `callback_watch`,
  `callback_list`, `callback_cancel`, `terminal_current`, `pi_reload`,
  `pi_respawn`.

The schema estimate is gated at no more than 1,200 tokens; the current
production surface is approximately 1,128. The extension does not inject runtime
state into the model system prompt. State is read only when the agent calls a
context operation or another operation resolves the live target it needs.
`/zmux status` retains the full human diagnostic snapshot without adding it to
model context. Bash hooks, lifecycle glyphs, callbacks, and continuation
handlers add no prompt tokens by themselves.

The dispatcher preserves operation-specific safety:

- persistent processes use `runtime_ensure`, which passes readiness into the same detached `zmux run` operation so the output baseline is captured before command delivery and immediate startup output cannot race a later watch;
- `run` maps `focus:false` to focus-preserving creation; every detached run
  automatically arms a shell-lifecycle completion callback and later reports its
  evidence, while `trackCompletion:false` is reserved for work expected never to
  return; `completionTimeoutSeconds` independently controls the one-day wait
  window, which renews silently while the command is still running;
- sudo/manual input uses `interactive_type`;
- atomic `peer_handoff` arms a fresh `turn:ready` callback, marks the peer
  running, and only then submits before the default follow-up continuation; if
  its wait window expires while the peer is still `running`, Pi-zmux silently
  arms the next lifecycle wait instead of emitting a terminal unproven result;
- output-regex and idle callbacks are explicit fallbacks;
- literal pane keys remain separate from submit-with-Enter pane typing;
- focus-moving options stay false unless the user explicitly requests focus.

Native rendering updates one tool card through pending, partial, and final
states. Foreground operations longer than the anti-flicker delay show TUI-only
phase/countdown feedback; the completed card presents its operation,
destination, input, options, and evidence once. Expanded views retain structured
operation, argv, cwd, lifecycle, and raw evidence details.

Scheduled background callbacks—including core-owned detached-run completion
tracking—publish one aggregate above-editor widget line immediately above tasks.
They do not use footer status or periodic model messages. The component occupies
no rows while inactive, remains visible across running-state wait renewals, and
clears on completion, cancellation, session replacement, or shutdown.

Terminal outcomes use the compact native `pi-zmux-callback` renderer. Models do
not need a second `callback_watch` for ordinary detached runs.

Lifecycle freshness is explicit:

- Runtime readiness is an atomic detached launch wait: `zmux run --detach --until <regex>` snapshots pre-launch output, submits the command, and accepts only matching output beyond that baseline.
- New tabs tolerate completion before callback spawn.
- Reused tabs capture the pre-run `cmdSeq` and require a newer lifecycle
  generation, so an old `done` state cannot satisfy the new run.
- Successful wait and callback completions summarize their match basis and
  freshness instead of returning captured output tails to
the model; expanded wait/callback views retain raw diagnostics for human
inspection.

## Bash guardrails

The extension classifies Pi `bash` tool calls as bounded, runtime, interactive,
background, direct-zmux, or direct-tmux. Policy modes are `observe`, `warn`, and
`enforce`; clear runtime/interactive/background/raw-tmux matches default to
enforcement.

A trusted project can override policy with `.pi/zmux.json` or
`.config/pi-zmux.json`. Emergency bypass is deliberately explicit:
`PI_ZMUX_ALLOW=1` or `# pi-zmux: allow` on the bash command.

The `PI_ZMUX_POLICY` environment variable (`observe`, `warn`, or `enforce`)
overrides the policy mode from both the built-in default and any project config
`policy.mode`; an unset or unrecognized value leaves the configured mode in
effect.

## Project config

Trusted project config can define reusable runtimes:

```json
{
  "policy": { "mode": "enforce" },
  "runtimes": {
    "server": {
      "command": "go run ./cmd/api",
      "tab": "server",
      "readiness": "listening|ready|localhost",
      "kind": "server",
      "timeoutSeconds": 90
    }
  }
}
```

With trusted config, agents can call `zmux operation=runtime_ensure` by
name without rediscovering commands or starting duplicate processes.

## Interactive and peer waiting

**Manual commands**

- `operation=interactive_type` reads baseline `cmdSeq`, types the command, and waits for fresh lifecycle evidence.
- It returns early with `needsUserInput` when a password/manual prompt appears without requested focus.
- Long-lived SSH, database REPL, and TUI sessions leave `options.waitForExit` false and tell the user which tab needs attention.

**Peer turns**

- `operation=type_text` delegates to first-class `zmux type --json` with peer-running and turn-wait options.
- `operation=peer_handoff` arms `zmux wait --for turn:ready`, sets `running`,
  then submits. The aggregate Pi activity widget keeps the scheduled wait visible after
  the tool returns. Completion clears it and uses `deliverAs: "followUp"` with
  `triggerTurn: true` unless explicitly changed.
- Pi and Claude currently publish native turn-end lifecycle. Codex and Agy
  complete visible answers without advancing `turnState`; callers use explicit
  `idleSeconds` fallback for those CLIs, while the agent-driven usage test matrix
  keeps their native-lifecycle rows failing until adapters land.
- `deliverAs: "nextTurn"` is rejected when `triggerTurn` is true because Pi
  ignores turn triggers for next-turn delivery.
- Freshness is generation-based via `turnSeq`; stale `ready` state cannot satisfy a new wait.
- If readiness is unproven, return status/output evidence rather than sleeping. Use `tab_inspect` for diagnosis and `peer_ensure` for spawn/reuse plus readiness.

## Grounding with zzmux

Objective extension behavior should be proven against the isolated edge profile:

```bash
./dev.sh zzmux
PI_ZMUX_BIN=zzmux pi -ne -e ./pi-zmux
```

`-ne` disables globally discovered extensions so a live installed
`zmux/pi-zmux` does not conflict with the explicit branch-local extension.
When dispatcher operations need low-level tmux access, `zzmux` implies the isolated
`tmux -L zzmux` socket. Override with `PI_ZMUX_TMUX_SOCKET` only for explicit
socket diagnostics.
