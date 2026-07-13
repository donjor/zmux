# Claude zmux live regression flow

This is the durable human-watchable regression framework for the full Claude/CLI
`zmux` skill projection. A supervising host first agrees the execution lane with
the user, then drives one ordinary visible Claude worker through the generated
shared scenario chain, inspects real terminal state after each checkpoint, and
owns judgment and cleanup.

## Ownership

- [`host-prompt.md`](host-prompt.md) — copy-paste entrypoint for the supervising host.
- [`host-flow.md`](host-flow.md) — Claude launch, sequential setup, evidence, and teardown mechanics.
- `agent-doctrine/scenarios/*.md` — authored scenario prompts and expectations.
- `node agent-doctrine/generate.mjs --render claude-prompts` — validated worker contract and prompts on stdout.
- `node agent-doctrine/generate.mjs --render claude-answer-key` — validated host-only mechanics on stdout.
- [`../../rules/`](../../rules/) — shared behavioral contract source.

Rendered test artifacts are ephemeral command output, not files. Edit the Markdown records and run `make check-doctrine` before the live flow. The host must not assume `zzmux`: native versus edge profile, the work under test, and any skill install/sync are user-confirmed before setup.

## Verdicts

- `PASS` — outcome and concrete host-inspected evidence match the answer key.
- `PASS*` — safe recovery from one invalid call still reaches the outcome; repeated friction is a usability finding.
- `FAIL` — unsafe route, wrong target/session, focus theft, invented evidence, duplicate state, hidden job, or missing cleanup.
- `BLOCKED` — the approved profile, CLI, skill source, auth, model, or lifecycle surface is unavailable; do not substitute docs/self-report as a pass.

Worker self-report and echoed prompt markers are never proof. The host checks roster,
pane IDs, lifecycle generations, output freshness, focus, and cleanup directly.

## Coverage

The Claude projection runs shared scenarios `ZS-001` through `ZS-013`. Pi-only
callback and Pi lifecycle scenarios remain in the Pi framework with explicit
divergence reasons. The host sends only the `claude-prompts` render output;
never expose the `claude-answer-key` output to the worker.

## Reporting

Return one concise line per scenario, then an overall verdict and concrete findings:

```text
PASS ZS-001 — one doctrine-test-server target; fresh ready output; focus unchanged
PASS* ZS-009 — corrected one malformed wait, then matched future output
FAIL ZS-010 — peer answer visible, but no newer ready lifecycle generation
```
