# zmux — Directory Structure (post-omega-merge)

Point-in-time snapshot of the repo **immediately after the omega ideal refactor
merged to master** (merge `26dc7a9`, 2026-05-24). Captured via `find` / `ls`.
This is an immutable stage record — paired with:

- `dir-tree-pre-refactor.md` — the pre-omega snapshot it supersedes
- `dir-tree-ideal-with-context.md` — the code-grounded ideal that omega realized
- `followup-01-cli-extraction.md` — the next convention-alignment plan

**Pruned from this view (still on disk):** `themes/iterm2/` (gitignored cache),
`legacy/v0/` (top-level only; archived bash prototype, now real asset copies),
`.git/`, build artifacts, `.claude/`.

**What changed vs `dir-tree-pre-refactor.md`** (so the diff reads clearly):

- `internal/terminalmeta` → `internal/termtitle`; `internal/terminaltarget` → `internal/terminal` (S5)
- `internal/overmind` extracted from `internal/source` (S6)
- flat `internal/tui` package **dissolved** → `styles`, `workspaceview`, `picker`,
  `tabpicker`, `themepicker`, `wizard` (S3)
- `internal/app` is the composition root; the global `app` is gone (S4)
- `internal/keys` (S1) and `internal/setup` (S2) are new
- `legacy/v0` de-symlinked to real asset copies (S10)

## Tree

```
zmux/
├── cmd/
│   ├── uiproto/
│   │   ├── main.go                   # UI prototyping harness (separate binary)
│   │   └── README.md
│   └── zmux/                         # ⚠ CLI: 35 command files in `package main`
│       │                             #   (flat; followup-01 extracts these → internal/cli)
│       ├── main.go                   # entry: NewRootCmd(app.New()).Execute()
│       ├── root.go                   # cobra root, NewRootCmd(app), flag wiring, shorthand
│       ├── app.go                    # `type App = apppkg.App` alias (wart, for test helpers)
│       ├── errors.go / errors_test.go
│       ├── popup_modes.go            # --picker / --palette / --dashboard / --tab-picker
│       ├── session_picker.go
│       ├── init.go                   # setup wizard (TUI)
│       ├── setup.go                  # NEW (S2): `zmux setup shell` → internal/setup
│       ├── keys.go                   # NEW (S1): hidden `zmux keys gen` (docs codegen)
│       ├── apply.go  status.go  help.go  version.go  completion.go  refresh.go
│       ├── new.go  open.go  kill.go  ls.go  tabs.go  tab.go
│       ├── pane.go  pane_list.go  pane_open.go  pane_resize.go  pane_select.go
│       ├── workspace.go  theme.go  bar.go  bar_adjust.go  bar_render.go
│       ├── terminal.go  run.go  watch.go  send.go
│       └── (tests: cmd, shared, shorthand, attach, dashboard_tab, tab, pane,
│            run, send, watch, terminal, terminal_capabilities, errors)
│
└── internal/
    │  ── Foundation (no zmux deps) ──
    ├── config/                       # TOML load/defaults + FS interface (RealFS)
    ├── debug/                        # opt-in logging (ZMUX_DEBUG=1)
    ├── procfs/                       # /proc process-tree inspection
    ├── tablabel/                     # stable tab-label overlay format
    ├── termtitle/                    # NEW NAME (was terminalmeta): title contract + parser
    │   └── title.go (+ test)
    ├── keys/                         # NEW (S1): keybinding registry — single source of truth
    │   ├── keys.go                   #   types (Binding, Context, Category)
    │   ├── bindings.go               #   prefix/no-prefix/copy-mode tables
    │   ├── inherited.go              #   documented tmux defaults (detach, pane keys)
    │   ├── render.go                 #   Humanize / DisplayKey / ByCategory
    │   ├── doc.go + keybindings.tmpl.md   # GenerateDoc → docs/keybindings.md
    │   └── (keys_test, doc_test ← golden check)
    │
    │  ── Composition root ──
    ├── app/                          # S4: App struct + New() — injected deps, no global
    │   └── app.go                    #   (only file; imported only by cmd/zmux)
    │
    │  ── tmux boundary ──
    ├── tmux/                         # Runner interface, MockRunner, conf.go (reads keys.*)
    │
    │  ── Domain ──
    ├── session/                      # session model, CRUD, templates/
    ├── workspace/                    # workspace state (TOML), reconciliation
    ├── theme/                        # palette, resolver, bundled/ (go:embed)
    ├── bar/                          # status bar; render_<preset>.go; probe.go (Prober seam, C3)
    ├── sync/                         # pull-only theme sync (ghostty, nvim)
    ├── source/                       # discovery (sockets, catalog) + attach fallback
    ├── overmind/                     # NEW (S6): overmind control Client interface
    ├── setup/                        # NEW (S2): shell-rc plan/apply behind config.FS
    │   ├── setup.go                  #   Edit/Plan/Apply, zmux-managed markers, .bak, dry-run
    │   └── shell.go                  #   shell detection + auto-start snippet
    ├── terminal/                     # NEW NAME (was terminaltarget): screenshot-target resolver
    ├── wm/                           # window-manager adapters (hyprland)
    │
    │  ── UI (flat `tui` package dissolved — S3) ──
    └── tui/
        ├── styles/                   # NEW leaf: shared lipgloss styles
        ├── workspaceview/            # NEW leaf: workspace-view data adapter (picker + dashboard)
        ├── picker/                   # primary workspace+session picker (12 files + keymap.go)
        ├── tabpicker/                # Alt+` tab switcher
        ├── themepicker/              # standalone theme picker
        ├── wizard/                   # zmux init setup wizard
        ├── outline/                  # tree-outline component
        ├── views/                    # shared row/column components (11 files)
        ├── palette/                  # command palette (registry, providers, executor)
        └── dashboard/                # tabbed popup (App + Tab interface)
            └── tabs/                 # current, sessions, themes, bar, settings, help (26 files)
```

## Package inventory

| Area | Packages | Notes |
|------|----------|-------|
| `cmd/` | 2 binaries | `zmux` (35 prod files in `package main`), `uiproto` |
| foundation | config, debug, procfs, tablabel, termtitle, keys | no zmux deps |
| boundary | tmux | the `Runner` seam |
| domain | session, workspace, theme, bar, sync, source, overmind, setup, terminal, wm | |
| composition | app | single-file; imported only by `cmd/zmux` |
| `tui/` | styles, workspaceview, picker, tabpicker, themepicker, wizard, outline, views, palette, dashboard(+tabs) | flat `tui` gone |

## Known structural smells (this snapshot ≠ final ideal)

1. **`cmd/zmux` holds 35 command files in `package main`** — logic living in a
   launcher package; not importable or externally testable. The well-regarded Go
   CLIs (gh, hugo, cobra-cli, kubectl) keep `main` thin + commands in an
   importable package. → `followup-01-cli-extraction.md`.
2. **`internal/app` is a one-file package** plus a `type App = apppkg.App` alias
   in `cmd/zmux/app.go` that exists only so `package main` tests can write
   `&App{}`. Resolves naturally once the command layer becomes a real package.
3. Optional seams still open: source-discovery prober (deferred in C3); B-purity
   items (overmind package-wrappers → injected `Client`, `terminal.go`
   test-injection globals, cmd tests could use a memFS instead of `RealFS`).

These are **cleanliness/convention**, not correctness. The plan is in
`followup-01-cli-extraction.md`.
