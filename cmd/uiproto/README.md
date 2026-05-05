# uiproto — zmux UI prototyping harness

A Bubbletea TUI for iterating on zmux UI visuals outside production
rendering paths. It now has a stronger design-lab shell inspired by the
Ayu/lipgloss patterns in `~/donjor/play/cli-showcase`: hero chrome,
pill tabs, rounded panels, badges, metric cards, sparklines, and richer
control styling. Today it hosts the **status bar** preview page and a
**pane visual system** preview page. Future pages can cover dashboard
rows, picker chrome, etc.

## Running

```bash
go run ./cmd/uiproto
```

Resize your terminal to test how each mode behaves at different widths
— the preview re-renders using `tea.WindowSizeMsg`.

## Keybindings

| Key | Action |
|-----|--------|
| `tab` / `shift+tab` | switch between pages |
| `↑` / `↓` or `j` / `k` | focus next/previous control |
| `←` / `→` or `h` / `l` | adjust the focused control (cycle choices, decrement/increment ints) |
| `space` / `enter` | toggle booleans / cycle choices |
| `q` / `esc` / `ctrl+c` | quit |

## Pages

### Status bar page

Renders the status bar at your current terminal width in five modes:

| Mode | Behavior |
|------|----------|
| `classic` | Today's live renderer (`internal/bar.RenderPreview`). Baseline. |
| `dots` | Single-line, `third ●○○` compact position indicator. |
| `pills` | Single-line, `main · third* · two` session list with truncation. |
| `two-line` | Two rows when >1 session in the workspace; one row otherwise. |
| `split` | Top row focused on workspace scope, bottom on active session. |

Controls:

- **mode** — pick the display mode
- **preset** — pick one of nine bar presets (default, minimal, powerline, …)
- **sessions** — 1–9 sessions in the workspace
- **current** — which 1-based session is the current one
- **names** — realistic fixture sets (short, realistic, long, mixed, agent-workflow)
- **clones** — whether to render `-b`/`-c` clone sessions in the list
- **show ws / show git / show clock** — per-segment visibility toggles

### Pane system page

Prototypes pane-local chrome for first-class zmux pane workflows in
general. `pi-clean-ui` is included only as an optional example consumer:
zmux owns pane lifecycle/chrome, while the app inside a pane owns its own
UI. This is intentionally non-production rendering while the visual
language settles.

Controls:

- **layout** — split, grid, stacked, or focus+rail pane arrangement
- **workload** — coding, services, review, or agent fixture content
- **focus** — which pane slot is focused (`primary`, `secondary`, `tertiary`)
- **header** — compact, verbose, or ribbon pane header treatment
- **divider** — subtle, strong, or rounded split-line styling
- **hints** — whether local shortcut hints appear in the pane header
- **clean-ui ex** — swap the secondary pane into a current clean-ui-like sidecar example
- **aux state** — running, attention, or stale/degraded state for the secondary pane
- **aux %** — width allocated to the secondary/right-side pane in split/grid scenarios

Design intent:

- Pane headers carry generic pane-local context: title, pane id, command,
  cwd, size, state, and relevant shortcuts.
- Focus is shown locally through header emphasis, pane gutter, and split
  line treatment.
- The main zmux status bar stays quiet; pane-specific shortcuts live with
  the pane chrome they affect.
- Sidecars are just one pane workload. The optional clean-ui example borrows
  current sidecar cues — operations cockpit, watch/tasks/artifacts tabs,
  metrics, live/degraded status — without making them zmux primitives.

Production constraint:

- Treat headers as single-line candidates for tmux `pane-border-status` /
  `pane-border-format`.
- Treat gutters and split emphasis as a visual stand-in for tmux active
  border styling, not as an arbitrary in-pane overlay.
- Shortcut labels distinguish no-prefix zmux bindings (`A-S-→`) from
  inherited tmux prefix bindings (`pfx+z`, `pfx+q`).

## Architecture

Everything generic lives in `internal/preview/`:

- `framework.go` — `Page`, `Control`, `RenderContext`, `App` (bubbletea model)
- `controls.go` — `ChoiceControl`, `IntControl`, `ToggleControl`
- `chrome.go` — hero, tabs, controls panel, preview canvas, footer helpers
- `styles.go` — prototype-only Ayu-inspired palette, badges, metric cards,
  and sparkline helpers

The bar page lives in `internal/preview/bar/`:

- `page.go` — the `Page` implementation wiring controls to renderers
- `fixtures.go` — realistic session name sets
- `draft/multisession.go` — **draft** renderers for dots/pills/two-line/split.
  Lives under `draft/` so we can iterate freely during Phase 0; graduates
  to `internal/bar/` in Phase 1+ once the visuals are settled.

The pane system page lives in `internal/preview/pane/`:

- `page.go` — non-production pane chrome renderer and fixtures for generic
  split/grid/stacked/focus-rail pane scenarios, plus an optional clean-ui
  sidecar example fixture.

## Adding a new page

Implement `preview.Page`:

```go
type MyPage struct {
    ctrls []preview.Control
}

func (p *MyPage) ID() string                       { return "mypage" }
func (p *MyPage) Title() string                    { return "My Page" }
func (p *MyPage) Controls() []preview.Control      { return p.ctrls }
func (p *MyPage) Render(ctx preview.RenderContext) string { ... }
```

Then register it in `cmd/uiproto/main.go`:

```go
app := preview.NewApp(
    barpage.New(),
    mypage.New(),
)
```
