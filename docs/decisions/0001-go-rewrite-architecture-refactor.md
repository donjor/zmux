# Go rewrite architecture refactor

## Context

zmux started as a bash and gum prototype and moved to a Go implementation as the
workspace, dashboard, theme, recipe, tab, and agent-terminal surfaces grew. The
prototype shape no longer gave enough type safety, test seams, or package
boundaries for the amount of tmux orchestration in the project.

## Decision

The current architecture keeps `cmd/zmux` as a thin launcher and places the
command tree under `internal/cli`. Business logic sits under focused
`internal/*` packages, and side effects are routed through typed interfaces such
as `tmux.Runner`, `config.FS`, `qa.CmdRunner`, `bar.Prober`, and the source and
terminal adapter seams.

The old bash implementation remains archived under `legacy/v0/`, but new work
targets the Go codebase.

## Consequences

- The Go compiler and package boundaries now enforce most implementation seams.
- Unit tests can mock tmux, filesystem, process, and command-runner behavior
  without launching real tmux for ordinary package tests.
- `cmd/zmux` stays small; command behavior belongs in `internal/cli`.
- Historical process logs and tree snapshots are archived outside tracked docs;
  this decision record preserves the durable architectural call.
