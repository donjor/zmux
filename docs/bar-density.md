# Bar density & font scale (feasibility)

Finding for the roadmap item _"Independent bar/header font scale — investigate
feasibility"_ (Bar layout & density, intake 2026-06-03). Companion to the
always-2-line + de-dup work in the same thread.

**TL;DR:** zmux **cannot** give the bar its own font size — a terminal cell grid
has no per-region font. What the request really wants (more information without
the bar swallowing tabs) is a **density** problem, and zmux already owns every
density lever. The realistic deliverable is a bundled _compact mode_, not a font
scale.

## Why an independent font scale isn't possible

tmux — and therefore the zmux bar, which _is_ tmux's status line — renders into
a **fixed character-cell grid**. Every cell is one glyph in the terminal
emulator's configured font, at the emulator's cell size. tmux has no concept of
fonts, point sizes, or sub-cell scaling, and there is no escape sequence to ask
the emulator to draw one row (the status line) at a different size than the rest
of the screen.

So font/cell sizing is wholly the **emulator's** job:

| Concern                              | Owner                               |
| ------------------------------------ | ----------------------------------- |
| Font family, weight, size, ligatures | Terminal emulator (Ghostty, etc.)   |
| Cell width/height (px)               | Terminal emulator                   |
| Per-region / per-row font scale      | **Nobody** — not a terminal feature |
| What fills each cell, how many cells | tmux / **zmux**                     |

A terminal can change _its_ global font size, but that scales the whole grid
(panes included), not the bar alone. "Bar at 0.8×" is not expressible.

## What zmux _can_ control: density

Density = information per cell. These are the achievable levers, most already
shipped:

1. **Layout** — `two-line` (default) vs `split`; both give the workspace/session
   row its own line so it never crowds the tabs. The 1-row `single` layout was
   removed (it reflowed and repeated info across the line); legacy `single`
   configs normalize to two-line.
2. **Row ownership / de-dup** — in two-line mode the top row owns
   workspace/session identity, and cwd lives in a right-aligned top overlay
   beside the edge-profile badge. The bottom-left drops to stable volatile aux
   only (process/git, preset-dependent), so directory changes cannot move the
   logical tabs row. This is the single biggest tab-survival win — it freed
   ~40–68 cells of bottom-left width, plus the later cwd-overlay cleanup.
3. **Segment toggles** — hide any of workspace / git / lang / clock / directory /
   process / group. Fewer segments → narrower sides → more tab room.
4. **Glyph vocabulary (preset)** — nerd-font pills/separators (powerline,
   rounded) pack more signal per cell but cost a nerd font; plain presets
   (minimal, zen, blocks) are wider-character but font-agnostic. Choosing a
   denser preset _is_ the closest thing to "smaller bar."
5. **Truncation** — `shortenDir` keeps the top-row cwd overlay compact, while
   `status-left/right-length` caps and tmux's native `<`/`>` overflow markers
   with `list=focus` keep the current window visible when tabs exceed their
   budget.
6. **Indicator compaction** — session indicator as dots (`○●○`), numbers
   (`2/3`), or none.
7. **Pane header detail** (automatic) — every pane, including a single-pane
   window, uses the tmux pane-border header for `<index> <name> <detail>`. The
   detail is the pane title (e.g. an agent's task line), so single-pane and split
   views now share one label surface instead of moving lone-pane titles into the
   right status bar.
8. **Split-aware prefix hints** (automatic) — while the prefix is held the right
   bar shows a hint cluster; a split window appends orient / move / even keys so
   pane-layout controls surface only when they apply.

## Recommendation

Independent font scale: **won't do** (categorically impossible in a cell grid;
document and close). Pair with the emulator's own font-size config if a globally
smaller terminal is wanted.

Density: the levers above already exist but are scattered across config keys. The
actionable follow-up — _not_ part of this feasibility item — is a single
**compact mode** that bundles a sensible dense default (e.g. two-line + de-dup +
trimmed segments + a compact preset) behind one switch, so "make the bar denser"
is one decision instead of six. Captured here for a future roadmap promotion.
