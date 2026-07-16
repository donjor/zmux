# Agent-surface test prompts

These prompts are for fresh-session exploratory QA of zmux's agent-facing surfaces. The Claude/CLI host confirms a native-or-edge lane with the user; the canonical Pi flow uses the already-synced installed package on native `zmux`.

## Activation model

`./dev.sh zzmux` installs only the isolated edge binary/profile. It intentionally
does **not** refresh shared skill mirrors, global Pi settings, shell startup, or
extension package links.

Use this split when testing:

- **Skill/CLI prompt:** paste `zmux-agent-skill-testing-prompt.md` into a fresh
  host session started from this repo. The host confirms whether to use native
  `zmux` or isolated `zzmux`, which checkout/install is under test, and whether
  any skill sync is allowed before setup. Branch-local docs can be read without
  refreshing a global mirror; installed-skill coverage requires an explicitly
  approved and verified sync.
- **Pi extension prompt:** after the native binary and installed Pi package have been synced, launch a fresh `pi` process in an attached native `zmux` session and paste `zmux-agent-pi-zmux-testing-prompt.md`. The flow does not install, sync, switch profiles, or ask the user to choose a lane; missing required surfaces are reported as `BLOCKED`.

## Prompts

These files are thin activation wrappers only:

- `zmux-agent-skill-testing-prompt.md` → durable Claude/CLI framework at
  [`../../../agent-doctrine/harnesses/claude/`](../../../agent-doctrine/harnesses/claude/).
- `zmux-agent-pi-zmux-testing-prompt.md` → durable Pi framework at
  `agent-doctrine/harnesses/pi/` — deferred with the Pi extension reintegration and
  not present on this shared branch.

Shared scenario prompts and harness answer keys are authored in
`agent-doctrine/scenarios/shared/*.md` and rendered to stdout with
`agent-doctrine/generate.mjs --render`; exploratory wrappers must not duplicate the chain.
Both frameworks use one ordinary visible worker, host-inspected evidence, explicit
`PASS`/`FAIL`/`BLOCKED` verdicts (there is no `PASS*`), and exact test-owned teardown.
