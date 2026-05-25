# Reafactor — Rundown / Light Log

A high-level, chronological record of the restructure effort: what was done, when,
and what user questions/comments steered it. Light by design — for detail see the
per-phase docs, `PLAN.md` (`.dump/plans/016_…`), and git history.

Anchor: omega refactor **merged to `master` at `26dc7a9`, 2026-05-24 19:54**.

---

## Phase 0 — Planning (2026-05-24)

- Goal: restructure zmux to the "true ideal" tree (`dir-tree-ideal-with-context.md`).
- Plan written as `016` under buddy `planned` workflow; reviewed, converged.
- **Scope locked (user gate):** *full ideal, phased*; **S4 = repeal the global
  `app`** → explicit DI. S7/S8/S9 = deliberate cuts. Q1 (keys docgen), Q2 (setup
  plan/apply), Q4 (theme dup) resolved.

## Phase A — Structural, behavior-preserving (2026-05-24)

- S5 split `terminalmeta→termtitle` + `terminaltarget→terminal`.
- S6 split overmind control out of `source` (`overmind.Client`).
- S3 dissolved the flat `tui` package → `styles`, `workspaceview`, `picker`,
  `tabpicker`, `themepicker`, `wizard`.
- S10 de-symlinked `legacy/v0` assets + fixed embed-root docs.
- Green + buddy-reviewed. **User checkpointed here** → Phase C in a fresh session.

## Phase B — DI repeal / S4 (2026-05-24)

- `App` → `internal/app`; `NewRootCmd(app)` composition root; command flag/state
  de-globalized; fixed a pre-existing `--picker` bug.
- Green. Inverted the old "use the global `app`" convention in CLAUDE.md/docs.

## Phase C — Features + seams (2026-05-24, this session's main work)

- **C1** `internal/keys` registry — single source of truth for keybindings;
  `conf.go` now references it (byte-identical output); `zmux help` + dashboard
  help render from it; `zmux keys gen` generates `docs/keybindings.md` with a
  golden test.
- **C2** `internal/setup` — shell-rc integration ported bash→Go (plan/apply,
  `# zmux-managed` markers, `.bak`, dry-run, removal); `zmux setup shell`;
  `install.sh` slimmed. *(Scoped to the real bash→Go gap; the wizard already
  owned config/dirs/tmux.conf.)*
- **C3** `bar.Prober` seam for git/lang. *(Source-discovery prober deferred.)*
- Docs consolidated (`architecture.md`, `CLAUDE.md`).

## Review & merge (2026-05-24)

- **`/ultrareview`** (free 1/3) → only **2 documentation nits** (stale
  CONTRIBUTING.md keybinding line; dashboard Help tab dropped `prefix+d`). Both
  fixed.
- **Squashed** 23 wip commits → **3 clean, green, bisectable phase commits**
  (A / B / C). Backup branch kept then deleted post-merge.
- **`--no-ff` merge to `master`** (`26dc7a9`). Build + test + vet green.
- ⚠ Installed binary (`~/.local/bin/zmux`) still the old build — `./dev.sh`
  intentionally avoided inside the live zmux session.

---

## Post-merge discussion & doc-first follow-ups (2026-05-24)

Driven by the user's questions about structure ("follow the experts"):

| User asked / commented | Outcome |
|------------------------|---------|
| "Don't love `internal/app/app.go` being the only file in its dir." | Clarified: file-count is a non-issue in Go; the real question is package boundary. Found only `cmd/zmux` imports `app`. |
| "`cmd/zmux` vs `internal/` — what's the idea? And so many files in `cmd/zmux` is odd." | Explained enforced rules (`internal/` privacy, `package main`) vs convention (`cmd/` + thin-main). Identified the fix: extract commands → `internal/cli` (gh/hugo/cobra-cli pattern). |
| "Want to follow the experts; record current state + a follow-up before changing anything." | Adopted **doc-first**. |
| Correction: "Leave `dir-tree-current.md` as it was — don't alter the history it represents." | Restored it byte-for-byte and `git mv` → **`dir-tree-pre-refactor.md`**. New live snapshot written as **`dir-tree-post-omega.md`**. |
| — | Wrote **`followup-01-cli-extraction.md`** (thin `main` + `internal/cli`, plan only). |
| "Do we use linting/formatting?" | Audited: `gofmt` is CI-enforced (clean); linting thin (`staticcheck` only local-if-installed; no `golangci-lint`/`gofumpt`). |
| "Yes" (level it up) | Wrote **`followup-02-tooling.md`** (`golangci-lint` in CI + `gofumpt` decision, plan only). |

---

## Reafactor close-out — followups 02 + 01 (plan 017, 2026-05-24)

Run via buddy `planned` (edge). Scope locked with buddy: execute the two written
plans only; 03/04 stay TBD. Order 02 → 01 (tooling first so the CLI move is linted).

**followup-02 (tooling)** — merged `f8cd74a`:
- gofumpt adopted (isolated format-only commit, 15 files).
- `.golangci.yml` v2: defaults + misspell + unconvert + gofumpt formatter;
  `max-same-issues:0` (defaults were *hiding* findings — whack-a-mole).
- 17 findings triaged: 15 fixed, 2 documented `//nolint` (S1016 intentional
  field-copy; QF1001 readability). Discovered golangci-lint v2 `run` enforces
  gofumpt formatting → no separate `fmt --diff` CI step needed.
- CI split into **lint** (golangci-lint-action@v9, pinned v2.12.2) + **test**
  jobs; kept explicit `go vet`. `make lint` / `make fmt`.

**followup-01 (cli extraction)** — merged to master (this merge):
- 33 prod + 13 test files `cmd/zmux` → `internal/cli` (`package cli`).
- `cmd/zmux/main.go` thinned to a launcher: `os.Exit(cli.Run(app.New(), version))`.
- `cli.Run(a, version) int` added; `version` threaded through `NewRootCmd` →
  `newInitCmd` → `runInitWizard`, and `newVersionCmd`. `-ldflags -X main.version`
  verified unchanged.
- `cmd/zmux/app.go` alias deleted; tests use `apppkg.App` + `const testVersion`.
- Snapshot: `dir-tree-post-cli.md`. Docs updated (CLAUDE.md, architecture.md,
  CONTRIBUTING.md).

**Incident:** a personal scratch file (`docs/NOTES.MD`) was accidentally swept
into the first tooling commit by `git add -A`; the branch was rewritten to keep
it out of history (file preserved untracked). Lesson: stage explicit paths.

## Reafactor close-out — followups 03 + 04 + arch refresh (2026-05-25)

Run via buddy `planned` (edge). Scope locked with user: write the two remaining
plan docs **and** execute both seams + the `architecture.md` refresh — full
close-out of the roadmap's "Architecture refactor" section. Branch
`refactor/followup-03-04-purity-seams` off master.

**Buddy catch (intent → plan-review):** Claude's first grep scoped overmind
callers to `internal/cli/` and found 2; buddy flagged **7 live callers** across
`cli` + `palette` + `dashboard/tabs` (and that `Restart`/`Stop` are live, not
just `Connect`). Deleting the wrappers after rewiring only the 2 cli sites would
have broken the build. Both plan docs were corrected before any code. Buddy also
flipped followup-04's signature: an exported `Discover(p Prober)` isn't real
public injection (the interface uses unexported `socketInfo`/`processEntry`), so
the seam is unexported — `Discover()` → `discoverWith(systemProber{})`.

**followup-03 (B-purity seams)** — 3 focused commits:
- `App.Overmind overmind.Client` (default `CLI{}`); injected through
  `palette.Executor` + dashboard `SessionsTab`; all 7 sites rewired; 5 package
  wrappers deleted. Tests use a `noopOvermind`.
- terminal `wm.Adapter` + `procfs.Inspector` injected as `newTerminalCurrentCmd`
  params (production defaults in `newTerminalCmd`); the `newTerminalAdapter`/
  `newTerminalProcess` package-global func vars and the global-swap test helpers
  are gone.
- cli test apps (`cmd_test.go`, `shared_test.go`) use the shared in-memory
  `memFS` instead of `RealFS` — hermetic, no real-disk access.

**followup-04 (source prober)** — 1 commit:
- `internal/source/probe.go`: unexported `prober` + `systemProber`. `Discover()`
  wraps `discoverWith(systemProber{})`; orchestration unchanged. New
  `probe_test.go` with `fakeProber` covers local+overmind happy path, ps-failure
  fallback, stale-socket skip, socket-scan-error local-only.

**Docs:** `architecture.md` size table refreshed (no prod file >500 now), seams
table gained `source.prober` + injection notes; ROADMAP section closed out.

Green throughout: `go build/test/vet ./...` + `make lint` (gofumpt + golangci-lint
v2) at each step. Behavior-preserving; no CLI-surface or generated-output change.

## Current state

- **omega refactor + followups 01/02/03/04 + arch refresh: DONE.** The entire
  roadmap "Architecture refactor" section is closed. All B-purity seams are
  injected; source discovery is testable; no prod file exceeds 500 lines.
- **Next roadmap runs (user-named, not started):** (1) Charm v2 stack upgrade
  (bubbletea v2 + lipgloss v2) + adopt `charmbracelet/log`; (2) `zzmux` full
  socket/config isolation; (3) world-class SSH/nested-zmux handling.

## For the dir-structure skill — catch / avoid / improve

This effort is the worked example behind a future `dir-structure` skill
(`~/donjor/skills/IDEAS/dir-structure/`). The point of keeping this log isn't the
zmux tree — it's the **flow and its failure modes**. What the skill must make
structural so a human doesn't have to catch it by hand:

### 1. The ideal got compromised — and only the human caught it
`dir-tree-ideal-with-context.md` was first drafted with **status-quo bias**: it
rationalized keeping the current flat structure using *migration-cost* arguments
("impossible", "low-ROI", "churn", "it's bash today") dressed up as design law.
Neither the agent nor buddy flagged it — they agreed with each other. It took the
human's challenge — *"is this ideal constrained by current state? it should be a
TRUE ideal, not limited by effort"* — to reframe.
- **Catch** with a structural self-gate, not model vigilance: for every "keep
  current / reject" call, label the reason **DESIGN** (survives "assume infinite
  effort") or **MIGRATION** (cost/convention/churn). Any MIGRATION reason is
  inadmissible in a true-ideal doc — move it to a separate pragmatic-target doc.
- **Avoid** treating buddy agreement as validation. Buddy sharpens a
  *correctly-framed* question; it does **not** reliably catch a *mis-framed* one.
- **Improve** with a banned-words tripwire (`impossible`, `non-negotiable`,
  `low-ROI`, `churn`, `it's bash today`, `the convention says`) + a literal
  re-derive prompt before finalizing.

### 2. The follow-ups are the gap between "done" and the intended bar
followups aren't "extra polish" — but be precise about what they evidence:
- **01 (cli extraction)** wasn't even in `dir-tree-ideal-with-context.md` — that
  doc still placed commands under `cmd/zmux`. It surfaced *after* omega, from a
  "follow the experts" discussion. So the **ideal doc itself was incomplete** and
  kept evolving as expert convention got applied.
- **03 / 04 (injected seams, source prober)** map directly to the ideal's
  side-effect-seam language, yet omega shipped, closed the roadmap section, and
  left them as deferred TODOs. So **execution also stopped short of the documented
  ideal** and called it done.
- **02 (tooling)** is the exception — engineering/process hardening found during
  close-out, not an ideal-structure gap. Don't lump it in.

Net: two distinct gaps — the **spec under-reached** (01) and the **execution
under-delivered** (03/04) — and both hid under "merged + green." It took four
more passes to actually meet the bar.
- **Catch:** the ideal needs **acceptance criteria**, not just a pretty tree.
  After each execution pass, emit a residual-gap table — `ideal item | landed? |
  deferred? | why | followup doc` — mapped to specific ideal-tree lines.
- **Avoid:** "merged + green" ≠ "ideal met." A clean close-out is not evidence
  the bar was reached.
- **Improve:** make the residual gap first-class output, not a celebratory
  close-out that buries it.

### 3. Process & mechanical hygiene that bit us
- **`git add -A` swept a scratch file into history** → stage explicit paths.
- **Caller undercount:** an overmind grep scoped to one package found 2 of 7 real
  callers; deleting shared surface on that basis would have broken the build →
  grep the **whole tree** before removing any shared symbol.
- **Historical artifacts are immutable.** When the layout moved on, the original
  `dir-tree-current.md` was *renamed* (`git mv` → `dir-tree-pre-refactor.md`) and
  new snapshots added — never rewritten in place. A snapshot edited to current
  truth is no longer a snapshot; preserve provenance.

### TL;DR for the skill
The hard part isn't drawing a nicer tree — it's (a) not letting the ideal quietly
shrink to the status quo, and (b) not letting execution quietly stop short of the
ideal and call it done. Both failures here were caught only by the human; the
skill exists to make them structural.

## To-do for the human

- Reinstall the binary from a fresh terminal: `make build && ./dev.sh`
  (the live `~/.local/bin/zmux` is still the pre-refactor build).
- `docs/NOTES.MD` is untracked roadmap scratch — `.gitignore` it or fold into a
  tracked roadmap doc when convenient.
