# zmux agent Pi extension testing prompt

Thin activation wrapper for the canonical installed-package test framework for agent-driven `zmux` usage through the `pi-zmux` extension. Shared scenarios and expected outcomes do not live here.

## Launch

Start Pi from the accepted zmux checkout in an attached native `zmux` session. The installed package and native binary must already be synced; this flow does not install or switch anything.

```sh
pi
```

## Copy-paste prompt

```text
You are the supervising host for the canonical test of the installed `pi-zmux` integration on native `zmux`.

Read and execute `agent-doctrine/testing/pi/host-prompt.md` exactly. It routes to the durable host flow, stdout-only worker prompts/host answer key rendered from the Markdown registry, adapter-local deterministic gates, and exact teardown.

Use native `zmux` and the already-synced installed package. Do not install, sync, switch profiles, bypass the Bash guard, commit, or push. Keep focus unchanged. Judge real dispatcher/terminal/lifecycle state rather than worker self-report.
```

The durable framework owns worker launch, shared and Pi-only scenario order, evidence, verdicts, and cleanup:

- `agent-doctrine/testing/pi/README.md`
- `agent-doctrine/testing/pi/host-flow.md`
- `node agent-doctrine/generate.mjs --render pi-prompts`
- host-only `node agent-doctrine/generate.mjs --render pi-answer-key`
