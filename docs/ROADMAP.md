# zmux - roadmap

> Forward plan. Shipped history lives in [../CHANGELOG.md](../CHANGELOG.md).
> Each item is self-contained enough to seed implementation without local
> scratch files or session history.

## Now — Picker ↔ dashboard convergence

The workspace+session picker and the dashboard have drifted into two
implementations of the same logic and now fight each other — a regression and a
gap fell out of the recent sessionless-fallback work. Converge them onto shared
logic so entry, creation, and management behave the same everywhere.

- [ ] **Bare `zmux` lands on the picker, even with no live session**
  - Running `zmux` explicitly should open the workspace+session picker whether or
    not a session is live; on a fresh reboot (no live session) it currently drops
    to the sessionless dashboard instead.
  - Keep the sessionless dashboard as the *automatic* fallback — closing the last
    session, or shell-startup auto-attach with nothing to attach — not for the
    explicit invocation.
  - Requires the picker to handle the empty state (pick or create a workspace,
    create the first session) so it can own the explicit entry path.

- [ ] **Create a new session under a workspace from the dashboard**
  - The dashboard has no entry point to create a new session within an existing
    workspace; the picker and CLI do.
  - Add it and match the picker's validation, naming, and attach behavior.

- [ ] **Consolidate the picker and dashboard onto shared workspace/session logic**
  - `internal/tui/picker/` and `internal/tui/dashboard/` reimplement the same
    workspace/session listing, creation, and switching, and the two drift apart;
    the bare-`zmux` regression and the missing dashboard create are symptoms.
  - Lift the shared model and actions into one place both surfaces render, and
    make the two items above its first consumers rather than separate one-offs.
  - This subsumes the earlier dashboard-vs-init-picker "bespoke logic" cleanup and
    sets up the Next → Dashboard Workspaces full-CRUD item as a thin consumer.

## Next

### Remote & nested zmux

Make zmux feel native across SSH and nested layers. Deferred behind the
picker/dashboard convergence so remote entry builds on a unified local surface.

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
    switching (Now) are boring.

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
