# docs/reafactor — the architecture-refactor record

> Directory name keeps its original spelling (`reafactor`) on purpose — it's
> referenced by ROADMAP, commits, and this log. Renaming would rewrite history.

The complete record of zmux's architecture restructure: the planning artifacts,
the per-phase follow-up plans, and a chronological log. It doubles as the worked
example behind a future `dir-structure` skill (`~/donjor/skills/IDEAS/dir-structure/`).

## Reading order

1. **`dir-tree-pre-refactor.md`** — the starting layout (was `dir-tree-current.md`;
   renamed when later snapshots were added). Point-in-time; don't edit.
2. **`dir-tree-ideal-blind.md`** — the ideal proposed from **docs only**, no source
   read. The uncontaminated hypothesis.
3. **`dir-tree-ideal-with-context.md`** — the **true ideal**, revised from the blind
   tree using real code knowledge (import edges, sizes, coupling). The deliverable.
4. **`followup-01..04`** — doc-first follow-up plans (then execution notes). In
   order: 01 cli-extraction, 02 tooling, 03 B-purity seams, 04 source prober.
   *These are also the gap* — see the RUNDOWN's "catch / avoid / improve" section.
5. **`RUNDOWN-LIGHT-LOG.md`** — chronological log of the whole effort + the
   distilled **lessons for the skill**. Start here for the executive view.

## Snapshots (point-in-time; historical, not edited to current truth)

- `dir-tree-post-omega.md` — after the omega restructure merged.
- `dir-tree-post-cli.md` — after followup-01 (cli extraction) merged.

## The methodology (why three ideal/current artifacts)

A single "ideal structure" doc silently anchors to whatever the repo already is.
Splitting into **blind → current → with-context** forces the status-quo bias into
the open and makes the reasoning reviewable. The failure modes this guards against
(and how they actually bit this project) are written up in the RUNDOWN's
"For the dir-structure skill" section.
