# Follow-up 01 — Extract the command layer (`cmd/zmux` → `internal/cli`)

> **Status: DONE — executed & merged** (plan 017, 2026-05-24). 33 prod + 13 test
> files moved to `internal/cli`; thin `cmd/zmux/main.go` calls `cli.Run`; `App`
> alias deleted; `version` threaded; `-ldflags -X main.version` unchanged (verified).
> Snapshot: `dir-tree-post-cli.md`. Baseline was `dir-tree-post-omega.md` (merge
> `26dc7a9`). Behavior-preserving, cleanliness/convention only.
>
> _Original plan (intent + mechanics) preserved below._

## Why

`cmd/zmux` currently holds **35 command files in `package main`**. That puts real
logic in a launcher package, which means the command layer **cannot be imported
or tested as a normal package** — `package main` is un-importable by design. It
also leaves `internal/app` as a lonely one-file package plus a `type App =
apppkg.App` alias that exists only so `package main` tests can write `&App{}`.

The established Go convention is the opposite: **`main` is a thin launcher; the
command tree lives in an importable package.**

### What the well-regarded Go CLIs do

| Project | `main` | Command tree |
|---------|--------|--------------|
| GitHub CLI (`cli/cli`) | `cmd/gh/main.go`, thin, returns exit code | `pkg/cmd/...` |
| Hugo | thin `main.go` | `commands/` package |
| cobra-cli generator | thin `main.go` → `cmd.Execute()` | a `cmd/` package |
| kubectl | thin | `pkg/cmd/...` |

Only two things are *enforced* by Go: `internal/` (compiler-private to the
module) and `package main` (= a binary). The thin-main / importable-command-pkg
split is **convention**, but it is the near-universal convention for cobra CLIs.

## Goal

```
cmd/zmux/main.go      ~12 lines: os.Exit(cli.Run(app.New(), version))
internal/cli/         the command tree (was cmd/zmux/*.go), package cli
internal/app/app.go   now a peer of internal/cli; the alias is deleted
```

Outcomes:
- `main` becomes a launcher (convention-aligned).
- Command layer becomes a normal **importable, externally-testable** package.
- `internal/app` gets a neighbour; the `type App = apppkg.App` alias is removed.
- Cleaner mental model: `cmd/` = launchers only; `internal/` = all logic.

## Scope

**In scope (this plan):**
- Move every `cmd/zmux/*.go` **except `main.go`** → `internal/cli/`, changing
  `package main` → `package cli`. That's 34 prod files + 13 test files.
- Rewrite `cmd/zmux/main.go` as a thin launcher.
- Delete the `cmd/zmux/app.go` alias; update the 3 test sites (`&App{}` →
  `&app.App{}` / `apppkg.App`).
- Thread `version` explicitly (see below) — keep `-ldflags -X main.version`
  unchanged.

**Out of scope (tracked separately, not bundled here):**
- B-purity items: overmind package-wrappers → injected `overmind.Client` on
  `App`; `terminal.go` test-injection package globals; cmd tests using a memFS
  instead of `RealFS`.
- C3 source-discovery prober (`ps`/`tmux`/socket-probe seam).
- These deserve their own focused follow-up docs; keeping this one mechanical.

## The move (mechanics)

1. `git mv cmd/zmux/<file>.go internal/cli/<file>.go` for all files except
   `main.go`; change the package clause `main` → `cli`. (Bodies unchanged —
   `apppkg "…/internal/app"` imports already used throughout, so call sites stay.)
2. `NewRootCmd(a *app.App)` → `NewRootCmd(a *app.App, version string)`; thread
   `version` into the two consumers that need it: `newVersionCmd(version)` and
   `newInitCmd(a, version)` (the wizard takes it). No other body changes.
3. Add `cli.Run(a *app.App, version string) int` — builds the root command,
   runs it, formats errors via the existing `formatError`/`exitCodeForError`
   (which move into `cli`), returns an exit code. `os.Exit` stays in `main`.
4. Delete `cmd/zmux/app.go` (the alias). Update the 3 test sites that use
   `&App{}` to `&apppkg.App{}`.
5. Rewrite `cmd/zmux/main.go`:
   ```go
   package main

   import (
       "os"
       "runtime/debug"
       "github.com/donjor/zmux/internal/app"
       "github.com/donjor/zmux/internal/cli"
   )

   var version = "dev" // set via -ldflags -X main.version (unchanged)

   func init() { /* existing build-info fallback, unchanged */ }

   func main() { os.Exit(cli.Run(app.New(), version)) }
   ```
6. `-ldflags -X main.version` stays valid (version still lives in `package
   main`) → **no Makefile / install.sh / dev.sh changes.**

## Target tree (after)

```
cmd/zmux/
  main.go            ← ~12-line launcher
internal/
  cli/               ← root.go, all command files, errors.go, popup_modes.go,
                       session_picker.go, + their _test.go  (package cli)
  app/app.go         ← unchanged; now a peer of cli, alias gone
  …(everything else unchanged)…
```

## Tests

- The 13 `cmd/zmux/*_test.go` move into `internal/cli` as `package cli` tests
  (or `package cli_test` for the externally-facing ones — decide per file).
- They keep working: same package, same helpers. Only the 3 `&App{}` sites change.
- New capability unlocked: `internal/cli` can now have an **external**
  `package cli_test` that exercises `NewRootCmd`/`Run` as a real consumer would.

## Verification / green discipline

- Branch off master; commit in 2–3 reviewable steps (move+repackage → main shim
  + version threading → alias removal + tests).
- Each commit: `go build ./... && go test ./... && go vet ./...` + `gofmt`.
- Behavior-preserving: no generated-output or CLI-surface change. `conf_test`,
  `TestKeybindingsDocInSync`, and cmd tests must stay green.

## Risks

| Risk | Mitigation |
|------|------------|
| Import cycle when commands move | `cli` sits at the top of `internal` (like `cmd` did); imports `app` + domain, nothing imports `cli`. No cycle. |
| `version` wiring breaks ldflags | Keep `version` in `package main`; thread as a param. ldflags target unchanged. |
| Test-helper churn (`&App{}`) | Only 3 sites; mechanical. |
| Big diff (47 files) obscures review | Granular commits; diff is `package` line + path moves, near-zero body change. |

## Open decision

- **Package name:** `internal/cli` (recommended — clear, doesn't collide with
  top-level `cmd/`). Alternatives: `internal/commands` (Hugo), `internal/cmd`
  (cobra-cli; visually clashes with `cmd/`).

## Related follow-ups (separate plans, not this one)

- `followup-02-tooling.md` (written): `golangci-lint` in CI + `gofumpt` decision.
- `followup-03-*` (TBD): B-purity seams — `overmind.Client` on `App`,
  `terminal.go` test injection, cmd-test memFS.
- `followup-04-*` (TBD): C3 source-discovery prober.
