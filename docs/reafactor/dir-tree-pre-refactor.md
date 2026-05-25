# zmux — Current Directory Structure

Snapshot of the repo as it actually exists today. Captured via `tree` /
`find` / `ls` — no edits beyond pruning generated/vendored noise. Paired with
`dir-tree-ideal-blind.md` for refactor planning.

**Pruned from this view (still on disk):**

- `themes/iterm2/` — large downloaded cache, gitignored (hundreds of dirs)
- `legacy/v0/` — only top-level shape shown; archived bash+gum prototype
- `.git/`, build artifacts (`./zmux`), `.claude/worktrees/`

## Tree

```
zmux/
├── cmd/
│   ├── uiproto/
│   │   ├── main.go
│   │   └── README.md
│   └── zmux/                       # CLI entry — cobra root + commands (flat)
│       ├── main.go
│       ├── root.go
│       ├── app.go                  # global `app` wiring
│       ├── errors.go
│       ├── errors_test.go
│       ├── shared_test.go
│       ├── cmd_test.go
│       ├── shorthand_test.go
│       ├── popup_modes.go          # --picker / --palette / --dashboard / --tab-picker
│       ├── session_picker.go
│       ├── attach_test.go
│       ├── dashboard_tab_test.go
│       │
│       ├── init.go
│       ├── apply.go
│       ├── status.go
│       ├── help.go
│       ├── version.go
│       ├── completion.go
│       ├── refresh.go
│       │
│       ├── new.go
│       ├── open.go
│       ├── kill.go
│       ├── ls.go
│       ├── tabs.go
│       ├── tab.go
│       ├── tab_test.go
│       │
│       ├── pane.go
│       ├── pane_list.go
│       ├── pane_open.go
│       ├── pane_resize.go
│       ├── pane_select.go
│       ├── pane_test.go
│       │
│       ├── workspace.go
│       ├── theme.go
│       ├── bar.go
│       ├── bar_adjust.go
│       ├── bar_render.go
│       │
│       ├── terminal.go
│       ├── terminal_test.go
│       ├── terminal_capabilities_test.go
│       │
│       ├── run.go
│       ├── run_test.go
│       ├── watch.go
│       ├── watch_test.go
│       ├── send.go
│       ├── send_test.go
│       └── type.go                 # (not present — `type` is wired via tab/send?)
│
├── internal/
│   ├── bar/                        # status bar generation + render + preview
│   │   ├── bar.go
│   │   ├── apply.go
│   │   ├── generate.go
│   │   ├── generate_test.go
│   │   ├── multisession.go
│   │   ├── preset.go
│   │   ├── preview.go
│   │   ├── preview_test.go
│   │   ├── render.go
│   │   ├── render_context.go
│   │   ├── render_test.go
│   │   ├── render_default.go
│   │   ├── render_minimal.go
│   │   ├── render_powerline.go
│   │   ├── render_blocks.go
│   │   ├── render_rounded.go
│   │   ├── render_hacker.go
│   │   ├── render_starship.go
│   │   └── render_zen.go
│   │   #  NOTE: no render_rpowerline.go file — rpowerline shares with powerline
│   │   #  NOTE: no presets/ subdir; render_* files sit flat in internal/bar/
│   │   #  NOTE: no segments/ subdir; segment logic lives inside render_context.go
│   │
│   ├── config/
│   │   ├── config.go
│   │   ├── fs.go
│   │   ├── load.go
│   │   └── load_test.go
│   │
│   ├── debug/
│   │   ├── debug.go
│   │   └── debug_test.go
│   │
│   ├── preview/                    # UI prototype framework
│   │   ├── framework.go
│   │   ├── chrome.go
│   │   ├── controls.go
│   │   ├── styles.go
│   │   ├── bar/
│   │   │   ├── fixtures.go
│   │   │   ├── page.go
│   │   │   └── draft/
│   │   │       └── multisession.go
│   │   └── pane/
│   │       ├── page.go
│   │       ├── page_fixtures.go
│   │       ├── page_layouts.go
│   │       └── page_util.go
│   │
│   ├── procfs/
│   │   ├── inspector.go
│   │   └── inspector_test.go
│   │
│   ├── session/
│   │   ├── session.go
│   │   ├── session_test.go
│   │   ├── root.go
│   │   ├── root_test.go
│   │   ├── actions.go
│   │   ├── actions_test.go
│   │   ├── template.go
│   │   ├── template_test.go
│   │   ├── embed.go
│   │   └── templates/              # //go:embed bundled session templates
│   │       ├── claude.toml
│   │       ├── dev.toml
│   │       ├── monitor.toml
│   │       └── webdev.toml
│   │
│   ├── source/                     # external session discovery
│   │   ├── catalog.go
│   │   ├── discover.go
│   │   ├── discover_test.go
│   │   └── overmind.go
│   │
│   ├── sync/                       # pull-only theme sync targets
│   │   ├── sync.go
│   │   ├── target.go
│   │   ├── ghostty.go
│   │   ├── ghostty_test.go
│   │   ├── nvim.go
│   │   └── nvim_test.go
│   │
│   ├── tablabel/                   # stable zmux tab-label overlay
│   │   ├── label.go
│   │   └── label_test.go
│   │
│   ├── terminalmeta/               # stable terminal title metadata writer
│   │   ├── metadata.go
│   │   └── metadata_test.go
│   │
│   ├── terminaltarget/             # `zmux terminal current` window correlation
│   │   ├── current.go
│   │   └── current_test.go
│   │
│   ├── theme/
│   │   ├── theme.go
│   │   ├── theme_test.go
│   │   ├── apply.go
│   │   ├── download.go
│   │   ├── download_test.go
│   │   ├── embed.go
│   │   ├── palette.go
│   │   ├── palette_test.go
│   │   ├── resolver.go
│   │   ├── resolver_test.go
│   │   ├── write.go
│   │   └── bundled/                # //go:embed 11 themes
│   │       ├── atom-one-dark/
│   │       ├── ayu-dark/
│   │       ├── carbonfox/
│   │       ├── catppuccin-mocha/
│   │       ├── dracula/
│   │       ├── gruvbox-dark/
│   │       ├── kanagawa-dragon/
│   │       ├── material-darker/
│   │       ├── nord/
│   │       ├── rose-pine/
│   │       └── tokyonight/
│   │
│   ├── tmux/                       # typed tmux CLI boundary
│   │   ├── runner.go
│   │   ├── mock.go
│   │   ├── client.go
│   │   ├── endpoint.go
│   │   ├── endpoint_test.go
│   │   ├── process.go
│   │   ├── parse.go
│   │   ├── parse_test.go
│   │   ├── types.go
│   │   ├── clipboard.go
│   │   ├── clipboard_test.go
│   │   ├── conf.go                 # generated tmux.conf emitter (flat, not in conf/)
│   │   ├── conf_test.go
│   │   └── split_pane_test.go
│   │
│   ├── tui/
│   │   ├── tui.go
│   │   ├── styles.go
│   │   ├── keymap.go
│   │   │
│   │   ├── picker.go               # workspace picker (flat, not in picker/)
│   │   ├── picker_types.go
│   │   ├── picker_update.go
│   │   ├── picker_view.go
│   │   ├── picker_view_help.go
│   │   ├── picker_view_list.go
│   │   ├── picker_view_templates.go
│   │   ├── picker_search.go
│   │   ├── picker_actions.go
│   │   ├── picker_outline.go
│   │   ├── picker_external.go
│   │   ├── picker_test.go
│   │   ├── picker_behavior_test.go
│   │   │
│   │   ├── tabpicker.go            # Alt+` tab switcher (flat, not in tabpicker/)
│   │   ├── tabpicker_test.go
│   │   │
│   │   ├── themepicker.go          # standalone theme picker (flat)
│   │   ├── themepicker_test.go
│   │   │
│   │   ├── wizard.go               # init wizard (flat, not in wizard/)
│   │   ├── wizard_data.go
│   │   ├── wizard_steps.go
│   │   ├── wizard_views.go
│   │   ├── wizard_test.go
│   │   │
│   │   ├── outline/                # tree-outline component
│   │   │   ├── tree.go
│   │   │   ├── tree_test.go
│   │   │   ├── nav.go
│   │   │   ├── nav_test.go
│   │   │   ├── row.go
│   │   │   ├── row_test.go
│   │   │   ├── scroll.go
│   │   │   └── scroll_test.go
│   │   │
│   │   ├── palette/                # command palette (prefix+p)
│   │   │   ├── model.go
│   │   │   ├── model_test.go
│   │   │   ├── action.go
│   │   │   ├── action_test.go
│   │   │   ├── executor.go
│   │   │   ├── executor_test.go
│   │   │   ├── providers.go
│   │   │   ├── providers_test.go
│   │   │   ├── registry.go
│   │   │   ├── registry_test.go
│   │   │   └── testhelpers_test.go
│   │   │
│   │   ├── views/                  # shared row/column components
│   │   │   ├── header.go
│   │   │   ├── input.go
│   │   │   ├── actions.go
│   │   │   ├── confirm.go
│   │   │   ├── depcheck.go
│   │   │   ├── sessionlist.go
│   │   │   ├── sessionrow.go
│   │   │   ├── sessionrow_test.go
│   │   │   ├── windowrow.go
│   │   │   ├── tabbar.go
│   │   │   ├── swatch.go
│   │   │   ├── colorpicker.go
│   │   │   └── colorpicker_test.go
│   │   │
│   │   └── dashboard/              # tabbed popup (prefix+Space)
│   │       ├── app.go
│   │       ├── app_test.go
│   │       ├── tab.go
│   │       ├── chrome.go
│   │       ├── layout.go
│   │       ├── messages.go
│   │       ├── reqid.go
│   │       └── tabs/
│   │           ├── current.go      # "Session" tab — current workspace/session
│   │           ├── current_test.go
│   │           ├── current_data.go
│   │           ├── current_tree.go
│   │           ├── current_tree_render.go
│   │           ├── current_overlay.go
│   │           ├── current_actions.go
│   │           ├── current_actions_edit.go
│   │           ├── current_actions_kill.go
│   │           ├── current_actions_window.go
│   │           │
│   │           ├── sessions.go     # "Workspaces" tab
│   │           ├── sessions_test.go
│   │           ├── sessions_tree.go
│   │           ├── sessions_actions.go
│   │           ├── sessions_overlay.go
│   │           │
│   │           ├── themes.go
│   │           ├── themes_test.go
│   │           ├── themes_data.go
│   │           ├── themes_picker.go
│   │           ├── themes_editor.go
│   │           │
│   │           ├── bar.go
│   │           ├── bar_test.go
│   │           ├── bar_view.go
│   │           ├── bar_helpers.go
│   │           │
│   │           ├── settings.go
│   │           ├── settings_test.go
│   │           │
│   │           ├── help.go
│   │           │
│   │           ├── scroll.go               # shared
│   │           ├── mode_state.go           # shared
│   │           ├── shared_mutations.go
│   │           ├── shared_mutations_test.go
│   │           ├── shared_overlay.go
│   │           └── shared_overlay_test.go
│   │
│   ├── wm/                         # window-manager adapters
│   │   ├── types.go
│   │   ├── hyprland.go
│   │   └── hyprland_test.go
│   │
│   └── workspace/
│       ├── types.go
│       ├── migrate.go
│       ├── store.go
│       ├── store_helpers.go
│       ├── store_lifecycle.go
│       ├── store_sessions.go
│       ├── store_workspaces.go
│       └── store_test.go
│
├── tests/                          # integration tests (build tag: integration)
│   ├── integration_test.go
│   └── testdata/
│       ├── test-theme
│       └── test.toml
│
├── skills/
│   └── zmux/                       # NOTE: real dir, not a symlink — contains SKILL.md
│       └── SKILL.md
│
├── pi-extension/                   # Pi agent TS extension
│   ├── index.ts                    # NOTE: also has top-level index.ts alongside src/
│   ├── package.json
│   ├── tsconfig.json
│   ├── src/
│   │   ├── index.ts
│   │   ├── tools.ts
│   │   ├── classify.ts
│   │   ├── config.ts
│   │   ├── shell.ts
│   │   ├── zmux.ts
│   │   └── respawn-continuation.ts
│   ├── docs/
│   │   └── config.md
│   └── test/
│       └── run.mjs
│
├── docs/
│   ├── README.md
│   ├── VISION.md
│   ├── ROADMAP.md
│   ├── architecture.md
│   ├── keybindings.md
│   ├── pi-zmux-extension.md
│   ├── terminal-capabilities.md
│   ├── terminal-current.md
│   ├── terminal-snapshot-correlation-proposal.md
│   └── reafactor/                  # (typo — "refactor") current refactor planning
│       ├── dir-tree-current.md
│       └── dir-tree-ideal-blind.md
│
├── templates/                      # top-level user-facing template scripts (NOT the embedded TOMLs)
│   ├── claude.sh
│   ├── dev.sh
│   ├── monitor.sh
│   └── webdev.sh
│
├── themes/
│   ├── bundled/                    # mirror of internal/theme/bundled/ — possible dup source
│   │   ├── atom-one-dark/
│   │   ├── ayu-dark/
│   │   ├── carbonfox/
│   │   ├── catppuccin-mocha/
│   │   ├── dracula/
│   │   ├── gruvbox-dark/
│   │   ├── kanagawa-dragon/
│   │   ├── material-darker/
│   │   ├── nord/
│   │   ├── rose-pine/
│   │   └── tokyonight/
│   └── iterm2/                     # downloaded cache (gitignored, hundreds of dirs)
│
├── legacy/
│   └── v0/                         # archived bash+gum prototype
│       ├── bin/
│       │   ├── zmux0
│       │   └── zmux0-apply-theme
│       ├── lib/
│       │   ├── help-popup.sh
│       │   ├── init.sh
│       │   ├── startup-info.sh
│       │   ├── status.sh
│       │   ├── sync.sh
│       │   └── theme.sh
│       ├── tmux/
│       │   └── zmux.tmux.conf
│       ├── templates -> ../../templates
│       ├── themes -> ../../themes
│       ├── install.sh
│       └── README.md
│
├── .github/
│   └── workflows/
│       └── ci.yml
│
├── Makefile
├── install.sh
├── dev.sh
├── go.mod
├── go.sum
├── .gitignore
├── README.md
├── CONTRIBUTING.md
└── CLAUDE.md
```

## Observations vs. `dir-tree-ideal-blind.md`

These stand out at a glance — call-outs for the refactor pass, not judgements:

- **`internal/bar/` is flat.** All `render_<preset>.go` files sit at the package
  root; there is no `presets/` subdir. No `segments/` subdir either — segment
  logic lives inside `render_context.go`. There is no `render_rpowerline.go`
  (rpowerline appears to share code with `render_powerline.go`).
- **`internal/tui/` is mostly flat.** `picker.go`, `tabpicker.go`,
  `themepicker.go`, and `wizard.go` (with their per-file companions) all live
  at the package root rather than in their own subpkgs. Only `outline/`,
  `palette/`, `views/`, and `dashboard/` are nested.
- **`internal/tmux/conf.go` is flat, not in `conf/`.** Generated-conf emission
  is a single file at the package root.
- **No `internal/tab/` package.** Tab-related logic is split between
  `internal/tablabel/` (label overlay) and command files in `cmd/zmux/tab*.go`;
  tmux-window operations sit inside `internal/tmux/`.
- **Terminal feature is split into two packages.**
  `internal/terminalmeta/` (title writer) and `internal/terminaltarget/`
  (window correlation for `zmux terminal current`). The ideal tree proposes a
  single `internal/terminal/`.
- **No standalone `internal/keys/` package.** Keybinding source-of-truth is in
  `internal/tmux/conf.go` (generated config) and `internal/tui/keymap.go`
  (TUI bindings) — two places, plus `docs/keybindings.md`.
- **Two `templates/` directories with different content.**
  `internal/session/templates/*.toml` are the embedded session templates
  (dev/claude/webdev/monitor as TOML). Top-level `templates/*.sh` are user-
  facing shell scripts with the same names — these are *not* the embedded
  TOMLs and the relationship is not obvious from the file tree.
- **Two `themes/bundled/` directories.** `internal/theme/bundled/` is the
  `//go:embed` source. Top-level `themes/bundled/` looks like a duplicate
  source mirror. Risk of drift — one of these is dead.
- **`pi-extension/` has both a top-level `index.ts` and a `src/index.ts`.**
  Two entry points where one would do.
- **`skills/zmux/` is a real directory in this repo**, not the symlink that
  `dev.sh`/`install.sh` creates elsewhere on the user's machine.
- **`docs/reafactor/`** is currently misspelled (`refactor` → `reafactor`).
- **No `internal/setup/`** for non-UI wizard helpers; wizard logic is entirely
  inside `internal/tui/wizard*.go`.

These are the deltas the ideal tree would address. Specific consolidations
(merge two `templates/`, kill one `themes/bundled/`, split out `setup/`,
collapse `terminalmeta`+`terminaltarget` into `terminal/`, group `tui/`
picker/wizard/tabpicker into subpkgs) can be sequenced as separate refactor
steps from here.
