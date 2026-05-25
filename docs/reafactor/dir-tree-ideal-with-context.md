# zmux — Ideal Directory Structure (code-informed, true ideal)

Starts from [`dir-tree-ideal-blind.md`](dir-tree-ideal-blind.md) (structure
derived from docs alone) and revises it using knowledge of the **actual
codebase** — real types, import edges, file sizes, package responsibilities
(cross-checked against [`dir-tree-pre-refactor.md`](dir-tree-pre-refactor.md)).

**This is a *true* ideal, not a migration plan.** The only admissible
constraints are **design quality** and **language mechanics that survive
restructuring**. In particular:

- An import cycle is **attacked** (extract a leaf / contract package), never
  accepted as a reason to keep two things fused or split the wrong way.
- "It's bash today", "the convention says X", "that'd be churn", "low-ROI" are
  **not** admissible — those are migration cost, which is explicitly out of
  scope here. (For the pragmatic, effort-aware target, that's a separate
  exercise.)

Reviewed with buddy (stable) — including a deliberate pass to stop the pendulum
swinging from status-quo bias into the opposite ditch of speculative
over-abstraction. Where buddy and I judged a blind proposal to be gold-plating,
it's **cut** below, not adopted.

## Verdict

The blind tree had the **right instincts** on several domain packages that an
earlier code-grounded pass wrongly rejected on effort grounds. Corrected for
design merit only, the true ideal:

- **Adopts** what blind proposed and effort-bias had killed: keybinding registry
  (scoped), setup-in-Go (scoped), per-surface TUI packages, explicit app
  composition.
- **Refines** blind's terminal merge into the correct **contract-leaf +
  resolver** layering (which also happens to dissolve the import cycle).
- **Splits** the overmind control client out of discovery.
- **Cuts** genuine over-abstraction — both blind's and the temptation to go
  further: no bar segment/preset *plugin* architecture, no single registry for
  every TUI keypress.

Net: the true ideal moves **further from current** than the pragmatic target
would, but only where a real design gain (single source of truth, correct
layering, isolation, testability) justifies it.

---

## Shifts from the blind tree

Grouped by disposition. Each carries **design-only** rationale.

### ADOPT — blind was right; earlier effort-bias wrongly rejected these

#### S1. `internal/keys/` — keybinding registry (scoped to global actions)

A single source for **global zmux actions**: prefix bindings, no-prefix
bindings, popup launchers. It **projects** into three consumers that today drift
independently:

- the `set -g bind-key …` strings emitted by `tmux/conf.go`,
- the help/keybinding reference rendered in the dashboard + `zmux help`,
- `docs/keybindings.md` (generated, not hand-maintained).

**Scope (the correction):** `internal/keys/` owns *global* action↔binding
mapping only. **Component-local** Bubble Tea keys (a picker's `j/k`, a wizard's
field nav) stay in their component packages — forcing every local keypress
through one registry would be indirection for its own sake.

Design gain: the prefix/no-prefix/popup matrix in `docs/keybindings.md` becomes
provably in sync with what the binary emits.

#### S2. `internal/setup/` — first-run setup in Go (scoped to user setup)

A single-binary tool's setup belongs **in the binary**, not in a parallel bash
implementation. `internal/setup/` owns: `~/.zmux` dir creation, initial config
write, shell-rc integration planning/apply, theme bootstrap. `zmux init`
orchestrates it.

**Scope (the correction):** *user* setup only. **Maintainer/dev** concerns —
symlinking the Claude skill and Pi extension into `~/.claude` / `~/.pi` — are
**not** product setup; they stay in `dev.sh`. `install.sh` remains a thin
bootstrap (build the binary, hand off to `zmux init`).

Design gain: setup is testable Go behind `config.FS`, not untested shell.

#### S3. TUI surfaces become real packages

`tui/picker/`, `tui/wizard/`, `tui/tabpicker/`, `tui/themepicker/` each own a
package; shared lipgloss styles move to a `tui/styles/` leaf; global action keys
come from `internal/keys` (S1). `tui/dashboard/`, `tui/palette/`, `tui/outline/`,
`tui/views/` are already packages and stay.

Design gain: each surface has an explicit public surface and isolated state;
consistency with the already-subpackaged surfaces. (The earlier "export churn"
objection is migration cost — out of scope here.)

Note: blind missed `themepicker` entirely — it's a **fifth** standalone surface,
distinct from the dashboard's themes *tab*.

#### S4. `internal/app/` + explicit composition — **flagged tension**

True-ideal wiring is an explicit composition root: an `App` struct holding
injected dependencies (`tmux.Runner`, `config.FS`, stores, theme resolver),
constructed once and passed in — `cmd/zmux` builds `NewRootCmd(app)` with **no
mutable package-global**.

**This contradicts a stated project convention** (`CLAUDE.md`:
"no `NewApp()` in cobra commands — use the global `app`";
`docs/architecture.md` → global `app` in `root.go`). Calling it out rather than
quietly overriding: a package-global mutable singleton is the kind of thing a
true ideal revises (testability, explicit dependencies, no init-order coupling).
**If** the convention is a deliberate ergonomics choice rather than an accreted
artifact, it can stand — but the ideal architecture is explicit DI. Decision
deferred to the maintainer; documented as a genuine fork, not assumed.

### REFINE — blind's direction was right, the shape was wrong

#### S5. Terminal: `termtitle` (contract leaf) + `terminal` (resolver)

Blind wanted one `internal/terminal/`. The right ideal is **two packages by
layer**, and the layering — not the (real) import cycle — is the reason:

- `internal/termtitle/` — pure **contract leaf**: the `set-titles-string`
  format constant plus `Parse`/`Validate`/`Matches`. No zmux deps. (This is
  today's `terminalmeta`, renamed for what it is.)
- `internal/terminal/` — the **resolver** for `zmux terminal current`; imports
  `tmux` + `wm` + `procfs` + `termtitle`. (Today's `terminaltarget`, renamed.)

The conf generator (`tmux/conf.go`) and the resolver both depend on the title
*contract*; that contract is a genuine third thing, so it's its own leaf. The
cycle blind would have created (`tmux → terminal → tmux`) simply never forms.
Splitting a pure contract from a heavy resolver is correct layering on its own
merits.

#### S6. `internal/overmind/` split from `internal/source/`

Two different concerns blind (and current) conflate:

- `internal/source/` — **discovery**: scan tmux sockets, build the process
  table, correlate, probe liveness, assemble the `Catalog`. A discovery-scoped
  `Provider` interface (overmind-detection, future providers) lives here.
- `internal/overmind/` — the overmind **control client**: `Connect`, `Restart`,
  `Stop`, `Logs`. This is process-supervisor control, not discovery, and
  deserves its own package.

### CUT — over-abstraction (blind's, and the tempting overcorrection)

#### S7. `internal/bar/segments/` and `internal/bar/presets/` plugin architecture → **cut**

Blind proposed segment/preset sub-packages; the maximalist trap is to go further
and build a `Segment` interface + `Renderer` interface + registry for
*third-party* presets and an exported renderer ecosystem.

That's gold-plating. The segment set is **fixed and config-toggled**
(`config.go` `[bar.segments]`); presets are a **fixed built-in set** (README's
9). The true ideal for a fixed, closed set is an **internal table/registry in
one package** with **file-per-preset** for navigability — which is essentially
the current shape. No exported plugin surface, no third-party renderer claims.

So `internal/bar/` stays flat: `preset.go` (enum/table), `render_<preset>.go`
(file each; `render_powerline.go` covers powerline **and** rpowerline), segment
helpers in `render_context.go`. This is one place the honest answer is "current
≈ ideal" — stated as a design conclusion, not inertia.

#### S8. `internal/tmux/conf/` sub-package → **cut**

`conf.go` is a cohesive ~250-line emitter (`GenerateConf`/`WriteConf`). Splitting
a coherent small file into a package is anti-ideal. The `tmux` package is
already correctly decomposed (`runner`/`client`/`mock`/`endpoint`/`parse`/
`process`/`types`/`clipboard`/`conf`). In the ideal it additionally consumes
`internal/keys` (S1) for binding strings and `termtitle` (S5) for the title
format.

#### S9. `internal/tab/` (folding in `tablabel`) → **cut**

`internal/tablabel/` is a correct **leaf** (`Format`/`PlainFormat`) consumed by
low-level packages (`tmux/conf`, `bar/generate`) and commands. Wrapping it in a
higher-level `internal/tab/` would pull command/tmux deps into a leaf. Tab
*command* logic stays thin in `cmd/zmux/`. The leaf is already the ideal shape.

### CLARIFY — blind misread the asset directories

#### S10. Embed sources are the single source of truth; v0 assets are separate

- `internal/session/templates/*.toml` and `internal/theme/bundled/*` are the
  `//go:embed` sources — the **only** first-class asset roots.
- Top-level `templates/*.sh` and `themes/bundled/*` are **legacy v0** assets,
  reachable only via `legacy/v0/{templates,themes}` symlinks. In the ideal,
  `legacy/v0/` owns **real** copies (symlinks removed).
- `themes/iterm2/` is a **gitignored dev cache**, not architecture — runtime
  themes resolve under `~/.zmux/themes`. It does not appear in the ideal tree.

---

## The true-ideal tree

`◆` = differs from current on design grounds. `▲` = differs **and** revises a
stated convention (see S4).

```
zmux/
├── cmd/
│   ├── zmux/                       # cobra command layer — thin; no package-global
│   │   ├── main.go
│   │   ├── root.go            ▲    # NewRootCmd(app) — explicit composition, no global `app`
│   │   ├── errors.go
│   │   ├── popup_modes.go          # --picker / --palette / --dashboard / --tab-picker
│   │   ├── session_picker.go
│   │   ├── init.go  apply.go  status.go  help.go  version.go  completion.go  refresh.go
│   │   ├── new.go   open.go   kill.go    ls.go    tabs.go
│   │   ├── tab.go                  # tab move|label|refresh-names|kill ; session kill
│   │   ├── pane.go pane_list.go pane_open.go pane_resize.go pane_select.go
│   │   ├── workspace.go  theme.go
│   │   ├── bar.go bar_adjust.go bar_render.go
│   │   ├── terminal.go             # current | capabilities | refresh
│   │   ├── run.go  watch.go  send.go     # send hosts `send` + `type`
│   │
│   └── uiproto/                    # UI prototyping harness (not shipped)
│
├── internal/
│   ├── app/                  ▲    # ◆ composition root: App struct + injected deps
│   │   └── app.go                  #   (replaces the package-global in cmd/zmux)
│   │
│   ├── keys/                 ◆    # global action↔binding registry (S1)
│   │   ├── actions.go              #   Action set (prefix / no-prefix / popup)
│   │   ├── bindings.go             #   binding tables
│   │   └── render.go               #   projects → tmux-conf strings + docs/help table
│   │
│   ├── tmux/                       # tmux boundary (already well-decomposed)
│   │   ├── runner.go  client.go  mock.go
│   │   ├── endpoint.go  parse.go  process.go  types.go  clipboard.go
│   │   └── conf.go                 # consumes internal/keys + internal/termtitle
│   │
│   ├── config/
│   │   ├── config.go  load.go  fs.go
│   │
│   ├── setup/                ◆    # first-run USER setup in Go (S2)
│   │   ├── dirs.go                 #   ~/.zmux scaffold
│   │   ├── config.go               #   initial config write
│   │   ├── shell.go                #   shell-rc integration
│   │   └── theme.go                #   theme bootstrap / iterm2 fetch
│   │
│   ├── session/
│   │   ├── session.go  actions.go  root.go  template.go  embed.go
│   │   └── templates/              # EMBED SOURCE (dev/claude/webdev/monitor .toml)
│   │
│   ├── workspace/
│   │   ├── types.go  migrate.go  store.go  store_helpers.go
│   │   ├── store_lifecycle.go  store_sessions.go  store_workspaces.go
│   │
│   ├── theme/
│   │   ├── theme.go  palette.go  resolver.go  apply.go  write.go  download.go  embed.go
│   │   └── bundled/                # EMBED SOURCE (11 themes)
│   │
│   ├── bar/                        # flat, fixed set — NO plugin architecture (S7)
│   │   ├── bar.go  preset.go  generate.go  render.go  render_context.go
│   │   ├── render_default.go … render_zen.go    # file-per-preset
│   │   ├── render_powerline.go     # powerline AND rpowerline
│   │   ├── multisession.go  preview.go  apply.go
│   │
│   ├── source/                     # DISCOVERY only (S6)
│   │   ├── catalog.go              #   Catalog / Source / SourceGroup
│   │   ├── discover.go             #   ◆ optionally → discover_{sockets,procs,probe}.go
│   │   └── provider.go             #   ◆ discovery-scoped Provider interface
│   │
│   ├── overmind/             ◆    # CONTROL client, split from source (S6)
│   │   └── overmind.go             #   Connect / Restart / Stop / Logs
│   │
│   ├── sync/                       # pull-only theme sync
│   │   ├── sync.go  target.go  ghostty.go  nvim.go
│   │
│   ├── termtitle/            ◆    # CONTRACT LEAF (was terminalmeta) (S5)
│   │   └── title.go                #   format const + Parse/Validate/Matches
│   │
│   ├── terminal/             ◆    # RESOLVER (was terminaltarget) (S5)
│   │   └── current.go              #   imports tmux+wm+procfs+termtitle
│   │
│   ├── tablabel/                   # LEAF — kept as-is (S9)
│   │   └── label.go
│   │
│   ├── wm/                         # window-manager adapters
│   │   ├── types.go  hyprland.go
│   │
│   ├── procfs/
│   │   └── inspector.go
│   │
│   ├── debug/
│   │   └── debug.go
│   │
│   ├── tui/                        # per-surface PACKAGES (S3)
│   │   ├── styles/           ◆    #   shared lipgloss leaf (was flat styles.go)
│   │   ├── picker/           ◆    #   model/update/view/search/actions/outline/external
│   │   ├── wizard/           ◆    #   wizard/views/data/steps
│   │   ├── tabpicker/        ◆    #   Alt+` switcher
│   │   ├── themepicker/      ◆    #   standalone theme picker (blind missed it)
│   │   ├── outline/                #   tree-outline component (already a pkg)
│   │   ├── views/                  #   shared row/column components (already a pkg)
│   │   ├── palette/                #   command palette (already a pkg)
│   │   └── dashboard/              #   tabbed popup (already a pkg)
│   │       ├── app.go tab.go chrome.go layout.go messages.go reqid.go
│   │       └── tabs/               #   current/sessions/themes/bar/settings/help (+shared_*)
│   │
│   └── preview/                    # UI prototype framework (used by cmd/uiproto)
│       ├── framework.go  controls.go  chrome.go  styles.go
│       ├── bar/   pane/
│
├── tests/                          # integration tests (build tag: integration)
│   ├── integration_test.go  testdata/
│
├── skills/zmux/                    # Claude Code skill (symlinked elsewhere by dev.sh)
├── pi-extension/                   # Pi agent TS extension (separate build)
│   ├── index.ts  package.json  tsconfig.json  src/  docs/  test/
│
├── docs/   (architecture, vision, roadmap, keybindings*, terminal-*, refactor/)
│
├── legacy/v0/                ◆    # owns its OWN templates/ + themes/ (de-symlinked, S10)
│
├── Makefile  install.sh  dev.sh  go.mod  go.sum  .gitignore
└── README.md  CONTRIBUTING.md  CLAUDE.md
```

\* `docs/keybindings.md` is **generated** from `internal/keys` in the ideal (S1).

---

## Dependency layering (why the package set is coherent)

```
leaves (no zmux deps):   termtitle, tablabel, procfs, debug, config, keys
boundary:                tmux (← keys, termtitle), wm
domain:                  session, workspace, theme, bar (← tmux, theme, tablabel),
                         source (← tmux), overmind, sync (← theme), terminal (← tmux, wm, procfs, termtitle)
orchestration:           setup (← config, theme), app (← everything it wires)
ui:                       tui/* (← domain, keys, styles), preview
entry:                    cmd/zmux (← app), cmd/uiproto (← preview)
```

No cycles. The two places current code risks one — the terminal title contract
and the keybinding strings — are resolved by leaves (`termtitle`, `keys`) that
both the boundary and higher layers may import.

---

## Out of scope (deliberately)

- **Migration cost / sequencing.** This document is the destination, not the
  route. How much churn S1–S6 cost, and in what order to land them, is a
  separate planning exercise.
- **Per-file splits below the package level**, except where they change the
  package boundary (e.g. the optional `discover_*.go` split is noted but not
  load-bearing).

## What the ideal *fixes* that current does not

Stated plainly so the ideal isn't confused with current:

- **I/O behind seams becomes real, not aspirational.** Today git/lang detection
  shells out directly in `bar/render_context.go`, and `source` calls `ps` /
  `overmind` inline. The ideal routes these through interfaces (a `vcs`/`lang`
  probe behind config.FS-style seams; `overmind` control behind its package
  boundary) so the architecture's "all side-effects behind interfaces" claim is
  actually true.
- **Keybinding drift becomes impossible** (S1): one registry, generated docs.
- **Setup is testable** (S2): Go behind `config.FS`, not unverified bash.
- **Composition is explicit** (S4): no package-global; dependencies are visible
  and injectable — pending the maintainer's call on the documented convention.
```
