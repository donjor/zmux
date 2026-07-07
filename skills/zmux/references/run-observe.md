# Run & observe with zmux

Use this when you need concrete `zmux` verbs after the hot rules in `SKILL.md` decide that work belongs in a visible terminal.

## Context and session targeting

```bash
[ -n "$TMUX" ] && echo inside-tmux   # inside a session — commands work without -s
zmux where                           # workspace / session / tab / pane / cwd (alias: whoami)
zmux ls                              # all workspaces / sessions
zmux tabs                            # tabs in the current session
```

`zmux where` is the one-shot "where am I". The raw `zws_…` session it prints is a valid `-s` target. Outside tmux, list sessions with `zmux ls -s` and pass `-s <session>`; never guess between attached sessions.

`-s <session>` is accepted by `run`, `watch`, `send`, `type`, `tabs`, `log`, `tab state`, and tab placement verbs. It accepts bare labels, `workspace/session`, or raw tmux names. Ambiguous bare labels are refused.

## Run commands in named tabs

```bash
zmux run '<cmd>' -n <name>              # run + wait for completion (default)
zmux run '<cmd>' -n <name> -T 180       # wait, 180s timeout (default 120)
zmux run '<cmd>' -n <name> -d           # detach — for commands expected to keep running
zmux run '<cmd>' -n <name> -f           # follow output live (Ctrl+C stops following)
zmux run '<cmd>' -n <name> -s <session> # target a specific session
```

`zmux run` waits by default, streams output, then returns the command exit code via zmux's shell-lifecycle run-result channel. It types normal single-line commands directly into the tab so they remain visible and shell-history re-runnable; temp script indirection is reserved for rare command text that cannot be delivered as one prompt line. It does not print completion sentinels — do not add your own `echo ":::DONE:::"` markers, wrapper scripts, or `sleep && watch` layer.

If a tab with that name already exists, the command is sent to it and the tab is reused. `-d` creates or reuses the tab without stealing focus; use it only for commands expected to keep running.

A wait timeout that mentions the shell lifecycle result usually means the target shell does not have the `zmux setup shell` block loaded, or the command is still running. Verify with `zmux watch <tab>` or `zmux log tail <tab>` before relaunching.

### Stable names

The first time `run`, `send`, or `type` reaches a tab by name, zmux pins that stable label. The tab stays reachable even if tmux auto-renames the window to `node`, `vite`, etc. `watch` resolves labels but is read-only and never pins; if a tab drifted before it was pinned, inspect `zmux tabs`, then address its current name with `send`/`type` or label it explicitly.

## Read tab state and output

Use `tab status` / `tab inspect` for lifecycle/command/peer state and one-shot diagnosis:

```bash
zmux tab status <tab> --json                  # machine-readable state
zmux tab inspect <tab> --json -l 120          # state + output tail + warnings
```

Use `wait` for structured condition waits and `watch` for human output reading:

```bash
zmux wait <tab> --for cmd:done --json         # fresh command lifecycle result
zmux wait <tab> --for turn:ready --json       # fresh peer/agent turn result
zmux wait <tab> --for output:'ready|listening' --json -T 60
zmux wait <tab> --for idle:3s --json -T 300
zmux watch <tab>                              # last 50 lines
zmux watch <tab> -l 200                       # last 200 lines
zmux watch <tab> -f                           # follow live
```

`wait --for output:<regex>` and legacy `watch --until` snapshot the buffer at start and match only new output after that baseline. Still choose a pattern that comes from future output, not from text you just typed or an echoed prompt. For fast responders, `wait --json` distinguishes an already-visible marker with `failureKind: "output_regex_already_present"` and `alreadyInTail: true`; treat that as tail evidence, not fresh future output. Pair waits with a buffer/log proof (`tab inspect`, `watch -l`, `zmux_log tail`, or `zmux_tab_inspect` in Pi) instead of retrying blindly. Do not use output/idle evidence as lifecycle truth when `tab status` or `wait --for turn:/cmd:` can answer state. Do not use `watch` as lifecycle truth.

For persistent bounded recording that survives detach, use `zmux log`:

```bash
zmux log start <tab>          # record output to a bounded file
zmux log start <tab> --ansi   # preserve color/escape output
zmux log tail <tab>           # print recorded log
zmux log status               # currently recorded tabs
zmux log stop <tab>           # stop recording
```

`log` is for line-oriented output such as servers, builds, and tests. Fullscreen TUIs record as escape soup. Use `watch -f` for live following.

## Send keys and type text

```bash
zmux send <tab> C-c          # raw keys: C-c, Enter, Escape, arrows, etc.
zmux type <tab> '<text>'     # type text and submit it
zmux type <peer> '<prompt>' --mark-peer-running --wait-turn ready --json
```

`send` and `type` target an existing tab. Create a persistent shell first with `zmux run … -n <tab> -d` when you intend to drive it interactively.

## Common patterns

```bash
# Reviewable one-shot that exits but should stay inspectable
zmux run 'go test ./...' -n scratch -T 180

# Headed/browser-visible Playwright or Chrome proof batch
# Reuse one scratch/proof tab for serial lanes; do not mint one tab per spec.
zmux run 'PLAYWRIGHT_BASE_URL=https://app.localhost bun run test:2d:surface' -n pw-scratch -T 300
zmux run 'PLAYWRIGHT_BASE_URL=https://app.localhost bun run test:2d:quality' -n pw-scratch -T 300
zmux watch pw-scratch -l 160
zmux tab kill pw-scratch     # after evidence is copied and no inspection is needed

# Dev server / persistent runtime
zmux run 'npm run dev' -n dev -d
zmux watch dev --until 'Local:|ready|listening' -T 90

# Restart in place
zmux send dev C-c
zmux run 'npm run dev' -n dev -d
zmux watch dev --until 'ready|listening' -T 90

# Interactive / privileged handoff
zmux run 'sudo apt update' -n admin -d
# Tell the user: "sudo command is in the admin tab — enter your password."
```

Do not automate password entry.

## Logical tabs and placements

A zmux tab is a stable logical unit (id + label + state), not a window slot. It can be a full window, a pane inside another tab, or hidden in the dock, and `run -n` / `send` / `type` / `watch` keep reaching it by name in every placement.

Address tabs by zmux name, not raw tmux window index, unless a command explicitly asks for a raw pane/window target.

For the full command catalog — sessions/workspaces, placements, panes, snapshots, terminal capabilities, config, and naming conventions — use `references/cli-catalog.md`.
