# zmux agent skill testing prompt

Thin activation wrapper for the durable Claude/CLI skill regression framework.
Shared scenarios and expected outcomes do not live here.

## Isolated launch

Work from the accepted zmux checkout in an attached test terminal:

```sh
./dev.sh zzmux
```

`./dev.sh zzmux` is binary/profile-only. It does not refresh live skill mirrors, shell integration, or Pi packages.

## Copy-paste prompt

```text
You are the supervising host for the branch-local Claude/CLI zmux regression.

Read and execute `agent-doctrine/testing/claude/host-prompt.md` exactly. It routes to the durable host flow and stdout-only worker prompts/host answer key rendered from the Markdown registry.

Use only isolated `zzmux`; never mutate live `zmux`, refresh global mirrors, install hooks, commit, or push. Keep focus unchanged. Judge real terminal/lifecycle state rather than worker self-report, and perform exact test-owned cleanup.
```

The durable framework owns worker launch, scenario order, evidence, verdicts, and teardown:

- `agent-doctrine/testing/claude/README.md`
- `agent-doctrine/testing/claude/host-flow.md`
- `node agent-doctrine/generate.mjs --render claude-prompts`
- host-only `node agent-doctrine/generate.mjs --render claude-answer-key`
