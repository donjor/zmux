# QA Walkthroughs

`./qa` is the repo-local QA walkthrough runner. It is a separate binary
(`cmd/qa`) launched by the root wrapper, not a `zmux qa` subcommand.

## Storage

- Specs: committed `checklists/*.toml`.
- Scorecards: gitignored `.qa/*.json`.
- Cached runner binary: gitignored `.qa/bin`.

## Commands

```bash
./qa                               # picker: checklist select -> steps
./qa ls                            # list checklists + scorecard summaries
./qa run <checklist>               # run all automatic steps
./qa run <checklist> <step...>     # run selected steps
./qa mark <checklist> <step> pass|fail --note '...'
./qa status <checklist> [--json]
./qa reset <checklist> [--force]
./qa lint
```

Exit codes for `run` and `status` are part of the contract:

- `0` — all steps pass.
- `1` — at least one step failed or errored.
- `2` — at least one step is pending or unrun.

## Checklist Semantics

Each step can combine an executable command, an expected human-visible outcome,
and an optional regexp check:

- `cmd` + `check`: automatic. The runner executes the command and marks pass
  when stdout/stderr matches the Go regexp.
- `cmd` only: assisted. The runner executes the command, then leaves the step
  pending for a human or agent verdict.
- neither: instruction-only. The picker presents the expectation and records a
  verdict.

Step commands run from the repo root through `sh -c` with a default timeout.
Use checklist `vars` for repeated values like `bin = "zzmux"`.

## Current Checklist

`checklists/tab-placements.toml` verifies the logical-tab placement work:

- `zmux tab pane/full/hide/show`
- hidden dock behavior
- bar glyphs and logical tab rows
- Alt+` switcher rows for full, rider, and hidden tabs

It targets the isolated `zzmux` profile so QA does not disturb a live `zmux`
session.
