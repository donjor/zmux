# zmux CLI catalog (cold reference)

Full command tables for sessions/workspaces, tabs, placements, panes, terminal
capabilities, snapshots, output recording, and config. The hot operational loop (run/watch/send,
raw-tmux guard, tab states, peer/worker) lives in `SKILL.md`; this is the
lookup-when-you-need-it reference, routed from there at point of use. In Pi,
prefer the one `zmux_lite` dispatcher over shelling out. Its session/tab/peer,
pane, runtime, callback, evidence, input, and lifecycle operations cover the
agent surface below.

## Sessions & workspaces

```bash
zmux ls [workspace]              # list workspaces, or sessions within one
zmux ls -s                       # flat list of all sessions
zmux where                       # current context: workspace/session/tab/pane/cwd (alias: whoami)
zmux where --json                # same, machine-readable — `session_tmux` is the raw -s handle
zmux new <workspace> [session…] # create workspace + sessions, attach (alias: n)
zmux session run <session> -n <tab> [--workspace <ws>] [--cwd <dir>] -- <cmd…>
                                 # create a DETACHED session, run <cmd> as its first tab
                                 # (no focus steal, no blank tab) — worker/background spawn
zmux run <recipe>               # recipe form with cwd/workspace/session defaults
zmux run <recipe> -y            # run recipe defaults without prompting
zmux run <recipe> --dry-run     # print the exact recipe plan
zmux recipe list                # list bundled and user recipes
zmux recipe show <recipe>       # inspect a recipe TOML
zmux recipe lint [recipe]       # validate recipes
zmux recipe fork <recipe>       # copy a bundled recipe for human editing
zmux recipe edit <recipe>       # edit a user recipe (human-maintenance surface)
zmux open <ws> [session]         # open/attach a workspace session (aliases: attach, a)
zmux open <ws> <session> --hijack   # take over a session attached elsewhere (advanced)
zmux fork <new-session-label> [--dir path]
                                 # human/worktree setup: copy current session tab names/order into a new local session
zmux kill <name>                 # smart kill, workspace-first (alias: k)
zmux session kill <session>      # kill a single session explicitly

zmux ws list                     # workspaces and their sessions
zmux ws add <workspace> <session>   # tag a session to a workspace
zmux ws remove <session>         # untag a session
zmux ws show <workspace>         # sessions in a workspace
zmux ws kill <workspace>         # kill a workspace and all its sessions
```

`zmux session run` is the orchestration-safe worker-spawn primitive: it creates
the session detached in the current (or `--workspace`) workspace and launches
the command as window 1, so it never steals the caller's focus and never leaves
a blank shell tab. The command must follow `--`. It errors if the session
already exists, and never makes the new session the workspace's default attach
target. Contrast `zmux new` (attaches by design) — see the worker doctrine in
`references/agent-worker.md`.

### Session targeting (`-s`)

`-s <session>` is accepted by `run`, `wait`, `watch`, `send`, `type`, `tabs`, `log`,
`tab state`, `tab status`, `tab inspect`, `tab peer ensure`, `tab move`, `tab kill`, and the `tab pane/full/hide/show` placements. (`open` takes the
workspace/session positionally; `zmux pane list --session --target <session>` uses
`--target` for the same session forms, while `pane list --target <session>` without `--session` stays a raw tmux pane/window target.) It accepts three
forms:

- **bare session label** — `-s server`
- **`workspace/session`** — `-s app/main`
- **raw tmux name** — `-s zws_app__main` (debug/interop)

A bare label that exists in more than one workspace is **refused, not guessed** —
qualify it as `workspace/session`. Omit `-s` and the **current** session is used
(error when run outside tmux). `zmux where` prints the raw name to pass here.

## Tabs

```bash
zmux tabs [session]              # list tabs — riders nested under hosts, hidden marked ~ (alias: t)
zmux tab move <tab> <dest-session>  # move a tab to another session in the workspace
zmux tab label '<label>'         # set a stable zmux label for the current tab
zmux tab label ''                # clear the label
zmux tab status <tab> --json     # read glyph, command, and peer lifecycle state
zmux tab inspect <tab> --json    # state + output tail + warnings in one result
zmux tab peer ensure <tab> --command '<cmd>' --json  # safe peer create/reuse
zmux tab kill <tab>              # kill a tab in the current session
zmux reap --dry-run              # preview reaper decisions; human-gated cleanup/maintenance
```

Labels are zmux overlays (`label [auto-name]`); they don't disable tmux's automatic
window renaming.

### Placements (pane / full / hide / show)

A zmux tab is a **stable logical unit** (id + label + state), not a window slot. It
can live as a full window, as a pane inside another tab, or hidden in the dock —
and `send`/`type`/`watch`/`wait`/`run -n` keep reaching it by name in every placement.

```bash
zmux tab pane <tab>                      # join <tab> as a pane beside your current tab (focus-safe)
zmux tab pane <tab> --focus              # human path: join and select the moved pane
zmux tab pane <tab> --into <host> --down --size 30%   # explicit host + geometry
zmux tab full <tab>                      # promote a pane back to its own tab (--after: next to old host)
zmux tab hide <tab>                      # park it off the bar — process keeps running
zmux tab show <tab>                      # bring it back to the session it was hidden from (focus-safe)
zmux tab show <tab> --focus              # human path: rejoin and select it
zmux tab kill <tab> -s <session>         # kill a tab by name in a source session
```

States and labels ride along: a `running` glyph set on a full tab is still there
when it's a pane or hidden. Placement verbs refuse while grouped viewports
(`-b`/`-c` clones) are attached — detach the extra clients first. Human keybindings
and palette placement actions pass `--focus`; agent/tool calls omit it by default.

## Panes & sidecars

```bash
zmux pane current --json         # current pane id + details
zmux pane list --json            # panes in current window (also --session, --all, -q)
zmux pane open <name> -r 35 -- <cmd>   # split right at 35%; focuses by default
zmux pane open --no-focus <name> -r 35 -- <cmd>   # agent/tool path: don't select it
zmux pane open --label-tab <name> -r 35 -- <cmd>   # preserve tab label across split
zmux pane toggle <name> -r 35 -- <cmd> # open if absent, close if present (--focus/--replace)
zmux pane focus <pane>           # focus by id/title/index
zmux pane close <pane>           # close by id/title/index
zmux pane resize <pane> --size 40%   # CLI width resize; Pi tool auto-selects width/height unless axis is forced
```

Split direction: `-r` right, `-l` left, `-d` down, `-u` up (each takes a size).
Use `--label-tab` for sidecars that would otherwise let tmux's auto-rename clobber
the conceptual tab label. From a detached agent shell, prefer the pane id printed
by `pane open` for follow-up `resize`/`close`; title/index lookup is scoped to the
current target window, and unrelated `run` commands can move that current window.

## Terminal capabilities

```bash
zmux terminal current --json       # the visible desktop terminal target (e.g. for screenshots)
zmux terminal capabilities --json  # diagnose RGB/truecolor path (alias: caps)
zmux terminal refresh              # reattach current client to re-resolve RGB features
```

`zmux terminal refresh` (and `zmux refresh` below) **reattaches/redraws the current
client** — it can disturb an active agent connection. Don't run it from an automated
session unless the user asked or disruption is acceptable; otherwise tell the user to
run it.

## Visual snapshots

Capture terminal/TUI evidence when the *visual* state matters — debugging a TUI,
showing a render, or grounding work on another app you're driving in a pane.

```bash
zmux snapshot                       # all panes in current window: text + ANSI + PNG
zmux snapshot --no-png              # text + ANSI only (no screenshot)
zmux snapshot --pane %5 --pane %6   # specific panes (PNG only if both are current-window)
zmux snapshot --lines 400 --json    # more scrollback; print result as JSON
zmux snapshot --out /tmp/run1       # custom output dir
```

Each run writes a bundle to `~/.zmux/snapshots/<timestamp>/` (override with `--out`):
`<pane>.txt`, `<pane>.ansi` (colour-preserving), `<pane>.meta.json`, an optional
`terminal.png`, and `snapshot.json` + `manifest.json` + `README.md`. Read `.ansi`
with `less -R`, `.txt` for plain parsing; `snapshot.json` lists every artifact,
the `modalities` captured, and any `warnings`.

The PNG only ever covers the **current** terminal window (zmux resolves its geometry
strictly and refuses hidden/ambiguous windows rather than screenshotting the wrong
one). It's captured only when every requested pane is in the current window; target
a pane elsewhere with `--pane` and the PNG is skipped (text/ANSI still captured).
Check `warnings` and report blind spots rather than trusting a missing screenshot
as evidence.

## Output recording (`zmux log`)

Persistent, background recording of a tab's output stream to a **bounded** file. It
keeps recording with no client attached (tmux `pipe-pane`) and self-truncates so
disk never runs away — use it to walk away and read the stream back later. Contrast
`zmux watch` (reads the live buffer only) and `zmux snapshot` (one-shot screen state). For lifecycle/command/peer state, prefer `zmux tab status --json`, `zmux tab inspect --json`, or structured `zmux wait --for turn:/cmd:` over reading screen output.

```bash
zmux log start <tab>                  # begin recording to a bounded file (background)
zmux log start <tab> --ansi           # keep ANSI colour/escapes instead of stripping to plain
zmux log start <tab> --max-bytes 4096 # cap before oldest output is dropped (default 1 MiB)
zmux log start <tab> -s <session>     # target a tab in another session
zmux log status                       # global recording view; no -s/--session
zmux log tail <tab>                   # print the recorded log (already bounded)
zmux log tail <tab> -n 50             # last 50 lines only
zmux log stop <tab>                   # stop recording
```

Logs land in `<state-dir>/logs/` (`~/.zmux/logs/`). The cap keeps only the trailing
`--max-bytes`, trimmed at a line boundary — oldest dropped, newest retained, no
rotation files. Best for **line-oriented** output (servers, builds, tests); a
fullscreen TUI records as escape soup even with stripping. For live following of a
running tab use `zmux watch <tab> -f`.

## Config & maintenance

```bash
zmux status                      # current theme, bar, prefix, sync target, session count
zmux apply                       # regenerate tmux.conf + apply theme/bar (non-disruptive)
zmux refresh                     # apply config + reattach current client (disruptive — see above)
zmux keys                        # keybinding help
```

Cosmetic/user-facing surfaces — drive these only when the user explicitly asks or
for config troubleshooting, not as part of agent ops:

```bash
zmux theme set <name>            # set theme directly (also: list, sync, pull <target>)
zmux bar [preset]                # list presets, or set one (also: bar show)
zmux init                        # interactive setup wizard — MUST be run outside tmux
```

## Naming conventions

Stable, descriptive tab names so `run -n`/`send`/`type`/`watch` keep reaching them:

- `server` — dev servers
- `test` — test runners/watchers
- `build` — builds
- `logs` — log tails
- `admin` — sudo/interactive commands
- `<tool>-sidecar` — UI sidecars
