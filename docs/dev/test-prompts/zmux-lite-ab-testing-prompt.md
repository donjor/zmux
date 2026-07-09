---
origin:
  interface: pi
  provider: openai
  model: gpt-5.5
  variant: unknown
last_touch:
  interface: pi
  provider: openai
  model: gpt-5.5
  variant: unknown
signoff: none
touched: 2026-07-10
---
# zmux lite A/B testing prompt

Use this prompt in a fresh isolated Pi session to collect comparable current-vs-lite routing evidence.

## Setup

Run one profile at a time from the zmux repo root:

```bash
./dev.sh zzmux
PI_ZMUX_BIN=zzmux pi -ne -e ./pi-extension
PI_ZMUX_BIN=zzmux pi -ne -e ./pi-extension-lite
```

Use the same model and cwd for both runs. Store JSONL result records under:

```text
.dump/zmux-lite-results/<profile>/<run-id>.jsonl
```

## Instructions for the test agent

You are testing agent routing, not trying to be clever. For each scenario in `pi-extension-lite/scenarios/zmux-lite-scenarios.json`:

1. Read the scenario prompt exactly.
2. Act naturally from the prompt and current tools.
3. Do not name or force a tool unless the scenario itself asks for an explicit compatibility check.
4. Record the actual tool calls or attempted shell commands.
5. Mark pass/fail against `expectedBehavior`, not against the legacy tool name.
6. Classify any failure as one of:
   - `schema-wording`
   - `missing-operation`
   - `missing-first-class-tool`
   - `implementation-bug`
   - `source-cli-pruning-candidate`
   - `unsafe-routing`
   - `harness-error`
7. If the lite run fails, first decide whether better dispatcher wording or validation could fix it before proposing another first-class tool.

## Result row shape

```json
{
  "runId": "2026-07-10T00-00-00Z-lite-gpt-5.5",
  "profile": "lite",
  "model": "openai/gpt-5.5",
  "promptId": "N-001-runtime-start",
  "expectedBehavior": "Uses a zmux-managed runtime/tab...",
  "actualToolCalls": [
    { "tool": "zmux_lite", "args": { "operation": "runtime_ensure" } }
  ],
  "pass": true,
  "failureClass": "pass",
  "transcriptRef": ".dump/zmux-lite-results/lite/transcripts/N-001.md",
  "implication": "none",
  "notes": ""
}
```

The current profile is the behavior oracle. The lite profile is allowed to fail early; failures are useful only when the record explains whether the problem is schema wording, a missing dispatcher operation, implementation behavior, or a genuinely necessary first-class tool.
