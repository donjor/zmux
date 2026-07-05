# Bar density

A domain note for status-bar density, pane headers, and the rejected idea of a
separate bar font scale.

## Owned paths

- `internal/bar/**` — status-line rendering, presets, segment budgets, tab-row display.
- `internal/tmux/conf.go` — generated tmux options that shape status lines and pane headers.
- `internal/keys/**` — prefix hints and help surfaces that expose bar/pane layout controls.
- `docs/reference/keybindings.md` — generated keybinding reference that must stay in sync with `internal/keys`.

## Invariants

- zmux cannot set a separate font size for the tmux status line. tmux renders a fixed terminal cell grid, so font size belongs to the terminal emulator.
- Bar density means information per cell: layout, segment toggles, glyph vocabulary, truncation, and indicator compaction.
- Two-line layout gives workspace/session identity its own row so tab labels do not fight volatile status segments.
- Pane-border headers carry `<index> <name> <detail>` for both single-pane and split views.
- Cwd/status changes must not push the logical tab row around.

## Reusable primitives

- Bar presets and segment definitions live in `internal/bar/bar.go`.
- Rendering lives in `internal/bar/render*.go` and `internal/bar/generate.go`.
- `internal/bar/render_context.go` gathers live context.
- `internal/bar/preview.go` and preview pages support dashboard/bar previews.
- Keybinding hints come from `internal/keys`, not duplicated strings.

## Split-logic warnings

- Do not try to express bar font scale in zmux config; it cannot be implemented in tmux.
- Do not revive a one-row `single` layout without proving it preserves tab width and de-duplicates workspace/session/cwd context.
- Keep pane-header and status-line ownership separate: pane details belong in pane headers, volatile global status belongs in the status bar.
- Visual density changes need real terminal grounding because string width, ANSI style, and tmux truncation interact.

## Update triggers

Update this doc when bar layout, presets, segment toggles, pane-header format,
status-line truncation, prefix hints, or the compact-mode roadmap changes.

## Feasibility finding

Independent bar font scale is a **won't do**. A terminal can change its global
font size, but that scales panes and status lines together. There is no terminal
escape sequence or tmux option for drawing one row at a different point size.

Density is the workable product direction. The existing levers are:

1. layout (`two-line` vs split variants);
2. row ownership and de-duplication;
3. segment toggles;
4. preset/glyph vocabulary;
5. truncation and status length caps;
6. session indicators (`dots`, `numbers`, or none);
7. pane-header detail;
8. split-aware prefix hints.

The likely future feature is a **compact mode** that bundles the density levers
behind one user-facing switch.
