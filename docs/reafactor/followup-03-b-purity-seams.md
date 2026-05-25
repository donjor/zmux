# Follow-up 03 — B-purity seams (overmind client, terminal injection, cmd-test FS)

> **Status: DONE** (2026-05-25). Behavior-preserving purity/testability work,
> deferred out of `followup-01` (see `followup-01-cli-extraction.md:60-65`).
> Executed in 3 focused commits: `App.Overmind` injection (7 sites rewired, 5
> wrappers deleted), terminal adapter/process injected as cmd params (globals
> gone), cli test apps on in-memory FS. Green (build/test/vet/lint). No
> CLI-surface or generated-output change. Baseline: `dir-tree-post-cli.md`.
>
> _Original plan (intent + mechanics) preserved below._

## Why

The omega refactor (Phase B) made `App` the explicit composition root and pushed
side-effects behind interfaces (`tmux.Runner`, `config.FS`, `wm.Adapter`,
`procfs.Inspector`, `bar.Prober`). Three seams were knowingly left for a focused
follow-up — they're the remaining spots where the command layer reaches around
DI instead of through it:

1. **overmind package-level wrappers** — `internal/overmind/overmind.go:73-82`
   exposes `Connect`/`Restart`/`Stop`/`StopAll`/`Logs` free functions that
   hardcode `CLI{}`. The `overmind.Client` interface + `CLI` impl already exist
   (`:18-29`) but **nothing wires them onto `App`**. There are **7 live wrapper
   call sites across two layers** — so overmind control can't be faked in any
   test:
   - `internal/cli/popup_modes.go:247` — `Connect`
   - `internal/cli/session_picker.go:159` — `Connect`
   - `internal/tui/palette/executor.go:101,107,113` — `Connect`/`Restart`/`Stop`
   - `internal/tui/dashboard/tabs/sessions_actions.go:175,216` — `Restart`/`Stop`

   `Connect`, `Restart`, and `Stop` all have live callers; only `StopAll`/`Logs`
   are currently unused.

2. **terminal test-injection package globals** — `internal/cli/terminal.go:62-65`:
   ```go
   var (
       newTerminalAdapter = func() wm.Adapter { return wm.NewHyprlandAdapter() }
       newTerminalProcess = func() procfs.Inspector { return procfs.LinuxInspector{} }
   )
   ```
   Tests swap these globals (`terminal_test.go:66-77` `withTerminalAdapter` /
   `withTerminalProcess`). Mutable package state as a test seam is the exact
   anti-pattern DI was meant to retire. Note: the *resolver* package
   (`internal/terminal/current.go`) is already clean — it takes `Adapter` +
   `Process` as struct fields. This item is only the **cli command** that builds
   the resolver.

3. **cmd tests on `RealFS`** — `internal/cli/cmd_test.go:21-24` and
   `internal/cli/shared_test.go:20-23` build their `App` with `&config.RealFS{}`,
   so those tests touch the real home dir / disk. A `memFS` test double already
   exists (`internal/cli/shorthand_test.go:19` and `internal/config/load_test.go:12`)
   — these sites just never adopted it.

## Goal

- `App` gains an `Overmind overmind.Client` field, defaulted to `CLI{}` in
  `New()`. All 7 call sites route through an injected client (cli sites via
  `app.Overmind`; the two TUI consumers — `palette.Executor` and the dashboard
  `SessionsTab` — grow an injected `overmind.Client`). The five package wrapper
  funcs are **deleted** (interface + `CLI` stay).
- `terminal` cmd takes its `wm.Adapter` + `procfs.Inspector` through injection
  (constructor params / `App`), not package-global func vars. Tests inject a
  fake the same way every other cli test does — no global mutation.
- `cmd_test.go` + `shared_test.go` build `App` with the shared `memFS`, matching
  `shorthand_test.go`. No cli test touches real disk.

## Scope

**In scope:**
- Add `App.Overmind`; default in `app.New()`; thread an injected client through
  `palette.Executor` + dashboard `SessionsTab`; rewire all 7 call sites; delete
  the 5 package wrappers in `overmind.go`.
- Replace the 2 terminal package globals with injected deps; update
  `terminal_test.go` to inject instead of mutate globals.
- Migrate the 2 `RealFS` cli-test sites to `memFS`.

**Out of scope:**
- Source-discovery prober → `followup-04-source-prober.md`.
- `docs/architecture.md` refresh (tracked separately on the roadmap).
- Any behavior, flag, or output change.

## Mechanics

### 1. overmind.Client on App + the two TUI consumers

The seam spans two layers; the injected client has to reach the TUI consumers,
not just the cli flows. Order: wire the field → thread into consumers → rewire
all 7 sites → only then delete the wrappers (build must stay green at each step).

- `internal/app/app.go`: add field `Overmind overmind.Client`; in `New()` set
  `Overmind: overmind.CLI{}`. (Import `internal/overmind`.)
- **cli sites (hold `app` already):**
  - `popup_modes.go:247`: `overmind.Connect(...)` → `app.Overmind.Connect(...)`.
  - `session_picker.go:159`: `overmind.Connect(...)` → `app.Overmind.Connect(...)`.
    (Confirm `app` in scope; thread through if not.)
- **`palette.Executor`** (`internal/tui/palette/executor.go`): add field
  `Overmind overmind.Client`; `NewExecutor(runner, fs, overmind)` (`:37-39`);
  `:101/:107/:113` use `e.Overmind`. Caller `popup_modes.go:149` passes
  `app.Overmind`. Test `executor_test.go:22` passes a fake/noop.
- **dashboard `SessionsTab`** (`internal/tui/dashboard/tabs/sessions.go:95`):
  add an `overmind.Client` param to `NewSessionsTab`; `sessions_actions.go:175/216`
  use it. Caller `popup_modes.go:104` passes `app.Overmind`. Test
  `sessions_test.go:118` passes a fake/noop.
- `internal/overmind/overmind.go`: **after all 7 sites are rewired**, delete the
  `Connect/Restart/Stop/StopAll/Logs` free functions (`:73-82`). Keep `Client`,
  `CLI`, and the `var _ Client = CLI{}` assertion.
- Grep guard: `grep -rn 'overmind\.\(Connect\|Restart\|Stop\|StopAll\|Logs\)'`
  across the whole tree must return zero non-method hits afterward.

> **Test double:** a `noopClient` (all methods return `nil`/empty) in a shared
> test helper covers the TUI tests that don't assert on overmind; a recording
> fake covers any test that does.

### 2. terminal adapter/process injection

Decide the injection vehicle (see Open decisions). Recommended: **constructor
params with production defaults**, no `App` change:

```go
// newTerminalCmd(app, adapter, process) — or a small opts struct.
// Production wiring in NewRootCmd passes wm.NewHyprlandAdapter / procfs.LinuxInspector.
// terminal_test.go constructs the cmd with fakes directly.
```

- Remove the `var ( newTerminalAdapter… newTerminalProcess… )` block.
- Replace `terminal_test.go`'s `withTerminalAdapter`/`withTerminalProcess`
  global-swap helpers with direct injection into the command under test.

### 3. cmd-test memFS

- Point `cmd_test.go:21-24` and `shared_test.go:20-23` at the existing `memFS`
  (the `shorthand_test.go` constructor) instead of `&config.RealFS{}`.
- Seed any config files the asserting tests expect into the memFS.
- If the memFS helper isn't shared across `_test.go` files in `package cli`,
  promote it to one shared test helper (it's all one package).

## Tests

- overmind: a cli test can now inject a fake `overmind.Client` via `App.Overmind`
  and assert connect is invoked with the right socket/process (new capability).
- terminal: same coverage as today, minus the global mutation.
- FS: cmd/shared tests run hermetically against memFS.
- Existing golden/green tests (`conf_test`, `TestKeybindingsDocInSync`) untouched.

## Verification

- Per-step: `go build ./... && go test ./... && go vet ./...` + `make lint`.
- Behavior-preserving: no generated output or CLI-surface change.
- Commit granularly (one commit per seam) so each is independently revertable.

## Risks

| Risk | Mitigation |
|------|------------|
| Deleting wrappers before all 7 sites are rewired breaks the build (`palette`, `dashboard/tabs`) | Strict order: field → thread into Executor + SessionsTab → rewire all 7 → *then* delete. `go build ./...` between steps. Grep guard before + after. |
| `session_picker.go` call site doesn't hold `app` | Thread `app` through; it's already the cli DI vehicle. Verify before deleting wrappers. |
| Threading client into `NewSessionsTab`/`NewExecutor` ripples to their tests | Two test sites (`executor_test.go:22`, `sessions_test.go:118`) pass a noop client. Mechanical. |
| terminal injection reshapes the cmd constructor signature | Keep production defaults; only `NewRootCmd` + the test change. Mechanical. |
| memFS missing files a test reads | Seed expected config into memFS; run the specific test to confirm. |

## Open decisions

- **terminal injection vehicle:** (a) constructor params (recommended — local,
  no `App` change, matches how the resolver already takes deps); (b) fields on
  `App` (consistent with `Overmind`, but `App` would carry desktop-WM deps only
  one command uses). Lean (a).
- **overmind on App vs constructor param:** `App.Overmind` (recommended —
  overmind connect is invoked from two unrelated cli flows; a shared injected
  client is cleaner than threading a param into both).

## Related

- `followup-01-cli-extraction.md` — deferred these items (`:60-65`).
- `followup-04-source-prober.md` — the remaining C3 discovery seam.
- `bar.Prober` (`internal/bar/probe.go`) — the in-repo precedent for an injected
  syscall seam (interface + Exec impl + fake).
