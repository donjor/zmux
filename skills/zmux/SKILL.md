---
name: zmux
description: "Terminal/session management via zmux (tmux wrapper). TRIGGER when: running inside tmux, user mentions zmux/tmux sessions, running dev servers, or needing to manage terminal processes. Provides rules for how to run commands, manage tabs, and share terminals with the user."
---

# zmux — Terminal Management

You are likely working inside a zmux-managed tmux session. Use zmux commands for all terminal/process management.

## Am I in zmux?

Check: `[ -n "$TMUX" ]` — if set, you're inside tmux and zmux commands work without `-s`.

If `$TMUX` is not set, check if tmux is running: `zmux ls`. If sessions exist, you can still use all zmux commands — just always pass `-s <session>` to target the right session. **Never fall back to running commands directly just because you're not inside tmux.**

## Key Rules

1. **Never run long-running processes in your own shell.** Use `zmux run` to create a named tab.
2. **Never use `&` for background tasks.** Use `zmux run` — the user can always see what's running.
3. **`zmux run` waits by default.** It prints output and returns the exit code. No need for `sleep && watch`.
4. **For servers that don't exit**, use `zmux run -d` (detach) then `zmux watch --until` to wait for ready.
5. **Read terminal output** with `zmux watch` instead of re-running commands to check status.
6. **For sudo or interactive commands**, use `zmux type` to put the command in a shared tab for the user to confirm.
7. **Never use raw `tmux` commands.** Always use `zmux` wrappers.

## Commands Reference

### Sessions
```bash
zmux ls                          # list all sessions
zmux <name>                      # attach or create session (shorthand)
zmux new [name]                  # create + attach (alias: n)
zmux new -t <template> [name]   # create from template
zmux attach <name>               # attach (alias: a)
zmux attach --mirror <name>      # shared view — both clients see same thing
zmux attach --hijack <name>      # steal session, kick other clients
zmux kill <name>                 # kill session (alias: k)
```

### Tabs
```bash
zmux tabs                        # list tabs in current session (alias: t)
zmux tabs <session>              # list tabs in specific session
```

### Workspaces
```bash
zmux ws list                     # see workspaces and their sessions
zmux ws add myproject dev        # tag session "dev" to workspace "myproject"
zmux ws remove dev               # untag session
zmux ws show myproject           # show sessions in workspace
zmux new server -w myproject     # create session tagged to workspace
```

### Run commands in named tabs
```bash
zmux run '<cmd>' -n <name>               # run + wait for completion (DEFAULT)
zmux run '<cmd>' -n <name> -T 60         # wait with 60s timeout
zmux run '<cmd>' -n <name> -d            # detach — for servers, don't wait
zmux run '<cmd>' -n <name> -f            # follow output live (Ctrl+C to stop)
zmux run '<cmd>' -n <name> -s <session>  # target specific session
```

**Default behavior:** waits for completion, prints output live, returns exit code.
**If tab exists:** command is sent to it (reused, not recreated).

**Standard tab names:**
- `server` — dev servers (npm run dev, make dev)
- `build` — build processes
- `test` — test runners
- `git` — git operations
- `admin` — sudo/privileged commands

### Send keys / type into tabs
```bash
zmux send <tab> <keys...>       # raw keys (C-c, Enter, Escape, etc.)
zmux type <tab> '<text>'        # type text + press Enter
```

### Read tab output
```bash
zmux watch <tab>                              # last 50 lines
zmux watch <tab> -l 200                       # last 200 lines
zmux watch <tab> -f                           # follow (tail -f style)
zmux watch <tab> --until "pattern"            # wait for regex match
zmux watch <tab> --until "ready|error" -T 60  # wait with timeout
```

## How `zmux run` works (important)

`zmux run` (default wait mode) automatically:
1. Appends a `:::AGENT_DONE $?:::` sentinel to your command
2. Polls the tab output for that sentinel
3. Streams new lines to stdout as they appear (deduped)
4. Returns the command's exit code when done

This means you **never need to add your own echo markers**. Just use `-T` for long commands.

## Patterns

### One-shot commands (waits by default)
```bash
zmux run 'npm test' -n test                    # fast, default timeout
zmux run 'npm run build' -n build -T 60        # 60s timeout
zmux run 'make dev-setup' -n server -T 600     # long setup, 10min timeout
zmux run 'make dev-stop' -n server             # waits for stop
```
This is the right pattern for **anything that exits** — fast or slow. Use `-T` for long ones.

### Dev server (long-running, never exits)
```bash
zmux run 'make dev' -n server -d               # detach — don't wait
zmux watch server --until "running|ready" -T 60 # wait for ready signal
```
Only use `-d` for processes that **run forever** (servers, watchers). Then use `--until` to wait for the ready signal.

### Restart a server
```bash
zmux run 'make dev-stop' -n server             # waits for stop
zmux run 'make dev' -n server -d               # start (detach)
zmux watch server --until "running" -T 60      # wait for ready
```

### Stop a process (Ctrl+C)
```bash
zmux send server C-c
```

### Check before running
```bash
zmux tabs                          # see what tabs exist
zmux watch server -l 5             # peek at output
```

### Commands needing user input (sudo, passwords)
```bash
zmux type admin 'sudo apt update'
```
Then tell the user: "I've put a sudo command in the 'admin' tab — please enter your password."

### Read error output
```bash
zmux watch server -l 200           # last 200 lines for context
zmux watch test -l 100             # full test output
```

## How `--until` works

`zmux watch --until` takes a baseline snapshot of the current buffer when it starts, then only matches against **new output** that appears after. This means stale output from previous commands won't cause false matches.

**Rule of thumb:**
- Command exits? Use `zmux run` with `-T` (has built-in sentinel)
- Server that runs forever? Use `zmux run -d` then `zmux watch --until`

## Session targeting

All commands default to the current tmux session. When multiple sessions exist (e.g. `bridge`, `bridge-b`), commands may go to the wrong one. Always use `-s` when ambiguous:
```bash
zmux run 'make dev' -n server -s bridge
zmux watch server -s bridge
```

## What NOT to do

- Don't run `npm run dev &` or `nohup` — use `zmux run -d`
- Don't use `tmux` directly — use `zmux`
- Don't create unnamed tabs — always use `-n`
- Don't guess at output — use `zmux watch`
- Don't run servers in your shell — use `zmux run`
- Don't `sleep N && zmux watch` — `zmux run` waits by default
- Don't add `echo ":::DONE:::"` markers — `zmux run` handles sentinels automatically
