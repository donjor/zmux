# Pi zmux extension

The repo owns a Pi extension plus the shared zmux agent skill. The skill teaches
terminal/session doctrine; the extension adds typed Pi tools and bash guardrails
so agents use visible zmux-managed tabs instead of hidden shell jobs or raw tmux.

## Owned paths

- `pi-extension/index.ts` — package entry for Pi extension loading.
- `pi-extension/src/**` — context injection, bash classification, zmux wrappers, and typed tool registration.
- `pi-extension/test/**`, `pi-extension/package.json`, `pi-extension/tsconfig.json` — TypeScript validation surface.
- `skills/zmux/SKILL.md`, `skills/zmux/references/**`, `skills/zmux/hooks/**`, `skills/zmux/test/**` — shared agent doctrine, hooks, and doctrine doctor.
- `docs/dev/agent-grounding.md` — live `zzmux` grounding protocol for agents.

## Invariants

- Long-running, interactive, sudo/password, watcher, server, and TUI commands belong in zmux tabs or panes, not the agent shell.
- Pi tools are focus-safe by default. They move terminal focus only when the user explicitly asks or after a focused confirmation.
- Direct raw tmux app-control paths are blocked when a typed zmux or Pi tool exists.
- `zmux_pi_reload` is the soft path after Pi extension, skill, prompt, or theme changes; `zmux_pi_respawn` is a destructive fallback.
- Persistent Pi process liveness is not a running signal. Only an active Pi turn should publish the running glyph.
- Project config containing commands is read only when Pi marks the project trusted.

## Reusable primitives

- `src/classify.ts` shares the guard vocabulary with the zmux skill and Claude hook tests.
- `src/zmux/**` contains low-level wrappers for context, sessions, tabs, panes, runtimes, Pi lifecycle, and interactive waiting.
- `src/tools/**` groups typed Pi tool registration by core, tabs, panes, and runtimes.
- `src/config.ts` loads trusted project config and configured runtimes.
- `src/reload-continuation.ts` and `src/respawn-continuation.ts` build safe continuation prompts for Pi lifecycle tools.
- `skills/zmux/references/agent-peer.md` and `agent-worker.md` own visible peer/worker terminal doctrine.

## Split-logic warnings

- Do not duplicate shell lifecycle waiting with temp sentinels or wrapper scripts; read `zmux tab status --json` / command lifecycle state.
- Do not add a Pi typed tool without updating skill doctrine and the guard redirect map when the workflow should be tool-preferred.
- Keep package loading settings-managed. A retired global `~/.pi/agent/extensions/pi-zmux` symlink can mask the local package.
- Keep `zzmux` grounding isolated from live `zmux`; edge profile QA must not mutate live shell startup or agent integration links.
- If a tool shells out to zmux, preserve structural non-zero results instead of crashing the extension process.

## Update triggers

Update this doc when Pi package loading, typed tool names, bash guard policy,
project config shape, lifecycle reporting, `zzmux` grounding, or skill doctrine
paths change.

## Maintainer setup

The active shared-skill source is `~/donjor/skills`. In the maintainer setup:

- `~/donjor/skills/skills/zmux` symlinks to this repo's `skills/zmux`.
- Pi consumes the generated mirror at `~/.pi/agent/skills/donjor/zmux`.
- Pi loads this extension as a settings-managed local package from `../../donjor/zmux/pi-extension`.

Refresh mirrors and package diagnostics from the repo root:

```bash
./dev.sh zmux
```

`./dev.sh zmux` refreshes shared skill mirrors, removes the retired global
extension symlink if present, and warns when global Pi settings still disable
the package. It does not rewrite global Pi settings.

One-off package smoke:

```bash
pi -e ./pi-extension --help
```

## Package/API baseline

The extension targets current Pi `0.80.x` era APIs and `@earendil-works/*`
package names. Runtime Pi core packages are peer dependencies; local development
uses dev dependencies from `pi-extension/package.json`.

```bash
bun --cwd pi-extension install
bun --cwd pi-extension run typecheck
bun --cwd pi-extension test
make test-agent-surfaces
```

`make test-agent-surfaces` runs the extension typecheck/tests, QA lint, and the
shipped zmux skill doctrine doctor.

## Tools

Core inspection and config:

- `zmux_current` — inspect current pane/session/tabs, selected binary/profile,
  terminal capabilities, project trust, and loaded pi-zmux config.
- `zmux_reload` — run `zmux reload` for zmux config/key/theme changes.
- `zmux_pi_reload` — type Pi's `/reload` into the current Pi pane after a safe
  delay and nudge the agent after reload.
- `zmux_pi_respawn` — hard fallback that respawns the Pi pane and discards
  unsent input.

Sessions, tabs, panes, and input:

- `zmux_sessions`, `zmux_session_run`, `zmux_session_kill`
- `zmux_tabs`, `zmux_tab_status`, `zmux_tab_state`, `zmux_tab_place`,
  `zmux_tab_label`, `zmux_tab_move`, `zmux_tab_kill`, `zmux_tab_focus`
- `zmux_pane_list`, `zmux_pane_open`, `zmux_pane_focus`, `zmux_pane_close`,
  `zmux_pane_resize`
- `zmux_send_keys`, `zmux_type`, `zmux_pane_send_keys`, `zmux_pane_type`

Runtime/output/evidence:

- `zmux_run` — reviewable command-in-tab one-shots.
- `zmux_runtime_ensure`, `zmux_runtime_logs`, `zmux_runtime_stop` — stable named
  runtime tabs.
- `zmux_log` — bounded persistent tab output recording.
- `zmux_snapshot` — terminal/TUI evidence bundles.
- `zmux_terminal_current` — visible desktop terminal resolution.
- `zmux_interactive_type` — sudo/password/manual-input commands in visible tabs.

## Bash guardrails

The extension classifies Pi `bash` tool calls as bounded, runtime, interactive,
background, direct-zmux, or direct-tmux. Policy modes are `observe`, `warn`, and
`enforce`; clear runtime/interactive/background/raw-tmux matches default to
enforcement.

A trusted project can override policy with `.pi/zmux.json` or
`.config/pi-zmux.json`. Emergency bypass is deliberately explicit:
`PI_ZMUX_ALLOW=1` or `# pi-zmux: allow` on the bash command.

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

With trusted config, agents can call `zmux_runtime_ensure` by name without
rediscovering commands or starting duplicate processes.

## Interactive waiting

For bounded sudo/manual commands, `zmux_interactive_type` reads a baseline
`cmdSeq`, types the command, and waits for a fresh lifecycle result. It returns
early with `needsUserInput` when a password or manual prompt appears and focus
was not requested. Long-lived shells such as SSH, database REPLs, and TUI apps
should leave `waitForExit` false and tell the user which tab needs attention.

## Grounding with zzmux

Objective extension behavior should be proven against the isolated edge profile:

```bash
./dev.sh zzmux
PI_ZMUX_BIN=zzmux pi -e ./pi-extension
```

When typed tools need low-level tmux operations, `zzmux` implies the isolated
`tmux -L zzmux` socket. Override with `PI_ZMUX_TMUX_SOCKET` only for explicit
socket diagnostics.
