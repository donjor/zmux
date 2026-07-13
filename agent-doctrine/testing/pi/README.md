# Pi zmux live regression flow

This is the durable human-watchable regression framework for the canonical
`pi-zmux` package. A supervising host drives one ordinary visible Pi worker
through the generated shared scenarios plus Pi-only callback/lifecycle scenarios
in a user-confirmed native or edge lane, inspects real state, and owns judgment
and cleanup.

## Ownership

- [`host-prompt.md`](host-prompt.md) — copy-paste supervising-host entrypoint.
- [`host-flow.md`](host-flow.md) — Pi launch, shared/Pi-only setup, evidence, and teardown mechanics.
- `agent-doctrine/scenarios/*.md` — authored scenario prompts and expectations.
- `node agent-doctrine/generate.mjs --render pi-prompts` — validated worker contract and prompts on stdout.
- `node agent-doctrine/generate.mjs --render pi-answer-key` — validated host-only mechanics on stdout.
- `pi-zmux/doctrine-manifest.generated.json` — committed runtime coverage manifest.
- `pi-zmux/fixtures/` — deterministic runtime and trusted-project fixtures.

Rendered test artifacts are ephemeral command output, not files. Edit the Markdown records and run `make check-doctrine` before the live flow. The host confirms the work under test, binary/profile, and package source with the user before setup; Pi may load the checkout package directly with `pi -e <path>`.

## Verdicts

- `PASS` — outcome and host-inspected evidence match the answer key.
- `PASS*` — the worker safely corrects one invalid dispatcher call before completing the outcome.
- `FAIL` — Bash bypass, raw tmux, hidden job, focus theft, wrong target/session, duplicate state, invented evidence, or cleanup loss.
- `BLOCKED` — the approved profile, package source, model/auth, trust, or lifecycle surface is unavailable; source inspection/self-report cannot substitute.

Worker reports and echoed markers are never proof. The host inspects tool calls,
structured state, lifecycle generations, output freshness, footer/callback state,
focus, and final roster.

## Coverage

Pi runs shared `ZS-001`…`ZS-013`, then Pi-only `ZS-014` callback delivery,
`ZS-015` soft lifecycle, and `ZS-016` automatic detached-command completion.
Adapter-local deterministic tests continue to own trusted-config, Bash guard,
renderer/schema, hard respawn, and non-TUI behavior where no shared live scenario exists.

## Reporting

Return one concise line per scenario plus overall verdict and findings:

```text
PASS ZS-001 — runtime_ensure; one server; fresh ready evidence
PASS* ZS-009 — corrected waitFor shape; then fresh output match
FAIL ZS-014 — callback delivered, but footer remained active
```
