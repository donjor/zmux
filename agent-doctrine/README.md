# Agent doctrine registry

This directory is the obvious authoring home for terminal behavior shared by the
Claude `zmux` skill and Pi `pi-zmux` extension. Package-local generated files are
compiled runtime projections, never authoring surfaces.

## Authored sources

- `rules/ZD-###-*.md` — human-authored neutral invariants, instructions, prompt guidelines, mechanisms, enforcement, caveats, and verification references.
- `scenarios/ZS-###-*.md` — human-authored prompts and expectations. Strict
  JSON-valued YAML frontmatter carries identity/applicability; normal Markdown
  sections carry prompt, setup, outcome, evidence, safety, cleanup, and
  harness-specific host answer keys.
- `testing/claude/**` and `testing/pi/**` — handwritten harness launch,
  inspection, judgment, and teardown mechanics.

A one-harness rule or scenario must include `divergenceReason`. Shared scenario
prompts contain outcomes only; operation names belong in host-only answer-key
sections.

## Generated projections

Run:

```sh
make gen-doctrine
make check-doctrine
```

`make gen-doctrine` validates all authored records and rewrites changed committed
runtime projections. `make check-doctrine` never mutates files and fails only for
invalid authored sources or stale committed projections.

Committed runtime/distribution projections:

- `skills/zmux/references/shared-doctrine.generated.md`
- `pi-zmux/src/generated/doctrine.ts`
- `pi-zmux/doctrine-manifest.generated.json`
- `docs/reference/agent-doctrine-matrix.generated.md`

Maintainer-only live-test renders:

```sh
node agent-doctrine/generate.mjs --render claude-prompts
node agent-doctrine/generate.mjs --render claude-answer-key
node agent-doctrine/generate.mjs --render pi-prompts
node agent-doctrine/generate.mjs --render pi-answer-key
```

These commands validate all records and print one ephemeral artifact to stdout;
they never write. The installed skill and source-loaded Pi package keep only the
committed runtime projections they need.

## Markdown contracts

Rules use the same frontmatter convention and named sections such as `Invariant`,
`Shared instruction`, `Pi prompt guideline`, and `Verify`. Harness projection
sections are present only where the rule applies; `_None._` represents a deliberate
empty prompt guideline or caveat list.

Scenario example:

```md
---
id: "ZS-001"
title: "Start one visible runtime"
doctrineRefs: ["ZD-001","ZD-003","ZD-004"]
applicability: ["claude","pi"]
---

## Prompt

Human-readable worker prompt.

## Setup

- Host-only setup requirement.

## Expected outcome

Human-readable outcome.

## Evidence

- Concrete evidence requirement.

## Safety

- Safety requirement.

## Cleanup

_None._

## Claude answer key

- Host-only Claude mechanics.

## Pi answer key

- Host-only Pi mechanics.
```

Frontmatter values are strict inline JSON, which is valid YAML and keeps parsing
deterministic without introducing a YAML dependency. Sections and Markdown lists
are validated exactly; generated worker prompts never receive answer-key sections.
