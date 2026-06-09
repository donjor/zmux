# Pi zmux extension

The zmux repo owns a Pi extension in [`pi-extension/`](../pi-extension/). It is
intended to be symlinked into Pi's global extension directory:

```bash
ln -s /home/user/donjor/zmux/pi-extension ~/.pi/agent/extensions/pi-zmux
```

The repo also owns the zmux skill in [`skills/zmux/`](../skills/zmux/). On the
maintainer setup, the skill is exposed to Pi through the shared generated skill
mirror at `~/.pi/agent/skills/donjor/zmux` rather than the disabled direct
`~/.pi/agent/skills/zmux` path. The full shared-skills setup, including Pi
settings, lives in `~/donjor/skills`:

```bash
node /home/user/donjor/skills/pi/sync-pi.mjs
```

For this repo's maintainer loop, `./dev.sh zmux` only refreshes the mirrors and
relinks the Pi extension; it does not rewrite global agent settings.

## Purpose

The skill teaches Pi agents how to reason about zmux. The extension adds
deterministic rails so agents do not accidentally start duplicate or hidden
long-running processes.

The extension treats these as **runtimes** rather than only "dev servers":
servers, workers, watch processes, TUI demos, local app prototypes, and any
software under development that keeps running and produces logs.

## Tools

- `zmux_current` — inspect current pane/session/tabs, terminal capabilities, and
  loaded pi-zmux config.
- `zmux_tabs` / `zmux_tab_kill` / `zmux_tab_focus` — list, intentionally
  remove, or focus tabs. Ask before focusing in agent sessions.
- `zmux_send_keys` / `zmux_type` — send raw keys or type text into existing tabs.
- `zmux_pane_send_keys` / `zmux_pane_type` — send raw keys or type text into a
  specific pane id, e.g. sidecar panes returned by `clean_split_control`.
- `zmux_pane_list` / `zmux_pane_focus` / `zmux_pane_close` — inspect and manage
  panes without shelling out through `bash`.
- `zmux_pi_respawn` — hard-restart the current Pi pane by respawning it with
  `pi -c`. Use after verified Pi extension/tooling changes when soft `/reload`
  is unavailable, or when Pi is wedged. Prefer it over asking the user to
  manually reload, but skip it when unsent user input or manual validation may be
  in progress. This kills and replaces the current pane process, so unsent input
  is discarded. If work should continue afterward, pass `continuationPrompt`; the
  tool writes a handoff file plus a pending continuation record. On the next
  Pi startup, pi-zmux injects it as a custom follow-up message rather than as a
  user-authored CLI prompt.
- `zmux_runtime_ensure` — ensure a runtime is running in a stable named zmux tab.
- `zmux_runtime_logs` — read output from a runtime tab.
- `zmux_runtime_stop` — send `C-c` to a runtime tab.
- `zmux_interactive_type` — type sudo/password/manual-input commands into a
  shared tab such as `admin`; for one-shot sudo/manual commands it can wrap the
  command with a completion sentinel and wait until the command exits, the user
  cancels, or a timeout expires.

## Bash guardrails

The extension intercepts Pi `bash` tool calls and classifies commands as:

- bounded/safe — allowed;
- runtime — should use `zmux_runtime_ensure`;
- interactive — should use `zmux_interactive_type`;
- background — blocks `&`, `nohup`, and `disown` style hidden jobs;
- direct zmux/tmux CLI — blocks common `zmux ...` and stateful `tmux ...` bash
  calls when an equivalent typed tool exists, so agents use deterministic tool
  calls for tab/pane/send/runtime operations.

Policy mode is configurable:

- `observe` — no blocking, useful for smoke tests;
- `warn` — notify but allow;
- `enforce` — block known runtime/interactive/background commands.

The default is `enforce` for clear matches. Override with `PI_ZMUX_POLICY` or
project config. For a one-off emergency escape hatch when a typed tool is broken,
add either `PI_ZMUX_ALLOW=1` or `# pi-zmux: allow` to the bash command; this logs
a warning and bypasses the guardrail for that command only.

## Project config

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
    },
    "worker": {
      "command": "python -m app.worker",
      "tab": "worker",
      "readiness": "ready|started",
      "kind": "worker"
    }
  }
}
```

With config present, agents can call:

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

The extension types a wrapped command into the shared tab, then polls tab output
for a unique completion sentinel. This avoids brittle command-specific regexes
and lets the agent know as soon as the user entered the password correctly, sudo
failed, the command exited, or the wait timed out. By default, `zmux_interactive_type` types into the tab without moving the user's
focus. When `waitForExit` is enabled, the extension writes a temporary wrapper
script that records the exit code to a status file; no `__PI_ZMUX_*` sentinels are
printed into the terminal. With `focus` false, it also detects common
password/manual-input prompts such as sudo password prompts and returns early
with `needsUserInput` details instead of silently waiting for timeout. Pass
`focus: true` or call `zmux_tab_focus` only when the user asked to be taken to
that tab or after an explicit confirmation. For long-lived interactive shells
such as `ssh`, `psql`, or a REPL, leave `waitForExit` false and tell the user
which tab needs attention.

## Safety

`zmux refresh` reattaches the current tmux client and can disrupt an automated Pi
harness. The skill and extension context warn agents not to run it unless the
user asked or disruption is acceptable.
