# `zmux terminal current`

`zmux terminal current --json` resolves the visible desktop terminal window that contains the invoking tmux client/pane. It is designed for visual tooling that needs safe screenshot geometry, such as `pi-clean-ui`.

The command is intentionally strict: it returns a screenshot target only when zmux can validate a machine-readable tmux title marker against live tmux client state, Hyprland window metadata, and local process ancestry.

## Contract

```bash
zmux terminal current --json
```

Successful output includes screenshot geometry:

```json
{
  "schemaVersion": "zmux-terminal-current/v1",
  "ok": true,
  "status": "ok",
  "reason": "matched visible terminal window with validated zmux:v1 title metadata",
  "target": {
    "wm": "hyprland",
    "windowAddress": "0x61427584a790",
    "geometry": "12,57 2536x1371",
    "workspace": "2",
    "visible": true,
    "confidence": "high"
  }
}
```

Expected refusal statuses use `ok:false` and do not expose screenshotable geometry:

- `not_in_tmux` — the command was not invoked from a tmux pane.
- `unsupported` — required tmux or window-manager metadata is unavailable.
- `not_found` — no desktop window exposes matching `zmux:v1` title metadata.
- `hidden` — a matching window exists, but is not visible on an active Hyprland monitor workspace.
- `ambiguous` — more than one visible window exposes the same validated marker.

## Title metadata

Generated zmux tmux config owns this title marker:

```tmux
set -g set-titles on
set -g set-titles-string "zmux:v1;tty=#{client_tty};sid=#{session_id};wid=#{window_id};pane=#{pane_id} #{?#{@zmux_session_label},#{@zmux_workspace}/#{@zmux_session_label},#{session_name}}:#{window_index}:#{window_name}"
```

Only the `zmux:v1;...` token is parsed. The human-readable suffix is
informational; managed sessions show `workspace/session` labels when tmux
metadata is present and fall back to the raw tmux session name otherwise.

Correlation uses raw tmux IDs and does not normalize grouped sessions such as `dev-b` to `dev`. After the title token matches, zmux also verifies that the tmux client process is a descendant of the Hyprland terminal process before returning geometry.

## Manual smoke test

```bash
zmux apply
hyprctl clients -j | jq '.[] | {address,class,title,workspace,at,size}'
zmux terminal current --json | jq .
```

If the command returns `ok:false,status:not_found`, confirm your terminal allows tmux to set the window title and that the active title contains `zmux:v1`.
