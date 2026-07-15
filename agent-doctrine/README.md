# Agent doctrine registry

This directory is the authoring home for shared terminal behavior and strict
natural-prompt live scenarios. Generated package files are runtime projections,
never authoring surfaces.

Records stay harness-neutral: every rule and scenario projects to both Claude and
Pi. The Pi extension package and its Pi-only records, harness, and campaign
ledgers are deferred and live on `feat/pi-zmux-parallel-delegation`; they return
whole when the Pi extension is revisited.

## Authored sources

- `rules/shared/ZD-###-*.md` — portable invariants and Claude/Pi projections.
- `scenarios/shared/ZS-###-*.md` — portable atomic, workflow, and resilience jobs.
- `harnesses/claude/**` — launch, host perturbation, inspection, five-lens judgment, and teardown mechanics.

Worker prompt bodies cannot contain internal operation identifiers or
callback/queue/lease test vocabulary outside fenced payloads.

## Three proof layers

1. Deterministic Go and generator tests own internal permutations and state matrices.
2. Natural-prompt scenarios own real agent routing, composition, resilience, presentation, responsiveness, and cleanup.
3. Human UAT samples a small `uatEligible` atomic/workflow subset.

Live scenario tiers:

- `atomic` — exactly one natural prompt and no host perturbation;
- `workflow` — two or more natural turns and no injected fault;
- `resilience` — natural turns plus one or more host-only perturbations.

Every record defines hard `outcome`, `orchestration`, `responsiveness`, `presentation`, and `cleanup` verdicts.

## Generated projections and validation

```sh
make gen-doctrine
make check-doctrine
```

`make gen-doctrine` validates rules and scenarios, declared-operation mentions,
and prompt leakage before rewriting changed projections. `make check-doctrine` is
non-mutating.

Committed projections:

- `skills/zmux/references/shared-doctrine.generated.md`
- `docs/reference/agent-doctrine-matrix.generated.md`

Maintainer-only stdout renders:

```sh
node agent-doctrine/generate.mjs --render claude-answer-key --tier atomic
node agent-doctrine/generate.mjs --render claude-prompts --ids ZS-001
node agent-doctrine/generate.mjs --render pi-answer-key --tier resilience
node agent-doctrine/generate.mjs --render pi-prompts --tier workflow --ids ZS-013,ZS-018
```

A single selected turn renders as exact copy/paste worker input. Multi-turn/tier
output uses `BEGIN/END HOST TURN` HTML comments for the supervising host; send only
each enclosed body. Answer-key output always remains host-side. Renders never write
files.

Live workers run from an allowlisted disposable sandbox outside the checkout. The
sandbox contains only selected-row subject files and excludes `agent-doctrine/`,
`.dump/`, harnesses, and answer keys; otherwise prompt search can disclose the host
contract and invalidate the row.

## Scenario Markdown contract

```md
---
id: "ZS-001"
title: "Run one finite visible command"
tier: "atomic"
doctrineRefs: ["ZD-001","ZD-003"]
applicability: ["claude","pi"]
uatEligible: true
---

## Prompt 1

Ordinary user request, preserved verbatim.

## Host setup

- Supervising-host baseline or fixture.

## Host perturbations

_None._

## Verdict

- outcome: Observable task bar.
- orchestration: Smallest correct route and forbidden redundancy.
- responsiveness: Host/editor/focus bar.
- presentation: Timing, styling, useful output, and truth bar.
- cleanup: Exact baseline and no-delayed-traffic bar.

## Evidence

- Concrete host-inspected proof.

## Cleanup

- Exact host-owned teardown.

## Claude answer key

- Host-only Claude mechanics.

## Pi answer key

- Host-only Pi mechanics.
```

`Prompt 1`, `Prompt 2`, and later turns are sequential and verbatim. Each shared
record carries both the Claude and Pi answer keys; the Pi key stays host-side and
never reaches a worker.

Frontmatter values are strict inline JSON. `_None._` represents an intentionally
empty perturbation or cleanup list. The generator rejects malformed tiers,
verdicts, leaked host sections, undeclared operations, and stale projections.
