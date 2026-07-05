# zmux - roadmap

> Forward plan. Shipped history lives in [../CHANGELOG.md](../CHANGELOG.md).
> Each item is self-contained enough to seed implementation without local
> scratch files or session history.

## Now

### Agent collaboration QoL

Make agent-driven zmux work feel like supervised terminal collaboration, not a
pile of bespoke `zmux run` / `watch` / `tab status` loops.

- [ ] **Pi extension exposes first-class peer/agent inspection tools**
  - In progress on `feat/zmux-agent-qol`.
  - Agents should be able to inspect a peer tab, start/reuse a peer, type a
    prompt, and wait for fresh lifecycle evidence without hand-rolled shell
    loops.
  - Ship typed tools for status-plus-tail inspection, peer ensure/readiness,
    wait-aware typing, and wait/idle runtime logs; update the bash classifier and
    extension tests so agent CLIs route through those tools.

- [ ] **Agent/human pane handoff chooses and reuses the right shared surface**
  - When an agent needs human input such as a sudo password or manual command,
    inspect the current window size, tabs, panes, and existing scratch/shared
    surfaces before creating anything new.
  - Reuse an idle scratch or joined pane when one exists; never reuse a pane that
    is running something.
  - Agent mode: portrait terminals should split horizontally with roughly 70% for
    the agent and 30% for the shared surface; landscape terminals should split
    side-by-side. Non-agent usage should get the same context-aware helper, but
    default to an even portrait split and focus the new/joined surface.

### Human-driving QA harness

- [ ] **`zmux test` / `zzmux test` drives real interaction flows**
  - Build a pass/fail suite that uses real keypresses / `send-keys` behavior, not
    internal command shortcuts, against isolated `zmux-test` / `zzmux-test`
    sessions.
  - Cover creating, switching, renaming, and deleting workspaces, sessions, tabs,
    dashboard surfaces, panes, joining, and promoting.
  - Maintain a command/feature → human-driving-step map so missing test surfaces
    are deterministic gaps, and compose those steps into reusable example flows.

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

## Next

### Session persistence

- [ ] **Save and restore session layouts**
  - Users should be able to recover windows, panes, and working directories
    after a tmux restart.
  - Keep the first version layout-only; do not replay arbitrary commands.

- [ ] **Handle disconnects more gracefully**
  - Client drops and network blips should not leave zmux state confusing or
    stale.
  - Reuse existing workspace/session reconciliation before adding new state.

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

### Pane direct-manipulation

- [ ] **Header drag-swap**
  - Dragging one pane _header_ onto another should swap their positions. The
    right-click per-pane menu (promote/hide/kill) already shipped; this is the
    remaining direct-manipulation gesture.
  - Blocked by tmux 3.4: pane headers are reachable only through generic
    `Pane`/`Border` mouse locations, so a header drag can't be told apart from
    the native resize-border drag without breaking resize. Revisit if a later
    tmux exposes a header-specific mouse zone, or gate it behind a coordinate
    heuristic proven safe in `zzmux`.

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

### Distribution & packaging

- [ ] **Distribution packages**
  - GitHub releases, Homebrew, AUR, and Nix should wait until the local install
    and upgrade story is stable.
