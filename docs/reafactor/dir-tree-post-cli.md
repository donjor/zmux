# zmux — Directory Structure (post-cli-extraction)

Point-in-time snapshot **after followup-01 (CLI extraction) merged to master**
(plan 017, 2026-05-24). Supersedes `dir-tree-post-omega.md`. Paired with:

- `dir-tree-post-omega.md` — the omega-merge snapshot this supersedes
- `dir-tree-pre-refactor.md` — the original pre-omega snapshot
- `followup-01-cli-extraction.md` / `followup-02-tooling.md` — the executed plans

## What changed vs `dir-tree-post-omega.md`

- **`cmd/zmux` is now a thin launcher** — only `main.go` (~30 lines). It calls
  `cli.Run(app.New(), version)`; `version` stays in `package main` so
  `-ldflags -X main.version` is unchanged.
- **New `internal/cli` package** — the entire command tree (33 prod + 13 test
  files, `package cli`) moved here from `cmd/zmux`. Now importable & externally
  testable (a `package main` can't be imported).
- **`cmd/zmux/app.go` alias deleted** — `type App = apppkg.App` is gone; tests
  reference `apppkg.App` directly. `internal/app` now has a peer (`internal/cli`),
  resolving the "lonely one-file package + alias" smell.
- **Tooling (followup-02, merged just before):** `.golangci.yml` (v2, gofumpt),
  CI lint+test split, `make lint`/`make fmt`.

## Tree (CLI-relevant slice)

```
zmux/
├── .golangci.yml                    # NEW (followup-02): golangci-lint v2 config
├── cmd/
│   ├── uiproto/                     # UI prototyping harness (unchanged)
│   └── zmux/
│       └── main.go                  # ⬅ THIN launcher: os.Exit(cli.Run(app.New(), version))
│
└── internal/
    ├── cli/                         # ⬅ NEW: the command tree (was cmd/zmux/*.go), package cli
    │   ├── root.go                  #   NewRootCmd(a, version) + Run(a, version) int
    │   ├── errors.go                #   formatError + exitCodeForError (used by Run)
    │   ├── popup_modes.go  session_picker.go
    │   ├── init.go (newInitCmd(a, version) → runInitWizard(app, version))
    │   ├── version.go (newVersionCmd(version))
    │   ├── apply.go status.go help.go completion.go refresh.go keys.go setup.go
    │   ├── new.go open.go kill.go ls.go tabs.go tab.go
    │   ├── pane*.go  workspace.go theme.go bar*.go terminal.go run.go watch.go send.go
    │   └── (13 _test.go — package cli; use apppkg.App + const testVersion)
    ├── app/app.go                   # composition root — now a peer of cli (alias gone)
    └── …(all other packages unchanged from dir-tree-post-omega.md)…
```

## Package inventory (delta)

| Area | Before (post-omega) | After (post-cli) |
|------|---------------------|------------------|
| `cmd/zmux` | 35 prod files, `package main` | 1 file (`main.go`), thin launcher |
| `internal/cli` | — | 33 prod + 13 test, `package cli` (importable) |
| `internal/app` | one-file pkg + `cmd/zmux/app.go` alias | one-file pkg, **alias deleted**, has a peer |

## Remaining structural notes (post-cli ≠ final ideal)

The omega "known smells" #1 and #2 (commands-in-`package main`, lonely `app` +
alias) are now **resolved**. Still open / deferred (own future docs):

1. B-purity seams — overmind package-wrappers → injected `Client`; `terminal.go`
   test-injection globals; cmd tests could use a memFS. (`followup-03`, TBD)
2. C3 source-discovery prober. (`followup-04`, TBD)
3. `docs/architecture.md` has some pre-existing post-omega staleness unrelated to
   this move (e.g. `internal/tui/picker_view.go` → now under `internal/tui/picker/`);
   worth a separate architecture-doc refresh pass.

These are cleanliness/convention, not correctness.
