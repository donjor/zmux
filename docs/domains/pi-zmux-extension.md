# Pi zmux extension

The repo owns a Pi extension plus the shared zmux agent skill. The skill teaches
terminal/session doctrine; the extension adds one compact `zmux_lite` dispatcher,
on-demand runtime inspection, and bash guardrails so agents use visible
zmux-managed tabs instead of hidden shell jobs or raw tmux.

## Owned paths

- `pi-zmux/index.ts` — package entry for Pi extension loading.
- `pi-zmux/src/**` — dispatcher registration, on-demand diagnostics, bash classification, and zmux adapters.
- `pi-zmux/test/**`, `pi-zmux/package.json`, `pi-zmux/tsconfig.json` — TypeScript validation surface.
- `skills/zmux/SKILL.md`, `skills/zmux/references/**`, `skills/zmux/hooks/**`, `skills/zmux/test/**` — shared agent doctrine, hooks, and doctrine doctor.
- `docs/dev/agent-grounding.md` — live `zzmux` grounding protocol for agents.
- `docs/dev/test-prompts/zmux-agent-*-testing-prompt.md` — prompt-driven exploratory QA for fresh isolated sessions testing the whole agent-facing skill/Pi surface.

## Invariants

- Long-running, interactive, sudo/password, watcher, server, and TUI commands belong in zmux tabs or panes, not the agent shell.
- Dispatcher operations are focus-safe by default. They move terminal focus only when the user explicitly asks or after a focused confirmation.
- Direct raw tmux app-control paths are blocked when an equivalent dispatcher operation exists.
- `zmux_lite operation=pi_reload` is the soft path after Pi extension, skill, prompt, or theme changes; `operation=pi_respawn` is a destructive fallback.
- Persistent Pi process liveness is not a running signal. Only an active Pi turn should publish the running glyph.
- Project config containing commands is read only when Pi marks the project trusted.

## Reusable primitives

- `src/dispatcher.ts` owns the one-tool schema and its 40 operation mappings.
- `src/exec.ts` and `src/interactive.ts` are focused dispatcher adapters over the `zmux` CLI and tmux socket.
- `src/classify.ts` shares the guard vocabulary with the zmux skill and Claude hook tests.
- `src/zmux/**` retains context, lifecycle, and continuation primitives used outside the model-visible schema.
- `src/config.ts` loads trusted project config and configured runtimes.
- `src/reload-continuation.ts` and `src/respawn-continuation.ts` build safe continuation prompts for Pi lifecycle operations.
- `skills/zmux/references/agent-peer.md` and `agent-worker.md` own visible peer/worker terminal doctrine.

## Split-logic warnings

- Do not duplicate shell lifecycle waiting with temp sentinels or wrapper scripts; read `zmux tab status --json` or use first-class `zmux wait` / `zmux type --wait-*` condition results.
- Do not let the Pi dispatcher silently normalize opaque remote-admin behavior: numbered `remote-<host>N` tab sprawl and encoded/obfuscated remote payloads need deterministic warnings/tests, not just prose doctrine.
- Do not add a dispatcher operation without updating its contract test, skill doctrine, and the guard redirect map when the workflow should be tool-preferred.
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

Refresh mirrors and package diagnostics from the repo root:

```bash
./dev.sh zmux
```

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
bun --cwd pi-zmux install
bun --cwd pi-zmux run typecheck
bun --cwd pi-zmux test
make test-agent-surfaces
```

`make test-agent-surfaces` runs the extension typecheck/tests, QA lint, and the
shipped zmux skill doctrine doctor.

## Agent-surface test prompts

Deterministic checks are not enough to catch every agent-routing and fresh-session
failure. Prompt-driven exploratory QA lives under `docs/dev/test-prompts/`:

- `zmux-agent-skill-testing-prompt.md` — shared skill/CLI doctrine, `zzmux`
  smoke, raw-tmux avoidance, roster/session/lifecycle/peer-worker coverage.
- `zmux-agent-pi-zmux-testing-prompt.md` — active Pi dispatcher operation
  inventory, bash guardrails, operation smoke, peer composites, and Pi lifecycle
  safety.

Use these prompts after material agent-facing changes, especially new dispatcher
operations, guard classifications, peer/worker flow changes, or edits to shipped
skill doctrine. The prompts are exploratory QA wrappers: expected behavior
remains in this domain doc plus `skills/zmux/SKILL.md` and its references, while
`make test-agent-surfaces` remains the deterministic gate. The accepted one-tool
candidate now lives in this canonical package; its Terra/medium acceptance
artifacts remain in the skills repo.

## Dispatcher

Pi exposes one model-visible tool, `zmux_lite`, with 40 validated operations.
The stable package name remains `pi-zmux`; `lite` names the compact tool surface,
not a second installed extension.

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

The schema estimate is gated at no more than 1,200 tokens; the accepted
production surface is approximately 995. The extension does not inject runtime
state into the model system prompt. State is read only when the agent calls a
context operation or another operation resolves the live target it needs.
`/zmux status` retains the full human diagnostic snapshot without adding it to
model context. Bash hooks, lifecycle glyphs, callbacks, and continuation
handlers add no prompt tokens by themselves.

The dispatcher preserves operation-specific safety: persistent processes use
`runtime_ensure`; sudo/manual input uses `interactive_type`; peer prompts plus
future-output callbacks use atomic `peer_handoff`; and focus-moving options stay
false unless the user explicitly requests focus.

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

With trusted config, agents can call `zmux_lite operation=runtime_ensure` by
name without rediscovering commands or starting duplicate processes.

## Interactive and peer waiting

**Manual commands**

- `operation=interactive_type` reads baseline `cmdSeq`, types the command, and waits for fresh lifecycle evidence.
- It returns early with `needsUserInput` when a password/manual prompt appears without requested focus.
- Long-lived SSH, database REPL, and TUI sessions leave `options.waitForExit` false and tell the user which tab needs attention.

**Peer turns**

- `operation=type_text` delegates to first-class `zmux type --json` with peer-running and turn-wait options.
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
