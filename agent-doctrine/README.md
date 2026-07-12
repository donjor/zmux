# Agent doctrine registry

This package-local registry is the source of truth for terminal behavior shared by the Claude `zmux` skill and Pi `pi-zmux` extension.

## Sources

- `rules/ZD-###-*.json` — one neutral invariant with explicit Claude/Pi applicability, mechanism, enforcement, caveats, and verification references.
- `scenarios/ZS-###-*.json` — one host-driven behavioral scenario with a worker-safe prompt and harness-specific host answer keys.

A one-harness record must include `divergenceReason`. Shared scenario prompts contain outcomes only; operation names belong in host-only `answerKey` fields.

## Generate

```sh
node agent-doctrine/generate.mjs --write
node agent-doctrine/generate.mjs --check
```

`--write` validates all records and rewrites only changed committed projections. `--check` never mutates and exits non-zero with the stale output paths. Inputs and outputs must remain package-local committed files; symlinked registry records are rejected.

Generated outputs currently include:

- `skills/zmux/references/shared-doctrine.generated.md`
- `pi-zmux/src/generated/doctrine.ts`
- `pi-zmux/doctrine-manifest.generated.json`
- `docs/reference/agent-doctrine-matrix.generated.md`
- `skills/zmux/references/testing/{prompts.md,answer-key.generated.md}`
- `pi-zmux/references/testing/{prompts.md,answer-key.generated.md}`

The prompt files are complete harness projections. Shared scenario prompt text remains byte-identical because both render from the same `scenarios/*.json` field; harness-only rows require an explicit divergence reason.
