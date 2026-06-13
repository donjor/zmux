# zmux - roadmap

> Forward plan. Shipped history lives in [../CHANGELOG.md](../CHANGELOG.md).
> Each item is self-contained enough to seed implementation without local
> scratch files or session history.

## Now

### QoL polish first

- [ ] **Top bar repairs stale and blank session/tab state**
  - Dead or killed sessions should disappear from the bar without waiting for a
    later interaction.
  - On init/load, the second tab line should render its actual tabs instead of
    staying blank until a new tab opens.

- [ ] **Picker delete keeps cursor position**
  - In the outside-tmux picker, `ctrl+x` delete currently moves the cursor after
    removal, which makes repeated cleanup awkward.
  - Preserve the nearest stable row after deletion and keep confirm/cancel
    behavior unchanged.

- [ ] **Session tab gets search, numbering, and clearer scope**
  - The dashboard's current-session tab still lacks the search and quick index
    affordances users get in the main picker.
  - Keep the tab explicitly scoped to the active workspace while adding fast
    navigation for sessions and tabs inside it.

- [ ] **Popup focus gets background dimming**
  - The dashboard and scratch shell now have clearer borders, but the underlying
    terminal can still read as active.
  - Add a theme-safe dimming strategy or a documented fallback when tmux/terminal
    constraints make dimming unreliable.

### Agent terminal reliability

- [ ] **`watch --lines` respects the requested capture height**
  - The agent driver path expects small captures when it asks for a bounded
    screen window.
  - Fix the capture path so `--lines` limits output height without breaking idle
    detection or full capture use cases.

- [ ] **Guardrails catch hidden tmux/background invocations**
  - The shared guard corpus covers direct shell surfaces, but nested forms such
    as command substitution, `sh -c`, `xargs tmux`, and here-doc bodies can still
    escape scanning.
  - Extend parsing carefully so normal one-shot shell work stays cheap and false
    positives remain explainable.

## Next

### SSH and nested-zmux support

- [ ] **`zmux ssh <host>` connects and auto-attaches a remote tmux session**
  - Remote work should feel like opening a local workspace, not like manually
    stitching SSH, tmux, and profile state together.
  - Start with a narrow path: connect, detect or create the remote session, and
    attach with clear errors when the remote host is unsupported.

- [ ] **Remote sessions appear in the local picker**
  - A local user should see local and configured remote sessions in one place.
  - The first version can be discovery-only; remote create, rename, and kill can
    follow after attach/switch behavior is reliable.

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

### Workspace and dashboard management

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

### Bar and status customization

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

## Later / unscheduled

- [ ] **Remote create, rename, and kill**
  - Defer until local-to-remote discovery and switching are boring.

- [ ] **File-watcher theme sync**
  - Useful for live theme iteration, but it adds long-running process behavior
    that should wait until the core sync targets are settled.

- [ ] **Additional theme sync targets**
  - Add Alacritty, Kitty, and WezTerm after the target interface has another
    round of real-world use.

- [ ] **Bidirectional theme sync**
  - Potentially powerful, but writing into another tool's config is higher risk
    than the current pull-only model.

- [ ] **Picker behavior configuration**
  - Vim-style navigation and explicit-search modes are useful preferences, but
    should not complicate the default picker path.

- [ ] **Segment ordering**
  - Custom left/right ordering belongs with the custom segment work, after the
    segment model is explicit.

- [ ] **Distribution packages**
  - GitHub releases, Homebrew, AUR, and Nix should wait until the local install
    and upgrade story is stable.

- [ ] **Sentinel-free completion via optional shell integration**
  - A shell hook could report exit status without adding a visible sentinel to
    command history.
  - Keep the printed sentinel as the zero-setup fallback unless the hook proves
    reliable across supported shells.
