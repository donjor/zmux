# zmux - roadmap

> Forward plan. Shipped history lives in [../CHANGELOG.md](../CHANGELOG.md).
> Each item is self-contained enough to seed implementation without local
> scratch files or session history.

## Now

The command-palette and help-menu surface-parity work shipped (the palette is
re-derived from the action/keybinding registry behind a coverage gate, and
`prefix+?` is rebuilt as a scrollable, fuzzy-filterable viewer that renders from
the shared help source so it can't drift from `zmux help`), along with the core
pane-mode UX — readable headers, layout-control keys, repeatable resize,
split-aware hints, and index join. See the changelog. Three threads remain.

### Agent addressability — snapshot/tab-pane label resolution

- [ ] **`snapshot --pane` and bare `tab pane` honour the shared tab resolver**
  - `snapshot --pane <label>` treats its arg as a raw tmux pane id, so a tab
    label (`sim`) fails lookup, and it names captured panes by command (`ssh-2`)
    instead of the pinned label (`sim`). Route each `--pane` arg through
    `resolveTabTarget` / `tabs.Resolve` and name the target from the resolved tab
    label (command as fallback) — a one-file fix in `internal/cli/snapshot.go`
    that closes both the lookup and the naming gap.
  - Bare `zmux tab pane <tab>` errors "current window is not a zmux tab" from a
    window `zmux where` reports as that tab; make its implicit-host resolution
    reuse the same window→logical-tab lookup `where` uses.
  - This is the CLAUDE.md "route address changes through `tab_target.go`" gotcha
    — `snapshot` is the verb that skips it.

### Residual pane-mode UX

- [ ] **Mouse pane-header interactions**
  - Dragging a pane *header* should swap the two panes' positions, and right- or
    double-clicking a header should open per-pane actions. Drag-to-resize on the
    split border already works; this extends the direct-manipulation model to the
    header strip itself.

- [ ] **One-keystroke tab→pane**
  - A single `prefix+<key>` that creates a new tab and joins it as a pane in one
    motion, instead of the current create-then-join two-step.

### Agent workspace — reuse active panes, harden tab↔pane join

- [ ] **Reuse an active joined pane for long-running tasks**
  - When a session already holds an active joined pane, agents and skill doctrine
    should route new long-running work into it rather than spawning a fresh tab —
    an open pane is usually deliberate, holding a long-running task the user wants
    visible. Needs the agent/skill layer (zmux skill, peer/worker doctrine) to
    prefer the existing pane over a new `run -n <name>` tab.

- [ ] **Make the tab→pane join model solid end-to-end**
  - The core pane/tab unification — create, join, promote-to-full, and address —
    must behave consistently across the CLI, the TUI, and the agent skill, so the
    "joining" concept means the same thing everywhere it is referenced.

## Next

### Remote & nested zmux

Make zmux feel native across SSH and nested layers. Builds on the unified local
entry/creation/management surface (`internal/tui/workspaceoutline` +
`workspace.CreateManagedSession`).

- [ ] **`zmux ssh <host>` connects and auto-attaches a remote tmux session**
  - Remote work should feel like opening a local workspace, not like manually
    stitching SSH, tmux, and profile state together.
  - Start with a narrow path: connect, detect or create the remote session, and
    attach with clear errors when the remote host is unsupported.

- [ ] **Remote sessions appear in the local picker**
  - A local user should see local and configured remote sessions in one place.
  - The first version can be discovery-only; remote create, rename, and kill can
    follow once attach/switch behavior is reliable (see Later → Remote
    management).

- [ ] **Nested zmux gets explicit prefix, bar, and theme coordination**
  - Connecting into a host that also runs zmux needs predictable key handling
    and visual cues for outer vs inner layers.
  - Favor a small compatibility contract over broad terminal-emulator tricks.

### Session persistence

- [ ] **Save and restore session layouts**
  - Users should be able to recover windows, panes, and working directories
    after a tmux restart.
  - Keep the first version layout-only; do not replay arbitrary commands.

- [ ] **Handle disconnects more gracefully**
  - Client drops and network blips should not leave zmux state confusing or
    stale.
  - Reuse existing workspace/session reconciliation before adding new state.

### Workspace & session lifecycle

- [ ] **Dashboard Workspaces tab supports full CRUD**
  - Creating, renaming, and deleting workspaces from the dashboard should match
    the CLI and picker behavior.
  - Keep validation and conflict errors visible in-place.

- [ ] **`zmux fork <session>` promotes session branching**
  - Scratch extraction exists for tabs, but session-level branching still needs
    a first-class command.
  - Implement the already-decided shape with conservative naming and attach
    behavior.

- [ ] **Workspace members can include grouped sessions**
  - Multi-monitor workflows need several viewports over the same logical
    session without corrupting workspace membership.
  - Build on the existing session-group clone model and keep labels rooted.

### Bar & status customization

- [ ] **Compact mode bundles the density levers**
  - Users asking for a smaller bar are really asking for more information per
    terminal cell.
  - Offer one switch that combines dense layout, trimmed segments, compact
    indicators, and a suitable preset.

- [ ] **Status bar adapts based on workspace/session type**
  - Different workspaces care about different signals.
  - Start with simple configured indicators before attempting automatic
    inference.

- [ ] **User-defined custom status segments**
  - Built-in segments cover common cases, but project-local status often needs a
    small command or file-backed indicator.
  - Define a TOML shape for command, timeout, cache behavior, and left/right
    placement.

## Later / unscheduled

### Remote management

- [ ] **Remote create, rename, and kill**
  - The write side of remote sessions; defer until local-to-remote discovery and
    switching (Next) are boring.

### Theme sync expansion

- [ ] **File-watcher theme sync**
  - Useful for live theme iteration, but it adds long-running process behavior
    that should wait until the core sync targets are settled.

- [ ] **Additional theme sync targets**
  - Add Alacritty, Kitty, and WezTerm after the target interface has another
    round of real-world use.

- [ ] **Bidirectional theme sync**
  - Potentially powerful, but writing into another tool's config is higher risk
    than the current pull-only model.

### Edge profile polish

- [ ] **`zzmux` can read shared theme and recipe libraries**
  - The edge profile is intentionally isolated, but read-only fallback to common
    user libraries would reduce duplicate setup.
  - Keep writes profile-local so testing never mutates live `zmux` state.

- [ ] **Profile-aware display strings avoid hardcoded `zmux`**
  - Some help and ghost-prompt text still names the live binary even when the
    edge binary dispatches correctly.
  - Make display-only command examples follow the active profile where that does
    not leak implementation detail into packages that should stay generic.

### Picker & segment preferences

- [ ] **Picker behavior configuration**
  - Vim-style navigation and explicit-search modes are useful preferences, but
    should not complicate the default picker path.

- [ ] **Segment ordering**
  - Custom left/right ordering belongs with the custom segment work, after the
    segment model is explicit.

### Run & watch reliability

- [ ] **Sentinel-free completion via optional shell integration**
  - A shell hook could report exit status without adding a visible sentinel to
    command history.
  - Keep the printed sentinel as the zero-setup fallback unless the hook proves
    reliable across supported shells.

### Distribution & packaging

- [ ] **Distribution packages**
  - GitHub releases, Homebrew, AUR, and Nix should wait until the local install
    and upgrade story is stable.
