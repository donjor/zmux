# Pi zmux extension

The zmux repo owns a Pi extension in [`pi-extension/`](../pi-extension/) plus the
zmux skill in [`skills/zmux/`](../skills/zmux/). The skill teaches doctrine; the
extension adds typed Pi tools and guardrails so agents do not accidentally start
hidden runtimes, duplicate servers, or bypass zmux's tab/session bookkeeping.

## Maintainer setup

The active shared-skills source is `~/donjor/skills` (not `skills0`). In the
maintainer setup:

- `~/donjor/skills/skills/zmux` symlinks to this repo's `skills/zmux`.
- Pi consumes the generated mirror at `~/.pi/agent/skills/donjor/zmux`.
- Pi loads the extension as a local settings-managed package:

```json
{ "source": "../../donjor/zmux/pi-extension", "extensions": ["+index.ts"] }
```

Refresh mirrors and package diagnostics from this repo:

```bash
./dev.sh zmux
```

`./dev.sh zmux` refreshes the shared skill mirrors and removes the retired global
`~/.pi/agent/extensions/pi-zmux` symlink if present so it cannot mask the
settings-managed package. It deliberately does **not** rewrite global Pi
settings. It warns when `~/.pi/agent/settings.json` disables the package with an
entry such as `-extensions/pi-zmux/index.ts`; remove that exclusion and run
`/zmux reload` or restart Pi to enable it.

For a one-off load smoke without changing global settings:

```bash
cd /home/user/donjor/zmux
pi -e ./pi-extension --help
```

## Package/API baseline

The extension targets current Pi (`0.80.x` at the time of this overhaul) and uses
`@earendil-works/*` package names. Runtime Pi core packages are peer dependencies;
local development installs dev dependencies from `pi-extension/package.json`:

```bash
cd pi-extension
npm install
npm run typecheck
npm test
```

`pi-extension/node_modules` and `package-lock.json` are local dev artifacts and
are ignored by the repo.

## Tools

Core tools:

- `zmux_current` — inspect current pane/session/tabs, terminal capabilities,
  selected zmux binary/profile, project trust, and loaded pi-zmux config.
- `zmux_reload` — queue soft `/zmux reload` as a follow-up command. Prefer this
after extension/skill changes before hard respawn.
- `zmux_tabs` / `zmux_tab_kill` / `zmux_tab_focus` — list, intentionally remove,
  or focus tabs. Ask before focusing in agent sessions.
- `zmux_send_keys` / `zmux_type` — send raw keys or type text into existing tabs.
- `zmux_pane_list` / `zmux_pane_open` / `zmux_pane_focus` /
  `zmux_pane_close` / `zmux_pane_resize` — inspect and manage panes through zmux
  verbs instead of raw tmux.
- `zmux_pane_send_keys` / `zmux_pane_type` — lower-level pane-id input for
  sidecars when no logical tab name exists.
- `zmux_runtime_ensure` / `zmux_runtime_logs` / `zmux_runtime_stop` — manage
  persistent software-under-development runtimes in stable named tabs.
- `zmux_interactive_type` — type sudo/password/manual-input commands into a
  shared visible tab such as `admin`. One-shot sudo/manual commands can wait for
  completion using the status-file wrapper.
- `zmux_pi_respawn` — hard fallback: respawn the current Pi pane with `pi -c`.
  This kills the current pane process and discards unsent input; use only when
  soft reload is unavailable or Pi is wedged.

## Bash guardrails

The extension intercepts Pi `bash` tool calls and classifies commands as:

- bounded/safe — allowed;
- runtime — should use `zmux_runtime_ensure`;
- interactive — should use `zmux_interactive_type`;
- background — blocks `&`, `nohup`, and `disown` hidden jobs;
- `direct_zmux` — nudges to typed tools when a typed Pi tool exists;
- `direct_tmux` — blocks raw tmux app-level subcommands that have a zmux or typed
  equivalent; socket-scoped diagnostics such as `tmux -L zzmux ...` remain safe.

Policy mode is configurable:

- `observe` — no blocking, useful for smoke tests;
- `warn` — notify but allow;
- `enforce` — block known runtime/interactive/background/raw-tmux slips.

The default is `enforce` for clear matches. Override with `PI_ZMUX_POLICY` or a
trusted project config. For a one-off emergency escape hatch when a typed tool is
broken, add either `PI_ZMUX_ALLOW=1` or `# pi-zmux: allow` to the bash command;
this logs a warning and bypasses the guardrail for that command only.

## Project config and trust

Optional project config is JSON at `.pi/zmux.json` or `.config/pi-zmux.json` in
the project or an ancestor directory:

```json
{
  "policy": {
    "mode": "enforce",
    "blockBackgroundJobs": true,
    "redirectInteractive": true
  },
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

Because config can contain commands, the global extension reads it only when Pi
considers the project trusted (`ctx.isProjectTrusted()`). If a config path exists
but trust is false, the extension reports it as ignored and falls back to default
policy/no configured runtimes.

With trusted config present, agents can call:

```text
zmux_runtime_ensure({ "name": "server" })
zmux_runtime_logs({ "name": "server", "lines": 200 })
```

without rediscovering commands or starting duplicate processes.

## Interactive command waiting

For commands that should eventually exit, such as readonly sudo status checks,
use `zmux_interactive_type` with `waitForExit: true` or omit it for obvious sudo
one-shots:

```text
zmux_interactive_type({
  "tab": "admin",
  "command": "sudo ufw status",
  "waitForExit": true,
  "timeoutSeconds": 90
})
```

When `waitForExit` is enabled, the extension writes a temporary wrapper script
that records the exit code to a status file; no `__PI_ZMUX_*` sentinels are
printed into the terminal. With `focus: false`, it also detects common
password/manual-input prompts and returns early with `needsUserInput` details
instead of silently waiting for timeout. Pass `focus: true` or call
`zmux_tab_focus` only when the user asked to be taken to that tab or after an
explicit confirmation. For long-lived interactive shells such as `ssh`, `psql`,
or a REPL, leave `waitForExit` false and tell the user which tab needs attention.

## Grounding with zzmux

For objective extension behavior, build the isolated edge binary and point the
extension at it:

```bash
./dev.sh zzmux
PI_ZMUX_BIN=zzmux pi -e ./pi-extension
```

When a typed tool still needs raw tmux for low-level pane operations, `zzmux` is
inferred as `tmux -L zzmux`. Override with `PI_ZMUX_TMUX_SOCKET=<socket>` if
needed.

## Safety

`zmux refresh` / `zmux terminal refresh` can reattach the current tmux client and
disrupt an automated Pi harness. The skill and extension context warn agents not
to run it unless the user asked or disruption is acceptable.
