# Host prompt — Claude zmux skill regression

You are the supervising host for the branch-local Claude/CLI zmux doctrine regression flow.

Work from the accepted zmux checkout. Read, in order:

1. `agent-doctrine/testing/claude/README.md`
2. `agent-doctrine/testing/claude/host-flow.md`
3. Render worker material with `node agent-doctrine/generate.mjs --render claude-prompts`.
4. Render the host-only key separately with `node agent-doctrine/generate.mjs --render claude-answer-key`.

Execute the flow exactly as written.

Requirements:

- Build/install only isolated `zzmux`; never mutate the live `zmux` profile or global skill mirrors.
- Drive one ordinary visible Claude/Sonnet worker through the sequential shared chain.
- Send the generated session contract once, then one generated scenario prompt at a time.
- Keep answer-key operations and setup timing host-side.
- Inspect real terminal/session/lifecycle state after every prompt; self-report is supporting evidence only.
- Keep terminal focus unchanged and pin the isolated session on every read and write.
- Use a real visible low-tier interactive peer for `ZS-010`; headless/print mode is a failure.
- Continue after a safe isolated failure; stop when contaminated or ambiguous state invalidates later checks.
- Tear down every exact `doctrine-test` object and prove the final roster.
- Return `PASS`, `PASS*`, `FAIL`, or `BLOCKED` per scenario plus one overall verdict.

Do not edit source, install live hooks, commit, push, or create durable report files.
