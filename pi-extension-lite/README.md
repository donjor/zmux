# pi-zmux-lite

WIP one-tool Pi extension used to test whether the 37-tool `pi-zmux` surface can shrink to a compact dispatcher without losing normal zmux behavior.

## Launch profiles

Current oracle profile:

```bash
PI_ZMUX_BIN=zzmux pi -ne -e ./pi-extension
```

Lite candidate profile:

```bash
PI_ZMUX_BIN=zzmux pi -ne -e ./pi-extension-lite
```

`-ne` keeps globally installed extensions out of the run so the current and lite zmux surfaces do not cross-contaminate. `PI_ZMUX_BIN=zzmux` keeps tests on the isolated zmux/tmux profile.

## Tool surface

The lite profile registers exactly one model-visible tool: `zmux_lite`.

The tool accepts:

- `operation` — required compact action name;
- `target` — primary tab/session/pane/runtime target;
- `command` — shell command for run/runtime/pane/manual operations;
- `cwd` — optional working directory override;
- `options` — small operation-specific fields.

Start from `operation=current` or `operation=tabs` when the target is ambiguous. Extra first-class tools are not added here unless scenario evidence repeatedly proves the dispatcher cannot route safely.

## Scenario harness

Scenario definitions live in [`scenarios/zmux-lite-scenarios.json`](scenarios/zmux-lite-scenarios.json). Each prompt is written as an outcome request, not a tool-name hint, except future compatibility scenarios that explicitly say otherwise.

Normalized result records should be JSONL under:

```text
.dump/zmux-lite-results/<profile>/<run-id>.jsonl
```

Each row records profile, model, prompt id, expected behavior, actual tool calls or commands, pass/fail, failure class, transcript pointer, and implication.

## Deterministic checks

```bash
npm run typecheck
npm test
```

The test compiles this package, verifies that only `zmux_lite` is registered, validates the scenario suite shape, and compares the one-tool schema estimate against the current `pi-zmux` extension.
