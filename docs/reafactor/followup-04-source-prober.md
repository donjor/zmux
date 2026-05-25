# Follow-up 04 — Source-discovery prober (C3 seam in `internal/source`)

> **Status: DONE** (2026-05-25). Behavior-preserving testability work. The omega
> Phase C added a `bar.Prober` seam for git/lang but **deferred the analogous
> source seam** (`RUNDOWN-LIGHT-LOG.md:44` — "Source-discovery prober deferred").
> This finishes C3 by mirroring that pattern in `internal/source`: unexported
> `prober`/`systemProber`, `Discover()` → `discoverWith(systemProber{})`, +
> `fakeProber` orchestration tests. Green. Baseline: `dir-tree-post-cli.md`.
>
> _Original plan (intent + mechanics) preserved below._

## Why

`source.Discover()` (`internal/source/discover.go:41`) is a free function that
does all its I/O inline:

- `findTmuxSockets()` (`:122-149`) — `os.Getenv` + `os.ReadDir` of the tmux
  socket dir.
- `buildProcessTable()` (`:152-158`) — `exec.Command("ps", ...)`.
- `probeSocket()` (`:294-311`) — `exec.CommandContext(ctx, "tmux", ...)` live
  list-sessions.
- local listing constructs `tmux.NewClient()` directly (`:45`).

The **parsing/correlation logic is already pure and well-tested**
(`parseProcessTable`, `correlateSources`, `findOvermindProcesses`,
`isOvermindStart`, `deriveSocketName`, `overmindLabel`, `parseProbeOutput` — all
covered in `discover_test.go`). What's untestable is the **orchestration**: the
`Discover()` glue that decides local-first, ps-failure fallback, stale-socket
skipping. Today that path can only be exercised against the real machine.

A `Prober` seam makes `Discover` orchestration deterministically testable
(simulate "ps failed → generic external", "socket probe times out → skip",
"overmind correlated") without touching the host.

## Goal

```go
// internal/source/probe.go
type prober interface {
    listSockets() ([]socketInfo, error)              // fs scan of tmux socket dir
    processTable() ([]processEntry, error)            // ps -eo pid,ppid,args
    probeSocket(ep tmux.Endpoint) ([]CatalogEntry, SourceHealth, error)
    localSessions() ([]CatalogEntry, bool)            // local server, ok=running
}

type systemProber struct{}   // production: the current inline impls
var _ prober = systemProber{}
```

`Discover` becomes orchestration over an injected `prober`; the pure parsers stay
exactly as they are.

**The seam is unexported by design.** The interface signature uses package-private
types (`socketInfo`, `processEntry`), so it can never be a real *public* extension
point — an outside package couldn't construct those to implement it. So we keep
the prober internal and don't pretend otherwise (this is buddy's correction to
the first draft, which proposed an exported `Discover(p Prober)`).

## Scope

**In scope:**
- New `internal/source/probe.go`: `prober` interface + `systemProber` production
  impl. Move the four I/O bodies (`findTmuxSockets`, `buildProcessTable`,
  `probeSocket`, local-listing) into `systemProber` methods. The pure helpers
  (`parse*`, `correlate*`, `findOvermind*`, `deriveSocketName`, `overmindLabel`)
  stay as package funcs called by orchestration.
- Extract orchestration into unexported `discoverWith(p prober)`; keep the
  exported `Discover()` as a thin wrapper `return discoverWith(systemProber{})`.
  **Zero churn at the 3 callers** (`palette/providers.go:241`,
  `dashboard/tabs/sessions.go:240`, `picker/picker.go:153`) — they keep calling
  `source.Discover()`.
- New orchestration tests in `discover_test.go` calling `discoverWith(fakeProber{})`.

**Out of scope:**
- followup-03 B-purity seams (separate doc).
- Reworking the overmind socket-name derivation heuristic (`deriveSocketName`) —
  behavior preserved verbatim.
- Removing `tmux.NewClient()` usage *elsewhere*; only the source-local path.

## Mechanics

1. Add `probe.go` with `prober` + `systemProber`. `systemProber.listSockets`
   wraps `findTmuxSockets`; `.processTable` wraps `buildProcessTable`;
   `.probeSocket` wraps `probeSocket`; `.localSessions` wraps the local
   `tmux.NewClient().ServerRunning()/ListSessions()` block from `Discover:44-65`.
   (The wrapped funcs can stay unexported in `discover.go` or move into
   `probe.go` — keep the diff minimal.)
2. Extract `discoverWith(p prober) (*Catalog, error)` holding all the
   orchestration; `Discover()` becomes `return discoverWith(systemProber{})`.
   Orchestration logic (local-first, `psErr` fallback to generic external,
   `HealthStale` skip, source-ref attach) is **unchanged** — only the I/O calls
   swap to `p.<method>()` calls.
3. No caller changes — the 3 sites keep `source.Discover()`.
4. Add `fakeProber` to `discover_test.go`; add orchestration tests:
   - happy path: 1 local + 1 overmind external, both probe OK.
   - `ProcessTable` error → sockets become generic `SourceExternal`.
   - `ProbeSocket` returns `HealthStale` → source skipped from `cat.External`.
   - `ListSockets` error → local-only catalog returned.

## Target signature — DECIDED

`Discover()` (exported, unchanged) → `discoverWith(systemProber{})` (unexported,
testable). Rationale: the `prober` interface uses package-private types, so it's
not a viable public extension point — keeping it internal is honest and means
**zero churn at the 3 TUI callers**. Tests in `package source` call
`discoverWith(fakeProber{})` directly.

Rejected: exported `Discover(p Prober)` (first draft) — looks like public
injection but isn't, and forces 3 caller edits for no gain. Rejected:
`Discoverer{}` struct on `App` — the callers don't hold `App`; over-built.

## Tests

- All existing `discover_test.go` pure-parser tests stay green untouched.
- New orchestration tests via `fakeProber` (deterministic, no host I/O).
- `bar/probe_test.go` `fakeProber` is the structural template.

## Verification

- `go build ./... && go test ./... && go vet ./...` + `make lint`.
- Behavior-preserving: real `SystemProber` produces identical discovery to today;
  spot-check `zmux ls` against a live external/overmind socket if available.

## Risks

| Risk | Mitigation |
|------|------------|
| Orchestration subtly changes during the extraction | Move I/O bodies verbatim into methods; diff `Discover` to confirm only call-shape changed. |
| `socketInfo`/`processEntry` are unexported | Fine — `prober` is itself unexported and lives in `package source`; the whole seam is package-internal by design. |
| 3 TUI callers swallow the error today (`_ =`) | Preserve existing error handling at each site; don't change UX. |
| Probe timeout behavior | `SystemProber.ProbeSocket` keeps the `2s` `probeTimeout` context exactly. |

## Related

- `bar.Prober` (`internal/bar/probe.go`, `probe_test.go`) — the precedent.
- `followup-03-b-purity-seams.md` — the sibling B-purity follow-up.
- `RUNDOWN-LIGHT-LOG.md:44` — where this was deferred.
