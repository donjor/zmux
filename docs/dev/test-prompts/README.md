# Agent-surface test prompts

These prompts are for fresh-session exploratory QA of zmux's agent-facing surfaces. The Claude/CLI flow uses branch-local docs with isolated `zzmux`; the canonical Pi flow uses the already-synced installed package on native `zmux`.

## Activation model

`./dev.sh zzmux` installs only the isolated edge binary/profile. It intentionally
does **not** refresh shared skill mirrors, global Pi settings, shell startup, or
extension package links.

Use this split when testing:

- **Skill/CLI prompt:** paste `zmux-agent-skill-testing-prompt.md` into a fresh
  agent session started from this repo. The prompt tells the agent to read the
  branch-local `skills/zmux/**` docs, so no global skill mirror refresh is
  required. If you specifically want to test auto-discovery of the shipped skill
  in the live agent installation, run `./dev.sh zmux` separately and reload or
  restart that agent; that mutates live integrations and is not part of `zzmux`
  isolation.
- **Pi extension prompt:** after the native binary and installed Pi package have been synced, launch a fresh `pi` process in an attached native `zmux` session and paste `zmux-agent-pi-zmux-testing-prompt.md`. The flow does not install, sync, switch profiles, or ask the user to choose a lane; missing required surfaces are reported as `BLOCKED`.

## Prompts

These files are thin activation wrappers only:

- `zmux-agent-skill-testing-prompt.md` → durable Claude/CLI framework at
  [`../../../agent-doctrine/testing/claude/`](../../../agent-doctrine/testing/claude/).
- `zmux-agent-pi-zmux-testing-prompt.md` → durable Pi framework at
  [`../../../agent-doctrine/testing/pi/`](../../../agent-doctrine/testing/pi/).

Shared scenario prompts and harness answer keys are authored in
`agent-doctrine/scenarios/*.md` and rendered to stdout with
`agent-doctrine/generate.mjs --render`; exploratory wrappers must not duplicate the chain.
Both frameworks use one ordinary visible worker, host-inspected evidence, explicit
`PASS`/`PASS*`/`FAIL`/`BLOCKED` verdicts, and exact test-owned teardown.
