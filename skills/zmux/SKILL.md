---
name: zmux
description: Pi-native terminal/session orchestration with zmux, the local tmux wrapper. Use when managing long-running processes, dev servers, watchers, interactive/sudo commands, shared terminal panes, tmux/zmux sessions, sidecars, or terminal capability diagnostics from a Pi agent.
---

# zmux — Pi Terminal Orchestration

zmux is the local tmux wrapper for persistent, user-visible terminal work. In Pi,
use it deliberately: Pi's `bash` tool is still best for bounded one-shot file,
build, test, and inspection commands; zmux is for processes or terminal state
that should outlive one tool call, be visible to the user, or need interactive
control.

## Quick detection

```bash
[ -n "$TMUX" ] && echo inside-tmux
zmux pane current --json      # current pane/session details when inside zmux
zmux ls                       # workspaces/sessions
zmux tabs                     # tabs/windows in the current session
```

If multiple sessions/workspaces are attached, do not guess. Inspect with `zmux
ls`, `zmux tabs`, or `zmux pane current --json` and target commands explicitly
when the command supports it.

## Decision rules for Pi agents

Use **Pi `bash` directly** for:

- quick reads/searches (`rg`, `ls`, `cat`, `git diff`);
- bounded builds/tests/checks that should finish in this turn;
- scripts where the captured stdout/stderr is the artifact.

Use **zmux** for:

- dev servers, file watchers, queues, REPLs, or anything that keeps running;
- commands requiring user input, passwords, sudo confirmation, or manual control;
- shared visibility with the user in a named tab/pane;
- stopping/restarting or inspecting an existing long-running process;
- sidecars/panels/persistent UI panes;
- terminal capability/truecolor diagnosis.

Do not use `&`, `nohup`, `disown`, or ad-hoc background shell jobs. Use zmux so
processes are named, visible, and controllable.

Prefer typed Pi zmux tools over shelling out to `zmux ...` when a tool exists
(`zmux_tabs`, `zmux_tab_kill`, `zmux_tab_focus`, `zmux_send_keys`, `zmux_type`, `zmux_pane_*`,
`zmux_runtime_*`, `zmux_interactive_type`, `zmux_pi_respawn`). Shelling out to `zmux` is acceptable
only for bounded diagnostics or CLI surface not yet covered by a typed tool. If
a typed tool is broken and an emergency bypass is needed, add `PI_ZMUX_ALLOW=1`
or `# pi-zmux: allow` to the bash command and explain why. After verified Pi
extension/tooling changes that require reloading and no soft reload tool is
available, use `zmux_pi_respawn` instead of asking the user to manually reload;
if autonomous follow-up is expected, pass `continuationPrompt` with exact next
smoke/validation steps before respawning; pi-zmux will resume via a custom
follow-up message, not a user-authored prompt. Skip respawn when unsent user input or
manual validation may be in progress. Prefer zmux wrappers over raw `tmux` for
app-level actions. Raw `tmux` is okay only for low-level diagnostics not exposed
by zmux.

## Core commands

### Run finite commands in named tabs

```bash
zmux run 'npm test' -n test
zmux run 'go test ./...' -n test -T 180
zmux run 'npm run build' -n build -T 120
```

`zmux run` waits by default, streams output, and returns the command exit code.
Do not add your own sentinel/marker output.

### Start and observe servers/watchers

```bash
zmux run 'npm run dev' -n server -d
zmux watch server --until 'ready|listening|started' -T 60
zmux watch server -l 120
zmux send server C-c
```

Use `-d` only for processes expected not to exit. Use `zmux watch` to read
output instead of rerunning probes that disturb state.

### Interactive or privileged commands

```bash
zmux type admin 'sudo apt update'
```

Then tell the user what tab/pane needs their input. Do not attempt to automate
password entry. In Pi, prefer `zmux_interactive_type` with `focus: false`; if it
returns `needsUserInput`, call `ask_user` before switching focus with
`zmux_tab_focus` or rerunning with `focus: true`.

### Panes and sidecars

```bash
zmux pane current --json
zmux pane list
zmux pane open sidecar -r 35 -- some-sidecar-command
zmux pane open --label-tab sidecar -r 35 -- some-sidecar-command
zmux pane toggle sidecar -r 35 -- some-sidecar-command
zmux pane focus sidecar
zmux pane close sidecar
```

Use `--label-tab` for sidecars that may change tmux's automatic window name; it
preserves the conceptual tab label while still showing the auto-name overlay.

### Tab labels

```bash
zmux tab label 'pi'
zmux tab label ''       # clear label
```

Labels are zmux overlays (`label [auto-name]`) and do not disable tmux automatic
renaming.

### Config and terminal capability refresh

```bash
zmux apply                    # non-disruptive config apply
zmux refresh                  # apply config + reattach current client
zmux terminal capabilities    # diagnose RGB/truecolor path
zmux terminal current --json  # visible terminal target for screenshots
```

`zmux refresh` replaces/redraws the current tmux client. Do not run it from an
automated Pi harness unless the user asked for it or disruption is acceptable;
it can disturb the active agent connection. Prefer telling the user to run it
when the current session must stay stable.

## Naming conventions

Use stable, descriptive names:

- `server` for dev servers;
- `test` for test runners/watchers;
- `build` for builds;
- `logs` for log tails;
- `admin` for sudo/interactive commands;
- `<tool>-sidecar` for UI sidecars.

## Good Pi patterns

### Bounded verification

```bash
go test ./...
```

Run directly through Pi `bash` unless it is slow enough to need shared visibility
or the user asked to watch it.

### Long-running server

```bash
zmux run 'npm run dev' -n server -d
zmux watch server --until 'Local:|ready|listening' -T 90
```

### User-visible restart

```bash
zmux send server C-c
zmux run 'npm run dev' -n server -d
zmux watch server --until 'ready|listening' -T 90
```

## Avoid

- Starting servers/watchers in Pi `bash`.
- Using `&`, `nohup`, or `disown`.
- Creating unnamed terminals.
- Guessing existing process state instead of `zmux_runtime_logs`, `zmux_tabs`,
  or `zmux_current`.
- Running `zmux refresh` from an agent session without considering that it
  reattaches the current client.
- Using `bash` to run direct `zmux` or stateful `tmux` commands when an equivalent typed tool exists.
- Using raw `tmux` for ordinary zmux-managed actions. For sidecar pane ids, use
  `zmux_pane_send_keys` / `zmux_pane_type` instead of `tmux send-keys`.
