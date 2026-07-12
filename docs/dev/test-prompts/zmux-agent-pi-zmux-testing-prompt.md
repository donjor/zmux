# zmux agent Pi extension testing prompt

Thin activation wrapper for the durable branch-local Pi dispatcher regression framework.
Shared scenarios and expected outcomes do not live here.

## Isolated launch

Work from the accepted zmux checkout in an attached test terminal:

```sh
./dev.sh zzmux
PI_ZMUX_BIN=zzmux pi -ne -e ./pi-zmux
```

`-ne` suppresses globally discovered extensions so the live installed package cannot register a duplicate tool. The explicit branch package remains loaded, and `PI_ZMUX_BIN=zzmux` isolates dispatcher operations.

## Copy-paste prompt

```text
You are the supervising host for the branch-local canonical pi-zmux regression.

Read and execute `pi-zmux/references/testing/host-prompt.md` exactly. It routes to the durable host flow, generated worker prompts, generated host-only answer key, adapter-local deterministic gates, and exact teardown.

Use only isolated `zzmux`; never mutate live `zmux`, refresh global integrations, bypass the Bash guard, commit, or push. Keep focus unchanged. Judge real dispatcher/terminal/lifecycle state rather than worker self-report.
```

The durable framework owns worker launch, shared and Pi-only scenario order, evidence, verdicts, and cleanup:

- `pi-zmux/references/testing/README.md`
- `pi-zmux/references/testing/host-flow.md`
- generated `pi-zmux/references/testing/prompts.md`
- generated host-only `pi-zmux/references/testing/answer-key.generated.md`
