# zmux-guard corpus

Single source of truth for **command classification** shared across agent guards:

- Go `zmux guard` / `internal/guard` (`guard_test.go`)
- Claude PreToolUse hook (`skills/zmux/hooks/zmux-guard.mjs` test)
- pi-extension classifier (`pi-extension/src/classify.ts` test)

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
| pi-extension | `kind` only | decision is kind-derived; diverges on purpose (below) |

Before classifying, all three strip **quoted spans** (so `echo "tmux …"` is safe) and
**leading env assignments** (`NODE_ENV=prod npm run dev` / `env FOO=bar …` classify on
the real verb, not the assignment).

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

## Known gaps (segment model)

All three classifiers parse **command position** per simple-command segment (split on
`;`, `&`, `|`, newline). That deliberately ignores a few ways a raw `tmux` could still
reach a shell — accepted as low-risk because agents almost never use them, and the cost
of a false *positive* (blocking legit work) outweighs catching these:

- **Command substitution** — `echo $(tmux capture-pane -p)`: `tmux` isn't at segment
  command-position, so it's `safe`. (Corpus row marks this contract explicitly.)
- **Indirection** — `sh -c "tmux capture-pane"`, `xargs tmux …`: the wrapper is the
  command; the inner `tmux` is an argument, so it passes.
- **Here-doc bodies are stripped, not scanned** — `cat <<EOF … tmux … EOF` blanks the
  body before classifying. This is *correct*: a here-doc body is stdin data, never
  executed, so a `tmux`/`&` inside it is not an invocation (and not backgrounding).

A bypass token (`ZMUX_ALLOW=1` / `# zmux: allow`) covers the inverse — a legit raw tmux
the classifier *does* flag.

## Decision choices baked in

- `tmux` / `runtime` / `background` → **block** (clean zmux alternative; user chose full coverage).
- `interactive` (ssh/sudo/REPL) → **warn**, not block. Hard-blocking every `ssh`/`sudo`
  would brick legitimate work; warn nudges toward a shared zmux tab while letting it run.
  Flip to `block` per-category if desired. `ZMUX_ALLOW=1` / `# zmux: allow` bypasses any row.
