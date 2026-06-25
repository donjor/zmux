# zmux - roadmap

> Forward plan. Shipped history lives in [../CHANGELOG.md](../CHANGELOG.md).
> Each item is self-contained enough to seed implementation without local
> scratch files or session history.

## Now — Pane-mode UX & surface parity

Pane-mode (joined tabs) is the live dogfooding front: running several agent
panes in one window surfaced a cluster of readability, layout-control, and
discoverability gaps. Alongside it, two long-lived surfaces — the command
palette and the help menu — have drifted behind the feature set and need to be
pinned canonical so new verbs can't silently go missing.

### Pane-mode UX

- [ ] **Readable pane-border header: `<N> <tab-name> <detail>`**
  - The focused pane shows a raw `%ID` while the others show bare ordinals that
    renumber on focus change — unreadable at a glance, and the pixel/cell sizes
    add noise.
  - Render `<N> <tab-name> <detail>` with the index and label styled apart
    (colour/bold, maybe a colon). Surface the rich per-pane detail (the pane
    title, e.g. an agent's task line) even when a tab is *not* split — feed it to
    the top bar in single-pane mode too. Drop the cell/pixel sizes.

- [ ] **Discoverable layout-edit keys for panes**
  - Moving, swapping, reorienting, and equalizing panes has no obvious binding.
  - Add `prefix+Shift+Arrow` to move/swap a pane, `prefix+s` to toggle split
    orientation (horizontal↔vertical, Hyprland-style), and an
    equalize/normalize action (tmux `select-layout even-*`). `prefix+s` is
    currently only a secondary alias of the workspace+session picker
    (`session.picker`, primary `prefix+w`) and reported unused — reclaim it.
  - Mouse: drag a pane *header* to swap position (drag-on-split already
    resizes); right/double-click a header (and the window) for extra pane
    actions.

- [ ] **Repeatable pane resize**
  - `prefix+Alt+Arrow` resizes one step then consumes the prefix, so resizing is
    a repeated key-mash.
  - Bind the resize keys with tmux's `-r` repeat flag so the arrows keep
    resizing while the prefix is held.

- [ ] **Split-aware helper bar**
  - The prefix helper hints in the top bar don't change when a tab is split.
  - When panes exist, grow the hints with split-specific actions: F (promote to
    full), move, switch orientation, resize, equalize.

- [ ] **Join by index, one-keystroke split, no dead promote-to-full view**
  - Joining a tab as a pane addresses the source by name only — accept a tab
    **index** too.
  - After `prefix+F` (promote to full) the window parks on a `full: sim → tmp-1
    (@60)` confirmation that needs a keypress to clear — drop the interstitial,
    land straight back on the view.
  - Explore a single `prefix+<key>` that creates a new tab *and* drops it in as a
    pane, instead of create-then-join.

### Command palette parity

- [ ] **Palette reflects the full, current command surface**
  - The command palette (`prefix+p`, `internal/tui/palette/`) has fallen behind:
    pane actions are entirely absent, and recent surface changes (the `zws_…`
    workspace/session naming) aren't reflected.
  - Re-derive palette providers from the canonical action/keybinding registry so
    new and renamed verbs appear automatically rather than being hand-maintained.

- [ ] **Test gate: every command is reachable from the palette**
  - Features keep landing without a palette entry — the surface drifts silently.
  - Add a coverage test that fails when a registered command/action has no
    palette provider, mirroring how `TestKeybindingsDocInSync` pins the keys doc.

### Help menu (`prefix+?`)

- [ ] **Generate the help menu from the keybinding registry**
  - `prefix+?` help can drift from the real bindings.
  - Render it from `internal/keys` (the same source as the generated tmux conf
    and `docs/keybindings.md`) so it can't disagree with what's actually bound.

- [ ] **Help menu gets fzf search and working scroll**
  - Add fuzzy search over the entries, and fix the scroll, which is currently
    broken.

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
