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

Read and execute `skills/zmux/references/testing/host-prompt.md` exactly. It routes to the durable host flow, generated worker prompts, and generated host-only answer key.

Use only isolated `zzmux`; never mutate live `zmux`, refresh global mirrors, install hooks, commit, or push. Keep focus unchanged. Judge real terminal/lifecycle state rather than worker self-report, and perform exact test-owned cleanup.
```

The durable framework owns worker launch, scenario order, evidence, verdicts, and teardown:

- `skills/zmux/references/testing/README.md`
- `skills/zmux/references/testing/host-flow.md`
- generated `skills/zmux/references/testing/prompts.md`
- generated host-only `skills/zmux/references/testing/answer-key.generated.md`
