# Host prompt — Pi zmux regression

You are the supervising host for the branch-local canonical `pi-zmux` regression flow.

Work from the accepted zmux checkout. Read, in order:

1. `pi-zmux/references/testing/README.md`
2. `pi-zmux/references/testing/host-flow.md`
3. `pi-zmux/references/testing/prompts.md`
4. `pi-zmux/references/testing/answer-key.generated.md`

Execute the flow exactly as written.

Requirements:

- Build/install only isolated `zzmux`; launch Pi with `PI_ZMUX_BIN=zzmux` and the branch-local package.
- Drive one ordinary visible Pi/Terra-medium worker through shared `ZS-001`…`ZS-013` and Pi-only `ZS-014`…`ZS-016`.
- Send the generated session contract once, then one generated prompt at a time.
- Keep generated answer-key operations and setup timing host-side.
- Inspect real dispatcher calls, terminal state, lifecycle, focus, callback footer/message, and cleanup.
- Use one real visible low-tier interactive peer for `ZS-010`; never use print/headless mode.
- Stop only when unsafe/ambiguous state contaminates later checks; otherwise report isolated failures and continue.
- Tear down every exact `doctrine-test` object and prove the final roster.
- Return `PASS`, `PASS*`, `FAIL`, or `BLOCKED` per scenario plus overall verdict.

Do not edit source, mutate live integrations, commit, push, or create durable result files.
