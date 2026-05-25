# Follow-up 02 — Lint & format tooling (`golangci-lint` in CI)

> **Status: DONE — executed & merged** (plan 017, 2026-05-24). gofumpt adopted
> (repo reformatted), `.golangci.yml` v2 added (defaults + misspell + unconvert),
> 17 findings triaged (15 fixed, 2 documented `//nolint`), CI split into
> lint+test jobs (golangci-lint-action@v9 pinned v2.12.2), `make lint`/`make fmt`.
> Baseline was `dir-tree-post-omega.md`.
>
> _Original plan preserved below._

## Why

Formatting is solid but **linting is thin**:

| Check | Today | Gap |
|-------|-------|-----|
| `gofmt` | CI hard-enforces `gofmt -l .` empty | ok |
| `go vet` | CI + `make lint` | ok, but shallow |
| `go build` / `go test` | CI | ok |
| `staticcheck` | `make lint` **only if installed locally** | ⚠ never runs in CI → catches nothing on PRs |
| richer static analysis (`errcheck`, `ineffassign`, `unused`, `unconvert`, …) | — | ❌ absent |
| `golangci-lint` | — | ❌ absent (no `.golangci.yml`) |
| `gofumpt` | — | ❌ absent |

So `go vet` is the only static analysis actually gating PRs, and `staticcheck`
is effectively decorative (skipped unless a contributor happens to have it).

## Expert grounding

`golangci-lint` is the de-facto standard linter runner for Go — it bundles and
caches `staticcheck`, `errcheck`, `ineffassign`, `unused`, `govet`, etc. behind
one config + one CI step. Used by kubernetes, gh, hugo, prometheus, and most
serious Go projects. Many also standardize formatting on `gofumpt` (a stricter
`gofmt` superset). That's the upgrade.

## Goal

- One checked-in **`.golangci.yml`** defining the enabled linters.
- **CI runs `golangci-lint`** on every push/PR (replacing the bespoke `gofmt -l`
  step and the "if installed" `staticcheck` line).
- `make lint` runs the *same* thing locally, so local == CI.
- Decide on **`gofumpt`** for formatting (stricter, one-time reformat).

## Scope

**In scope:** `.golangci.yml`, CI workflow edit, `Makefile` `lint` target, and
**triaging/fixing whatever the first real run flags** (or justifying per-linter
disables). Optionally adopt `gofumpt`.

**Out of scope:** the `internal/cli` extraction (followup-01); B-purity seams;
the C3 source prober. Tooling lands independently of those.

## Proposed setup (sketch — exact syntax pinned at execution)

> `golangci-lint` v2 moved formatting into a `formatters:` block and a
> `golangci-lint fmt` subcommand, and its default linter set already includes
> `errcheck`, `govet`, `ineffassign`, `staticcheck`, `unused`. The version will
> be **pinned** in CI for reproducibility; the config below is illustrative.

```yaml
# .golangci.yml  (v2 schema — illustrative)
version: "2"
linters:
  default: standard          # errcheck, govet, ineffassign, staticcheck, unused
  enable:
    - misspell               # doc/comment typos
    - unconvert              # redundant conversions
    - revive                 # (optional) golint successor — start light
  exclude-rules:
    - path: legacy/          # archived v0 prototype — never lint
      linters: [all]
    - path: _test\.go        # relax a few noisy linters in tests if needed
formatters:
  enable:
    - gofmt                  # or: gofumpt (see decision below)
```

CI (replace the current gofmt-only step):

```yaml
- name: golangci-lint
  uses: golangci/golangci-lint-action@v6   # pin to a release
  with:
    version: v2.x.y                          # pin
```

`Makefile`:

```makefile
lint:
	golangci-lint run
	golangci-lint fmt --diff   # fail if unformatted (replaces gofmt -l)
```

## Migration reality (important)

A first `golangci-lint run` on an existing codebase is **rarely clean** — turning
on `errcheck`/`unused`/`staticcheck` typically surfaces real findings (unchecked
errors, dead helpers — e.g. the known dead funcs noted in
`internal/preview/bar/draft/` and `…/pane/`). Plan:

1. Add config with a **conservative** linter set first (defaults + 1–2 extras).
2. Run it; **triage** the findings into: fix now / `//nolint` with reason /
   disable the linter with a comment in `.golangci.yml`.
3. Only then wire it into CI as a hard gate (so CI doesn't land red).
4. Expand the linter set in later passes once the baseline is clean.

## Open decisions

1. **`gofumpt` vs stay on `gofmt`?** gofumpt is stricter and would cause a
   one-time repo-wide reformat (churn, but it's exactly the "follow the experts"
   tightening). Recommendation: adopt `gofumpt` — do the reformat in its own
   isolated commit so the diff is reviewable.
2. **Linter aggressiveness.** Start with the v2 default set + `misspell` +
   `unconvert` (low-noise, high-signal), or go broader (`revive`, `gocritic`,
   `errorlint`, …) and accept more triage. Recommendation: **start
   conservative**, expand later.
3. **Ordering vs followup-01.** Independent; can land before or after the cli
   extraction. Doing tooling *first* means the cli-extraction PR gets linted.

## Verification

- `golangci-lint run` clean (after triage) + `golangci-lint fmt --diff` clean.
- `go build ./... && go test ./... && go vet ./...` still green.
- One isolated commit for any `gofumpt` reformat; linter-fix commits separate
  from the config/CI wiring commit.
