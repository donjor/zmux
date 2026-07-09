# zmux-guard corpus

Single source of truth for **command classification** shared across agent guards:

- Go `zmux guard` / `internal/guard` (`guard_test.go`)
- Claude PreToolUse hook (`skills/zmux/hooks/zmux-guard.mjs` test)
- pi-zmux classifier (`pi-zmux/src/classify.ts` test)

`kind` is the **shared shell-surface category** of a command — invariant across the
categories all three agents agree on (`tmux`, `runtime`, `background`, `interactive`,
`safe`), and the part every consumer asserts. pi then layers **documented adapters** on
top (a `direct_zmux` nudge for shelled `zmux …`; folding socket-scoped tmux into `safe`)
— see "pi's deliberate divergences". `decision` is the **Claude/shell-surface policy**;
the Go classifier and Claude hook assert it, pi does **not** (its richer typed-tool
surface gives it a different policy — see below). The corpus catches **drift** between
the implementations. It does **not** prevent drift (an unlisted pattern added to one impl
won't be caught until added here) — keep all three tests pointed at it in CI.

This is a **build/test artifact, not a runtime asset** — never link it from `SKILL.md`.

## What each consumer asserts

| consumer | asserts | notes |
|---|---|---|
| Go `internal/guard` | `kind` + `decision` | canonical reference impl; also the `zmux guard` CLI |
| Claude hook (`zmux-guard.mjs`) | `kind` + `decision` | mirrors the Go classifier line-for-line |
| pi-zmux | `kind` only | decision is kind-derived; diverges on purpose (below) |

Before classifying, all three strip **quoted spans** (so `echo "tmux …"` is safe) and
**leading env assignments** (`NODE_ENV=prod npm run dev` / `env FOO=bar …` classify on
the real verb, not the assignment). They also run a **recursive executable-payload pass**
first (below) so a raw tmux or background job hidden one indirection deep is still caught.

## Row schema (JSONL)

| field | values | meaning |
|---|---|---|
| `command` | string | the shell command as the agent would run it |
| `cwd` | `project` \| `repo` | `repo` = inside the zmux repo (raw tmux exempt); `project` = any other cwd (enforced) |
| `kind` | `safe` \| `direct_tmux` \| `runtime` \| `background` \| `interactive` | classification |
| `decision` | `allow` \| `warn` \| `block` | enforce-mode outcome. `block` → hook exit 2; `warn` → visible nudge, non-blocking; `allow` → pass |
| `target` | semantic key or `""` | suggestion key each agent renders to its own surface (Claude: `zmux watch`; pi: typed tool `zmux_runtime_logs`) |
| `note` | string (optional) | why the row exists |

## Cross-agent scope

The corpus covers the categories **both** agents agree on (`tmux`, `runtime`,
`background`, `interactive`, `safe`). pi's extra `direct_zmux` category — nudging
shelled `zmux tabs/watch/run` toward pi *typed tools* — is **pi-specific** (Claude/Codex
have no typed tools and SHOULD shell `zmux …`). So `zmux watch …` is `safe` here; pi's
`direct_zmux` handling is tested pi-side only.

**pi's deliberate divergences** (it asserts `kind`, not `decision`, and skips two row
classes in its corpus test):

- **`zmux …` rows** (kind `safe`): correct CLI usage on a shell surface, but pi nudges
  them to a typed tool → `direct_zmux`. Excluded from pi's parity check; tested pi-side.
- **socket-scoped tmux** (`tmux -L …`, kind `direct_tmux` + `allow`): pi folds the
  exemption into `safe` because its block decision is purely kind-derived (no separate
  `decision` field). The Go side keeps `direct_tmux` + `allow`. Excluded from pi's check.
- **`interactive`** (kind matches): Claude **warns**, pi **blocks** (redirects to
  `zmux_interactive_type`). Same `kind`, different `decision` — fine, pi asserts only `kind`.
- **repo-cwd exemption**: the Go classifier/CLI exempt raw tmux inside the zmux repo via
  cwd; pi's `classifyBash` can't see cwd, so it still reports `direct_tmux` (kind unchanged).

## Recursive executable-payload pass

All three classifiers parse **command position** per simple-command segment (split on
`;`, `&`, `|`, newline). On its own that misses a raw `tmux`/background job that a segment
*executes indirectly*. Before the segment scan, each classifier extracts and recursively
classifies the inner commands a segment would itself run — a Block from any of them is the
verdict (depth-bounded so a pathological nest can't loop):

- **Shell `-c` payloads** — `sh -c 'tmux capture-pane -p'`, `bash -lc '… &'`: the quoted
  `-c` arg is pulled out *before* quote-blanking and classified. Matched only at command
  position, so `echo "sh -c 'tmux'"` (quoted) and `sudo sh -c …` (argument) stay safe. An
  `env ` wrapper and a path prefix are tolerated, so `env sh -c …` and `/bin/sh -c …` are
  caught too. The command is **trimmed at entry** (all three classifiers) so a leading-space
  `   sh -c …` is still at command position, and the `-c` scan runs on **here-doc-stripped**
  text so a `sh -c …` inside an inert `cat <<'EOF'` body is not falsely extracted.
- **`xargs tmux …`** — the command `xargs` execs is checked; if it's `tmux`, the rest is
  classified as a raw tmux call. `xargs grep tmux` (tmux as a pattern) stays safe.
- **Shell-fed here-doc bodies** — `bash <<EOF … tmux … EOF`: the body is *executed* by the
  shell receiver, so it is scanned. The receiver is normalized (path basename'd, leading
  `env` dropped) so `/bin/bash <<EOF` / `env bash <<EOF` count. A **file-writer** receiver
  (`cat > f <<EOF`, `tee`) makes the body inert data — still blanked, never scanned.

## Known gaps (segment model)

Accepted as low-risk — agents almost never use these, and the cost of a false *positive*
(blocking legit work) outweighs catching them:

- **Command substitution** — `echo $(tmux capture-pane -p)`: `tmux` isn't at segment
  command-position and isn't an executable payload, so it's `safe`. (Corpus row marks this
  contract explicitly.) A full shell parser is a deliberate non-goal.
- **Deeper / unanchored indirection** — `time bash -c …`, `nice`/`command`/`exec` wrapper
  chains, `find … -exec tmux …`, `xargs sh -c 'tmux …'` (xargs→shell→tmux, two hops), a
  shell `-c` nested past the recursion depth: the payload pass anchors at command position
  (after an `env`/path prefix only) and is depth-bounded, so these pass. (`nohup bash <<EOF`
  is still caught — as `background`, via the nohup word.)
- **`xargs` long value-flags** — `xargs --max-args 1 tmux …`: `xargsCommand` models only the
  short value-taking flags (`-n`, `-I`, …), so a `--long N` form leaves `N` read as the
  command word and the `tmux` after it escapes. Obscure, fail-open; mirrors all three impls.

### Known false positives (fail-safe)

The inverse of a gap — a **legit** command the classifier over-blocks. Fail-safe (it errs
toward blocking, never toward leaking a raw tmux), and the bypass token clears it, so these
are accepted rather than chased into a shell parser:

- **Separator inside quotes** — `echo 'ok; sh -c "tmux …"'`: the `;` inside the quoted string
  is read as a segment boundary, so the embedded `sh -c …` is matched at a false command
  position and blocked. A clean fix needs quote-aware segmentation (a deliberate non-goal).
  Pinned in the corpus so all three classifiers stay in lockstep on it.

A bypass token (`ZMUX_ALLOW=1` / `# zmux: allow`) covers any over-block — a legit raw tmux,
or a false positive above — that the classifier flags.

## Decision choices baked in

- `tmux` / `runtime` / `background` → **block** (clean zmux alternative; user chose full coverage).
- `interactive` (ssh/sudo/REPL) → **warn**, not block. Hard-blocking every `ssh`/`sudo`
  would brick legitimate work; warn nudges toward a shared zmux tab while letting it run.
  Flip to `block` per-category if desired. `ZMUX_ALLOW=1` / `# zmux: allow` bypasses any row.
