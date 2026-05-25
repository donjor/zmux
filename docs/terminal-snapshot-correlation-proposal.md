# Proposal: terminal/window correlation and snapshot targets

Date: 2026-05-01  
Audience: zmux implementation agent

> **Status (2026-05-25):** Phases 1, 3, and 4 are implemented. `zmux terminal
> current --json` resolves strict screenshot geometry (`internal/terminal`), and
> `zmux snapshot` now bundles per-pane text/ANSI + metadata + an optional strict
> PNG into `~/.zmux/snapshots/<timestamp>/` (`internal/snapshot`, exposed via the
> `zmux` skill). The PNG covers only the current terminal — off-current-window
> `--pane` targets get text/ANSI plus a warning, never a mismatched screenshot. Phase 2
> (`zmux terminals` plural listing) remains unbuilt; it isn't needed by snapshot.
> The bundle path is zmux-native (`~/.zmux/snapshots`), not the pi-parley
> `.dump/vision-snapshots` convention.

## Prompt for the implementation agent

Design and implement a small zmux feature that lets downstream tools reliably answer:

> Which real terminal window on the desktop is showing this zmux/tmux session, window, and pane layout?

This is motivated by `pi-clean-ui` visual iteration. It can capture tmux panes as text/ANSI, but real PNG screenshots require knowing the Hyprland/desktop window geometry that actually contains the relevant zmux client. Today every extension has to guess from terminal titles, visible workspaces, active windows, and pane ids. zmux should own that correlation.

Start with a proposal-quality implementation if full cross-platform support is too broad. Hyprland + Ghostty + tmux is enough for the first pass, but keep the API shape portable.

## Problem

A visual debugging loop needs three aligned artifacts:

1. pane text capture (`tmux capture-pane -p`),
2. pane ANSI capture (`tmux capture-pane -p -e`),
3. real terminal PNG screenshot (`grim`, `gnome-screenshot`, X11 tools, etc.).

The first two are easy because tmux knows panes. The third is hard because the screenshot tool sees desktop windows, not tmux panes.

Current heuristics are fragile:

- active window may be a browser or different monitor;
- the largest visible terminal may be the wrong zmux session;
- Hyprland reports geometry for hidden-workspace windows, but `grim -g` captures whatever is currently visible at that rectangle;
- terminal titles are often generic (`Ghostty`) unless zmux/tmux sets stable titles;
- multiple grouped clients can view the same tmux session with different active windows/panes.

This can produce false evidence: the `.ansi` files describe one pane/session while `terminal.png` shows another visible terminal.

## Goals

- Provide a zmux-owned way to list terminal/window candidates with tmux/zmux context and desktop geometry.
- Make current-session/current-client screenshot target selection deterministic enough for agent workflows.
- Prefer visible-window correctness over false positives: if a requested hidden workspace/window cannot be captured, report that instead of screenshotting the wrong window.
- Keep output machine-readable (`--json`) and script-friendly.
- Keep the first implementation small and useful on Hyprland/Ghostty, without blocking future adapters.

## Non-goals

- Do not build a full screenshot viewer/editor.
- Do not require every terminal emulator or window manager to be solved in the first pass.
- Do not mutate pane contents or session state while collecting snapshot metadata.
- Do not hide low-confidence matches behind a successful-looking result.

## Proposed CLI surface

### 1. Stable terminal title metadata

Configure tmux titles so the desktop window title includes zmux identity when possible:

```tmux
set -g set-titles on
set -g set-titles-string "zmux #{session_name}:#{window_index}:#{window_name} #{client_tty}"
```

The exact string should be tested with Ghostty and grouped sessions. Good title properties:

- starts with `zmux` for easy WM filtering;
- includes session name;
- includes active window index/name;
- includes client tty if tmux exposes it reliably;
- stays compact enough for terminal title bars.

Example title:

```text
zmux pi:2:pi-clean-ui /dev/pts/13
```

This alone greatly improves `hyprctl clients -j` matching.

### 2. `zmux terminals --json`

List visible desktop terminal windows that appear to correspond to zmux/tmux clients.

Example:

```bash
zmux terminals --json
```

Output shape:

```json
[
  {
    "id": "hyprland:0x61427584a790",
    "wm": "hyprland",
    "visible": true,
    "confidence": "high",
    "reason": "title contains zmux session/window and geometry is on visible workspace",
    "terminal": {
      "address": "0x61427584a790",
      "class": "com.mitchellh.ghostty",
      "title": "zmux pi:2:pi-clean-ui /dev/pts/13",
      "workspace": "2",
      "geometry": "12,57 2536x1371"
    },
    "tmux": {
      "clientTty": "/dev/pts/13",
      "session": "pi",
      "windowIndex": 2,
      "windowName": "pi-clean-ui",
      "paneIds": ["%58", "%136"],
      "activePaneId": "%58"
    }
  }
]
```

Implementation can start with best-effort fields. It is okay for early output to include `confidence: "medium"` with an explanatory reason when correlation is title/class/geometry based rather than tty-perfect.

### 3. `zmux terminal current --json`

Return the best screenshot target for the current zmux/tmux client.

Example:

```bash
zmux terminal current --json
```

Output:

```json
{
  "ok": true,
  "target": {
    "geometry": "12,57 2536x1371",
    "windowAddress": "0x61427584a790",
    "workspace": "2",
    "visible": true,
    "confidence": "high"
  },
  "tmux": {
    "session": "pi",
    "windowIndex": 2,
    "windowName": "pi-clean-ui",
    "clientTty": "/dev/pts/13"
  }
}
```

If the matched window is hidden or not visible:

```json
{
  "ok": false,
  "error": "matched terminal window is not visible; refusing screenshot geometry",
  "target": {
    "workspace": "8",
    "visible": false
  }
}
```

### 4. Optional later: `zmux snapshot`

Once terminal correlation is reliable, zmux can own the whole snapshot bundle:

```bash
zmux snapshot --current --ansi --text --png --out .dump/clean-ui/snapshots/foo --json
```

Bundle shape:

```text
snapshot/
├── panes/
│   ├── %58.txt
│   ├── %58.ansi
│   ├── %136.txt
│   └── %136.ansi
├── terminal.png
├── terminal.meta.json
└── snapshot.json
```

This can be a second mission. The first mission should focus on stable title/correlation metadata and screenshot target selection.

## Hyprland first-pass design

Use:

```bash
hyprctl clients -j
hyprctl monitors -j
```

Rules:

1. Build the set of visible workspaces from monitor `activeWorkspace.name`.
2. Consider only visible clients for screenshotable targets.
3. Filter terminal clients by class/title: `ghostty`, `kitty`, `alacritty`, `wezterm`, `terminal`, `foot`, `xterm`.
4. Prefer clients whose title starts with or contains the zmux title marker.
5. Correlate against tmux client/session/window data:
   - `tmux list-clients -F ...`
   - `tmux display-message -p ...`
   - `tmux list-panes -a -F ...`
6. Assign confidence:
   - `high`: title includes zmux session/window and workspace is visible;
   - `medium`: visible terminal title/class likely matches but tty/session correlation is incomplete;
   - `low`: fallback largest visible terminal.
7. Never return hidden workspace geometry as screenshotable.

Important: `grim -g` is geometry-based and only captures visible monitor contents. Hidden-workspace matches must be reported as not screenshotable.

## Consumer example: pi-clean-ui

Today `pi-clean-ui` does its own Hyprland selection. With zmux support it could do:

```bash
target=$(zmux terminal current --json)
geometry=$(jq -r '.target.geometry // empty' <<< "$target")
if [ -n "$geometry" ]; then
  grim -g "$geometry" terminal.png
fi
```

Or if `zmux snapshot` exists later, clean-ui can delegate completely.

## Acceptance criteria

- `zmux terminals --json` lists visible terminal candidates with geometry, workspace, title/class, and best-effort tmux context.
- `zmux terminal current --json` returns a usable geometry for the current visible zmux terminal window on Hyprland/Ghostty.
- Hidden-workspace matches do not produce screenshotable geometry without an explicit warning/error.
- Stable terminal title configuration is documented and tested enough that Hyprland clients show zmux session/window identity.
- Output is valid JSON and stable enough for `pi-clean-ui` to consume.
- Existing zmux tests pass.

## Suggested verification

Manual:

```bash
zmux terminals --json | jq .
zmux terminal current --json | jq .
```

Then compare with:

```bash
hyprctl clients -j | jq '.[] | {address,class,title,workspace,at,size}'
tmux list-clients -F '#{client_tty} #{client_session} #{client_width}x#{client_height}'
tmux list-panes -a -F '#{session_name}:#{window_index}.#{pane_index} #{pane_id} #{pane_width}x#{pane_height} active=#{pane_active} title=#{pane_title} current=#{pane_current_command} path=#{pane_current_path}'
```

Screenshot smoke:

```bash
geom=$(zmux terminal current --json | jq -r '.target.geometry')
grim -g "$geom" /tmp/zmux-current-terminal.png
xdg-open /tmp/zmux-current-terminal.png
```

Expected: PNG shows the same terminal window/session that `zmux terminal current --json` reported.

Automated/unit-ish:

- Factor Hyprland JSON parsing/correlation into pure functions.
- Fixture tests for:
  - one visible Ghostty with matching zmux title;
  - multiple visible terminals where only one title matches;
  - hidden workspace matching title (must not be screenshotable);
  - no WM data (graceful unsupported result).

## Open questions

- Can tmux reliably expose enough client tty/window information for grouped sessions with multiple clients?
- How should zmux handle terminals that override or ignore tmux-set titles?
- Should the first implementation be hidden behind `zmux terminal ...` or placed under existing `zmux pane ...` namespace?
- Should `zmux snapshot` use `grim` directly, or should zmux only return geometry and let callers choose screenshot tools?

## Recommendation

Implement in phases:

1. stable terminal title string;
2. `zmux terminals --json` with Hyprland/Ghostty support;
3. `zmux terminal current --json` target selection;
4. later, `zmux snapshot` for full text/ANSI/PNG bundles.

This gives `pi-clean-ui` and future agent-vision workflows a reliable target selector without forcing every extension to replicate wm/tmux correlation heuristics.
