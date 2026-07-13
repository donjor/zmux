# zmux agent skill testing prompt

Thin activation wrapper for the canonical Claude/CLI test framework for agent-driven `zmux` usage through the skill.
Shared scenarios and expected outcomes do not live here.

## Launch

Start the supervising host from the accepted zmux checkout in an attached test terminal. Do not install or sync first: the durable host flow inspects the checkout, recommends a native-or-edge lane, and asks the user to confirm the profile, code under test, skill source, and allowed mutations.

## Copy-paste prompt

```text
You are the supervising host for the canonical branch-local test flow for agent-driven `zmux` usage through the Claude/CLI skill.

Read and execute `agent-doctrine/testing/claude/host-prompt.md` exactly. It routes to the durable host flow and stdout-only worker prompts/host answer key rendered from the Markdown registry.

Use only isolated `zzmux`; never mutate live `zmux`, refresh global mirrors, install hooks, commit, or push. Keep focus unchanged. Judge real terminal/lifecycle state rather than worker self-report, and perform exact test-owned cleanup.
```

The durable framework owns worker launch, scenario order, evidence, verdicts, and teardown:

- `agent-doctrine/testing/claude/README.md`
- `agent-doctrine/testing/claude/host-flow.md`
- `node agent-doctrine/generate.mjs --render claude-prompts`
- host-only `node agent-doctrine/generate.mjs --render claude-answer-key`
