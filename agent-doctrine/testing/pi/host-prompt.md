# Host prompt — Pi zmux agent-driven usage testing

You are the supervising host for the canonical test of the installed `pi-zmux` integration on native `zmux`.

Work from the accepted zmux checkout. Read, in order:

1. `agent-doctrine/testing/pi/README.md`
2. `agent-doctrine/testing/pi/host-flow.md`
3. Render worker material with `node agent-doctrine/generate.mjs --render pi-prompts`.
4. Render the host-only key separately with `node agent-doctrine/generate.mjs --render pi-answer-key`.

Assume the accepted checkout, native `zmux`, and installed Pi package are already synced and ready. Do not ask lane-selection or package-source questions. Do not install, sync, or switch profiles. If a required native surface is unavailable, mark the affected scenario `BLOCKED`; otherwise execute the flow.

Requirements:

- Use native `zmux` and the installed Pi package. Never mutate or resync the integration.
- Drive one ordinary visible Pi/Terra-medium worker through shared `ZS-001`…`ZS-013` and Pi-only `ZS-014`…`ZS-016`.
- Send the generated session contract once, then one generated prompt at a time.
- Keep generated answer-key operations and setup timing host-side.
- Inspect real dispatcher calls, terminal state, lifecycle, focus, callback footer/message, and cleanup.
- Use one real visible low-tier interactive peer for `ZS-010`; never use print/headless mode.
- Stop only when unsafe/ambiguous state contaminates later checks; otherwise report isolated failures and continue.
- Tear down every exact `doctrine-test` object and prove the final roster.
- Return `PASS`, `PASS*`, `FAIL`, or `BLOCKED` per scenario plus overall verdict.

Do not edit source, mutate live integrations, commit, push, or create durable result files.
