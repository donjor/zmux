# zmux - product

> Current product framing from the implemented CLI and existing vision docs.
> Deeper product strategy remains human-owned.

## What it is

zmux is an opinionated tmux management wrapper for making tmux feel like a
polished workspace tool instead of a raw multiplexer. It provides workspace and
session management, recipes, theming, status bar presets, popup TUIs, logical
tabs, terminal evidence capture, and agent-friendly terminal controls in one Go
binary.

## Who it's for

- **Terminal-heavy developers** who use tmux and want safer defaults, better
  discovery, and less manual tmux configuration.
- **Solo/project-focused workflows** where workspaces, sessions, recipes, and
  status context should be quick to create and inspect.
- **Agent-assisted development** where long-running commands, peer agents,
  interactive prompts, and terminal evidence should stay visible and
  addressable.
- **The maintainer's Hyprland/Ghostty/neovim setup** as the primary proving
  ground, while keeping the core CLI useful on Linux and macOS.

## Core concepts

- **Workspace** - a named project container that groups one or more tmux
  sessions.
- **Session** - a tmux session managed by zmux, with grouped-session clones
  collapsed to their root in user-facing labels.
- **Logical tab** - a pane-canonical tab identity that can appear as a full tab,
  as a pane inside another tab, or hidden in the dock while remaining
  addressable.
- **Recipe** - a TOML launch plan for creating sessions, tabs, and commands.
- **Theme and bar preset** - the visual layer zmux generates into tmux config,
  including semantic colors and dynamic status segments.
- **Agent terminal command** - `run`, `watch`, `send`, and `type` workflows for
  keeping long-running or interactive work in visible named tabs.
- **Edge profile** - the isolated `zzmux` binary/profile used for testing
  changes without mutating the live `zmux` profile.

## What it does NOT do

- It does not replace tmux; it configures and drives tmux.
- It does not aim to be a general terminal emulator or window manager.
- It does not hide long-running processes in background shell jobs.
- It does not provide a full remote/session persistence product yet; those are
  forward roadmap areas.
- It does not make the archived bash prototype the supported path; new behavior
  belongs in the Go implementation.
