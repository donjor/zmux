# Host prompt — pi-zmux live regression flow

You are the supervising host for the canonical `pi-zmux` human-watchable regression flow.

Work from `/home/user/donjor/zmux`. Read:

1. `pi-zmux/references/testing/README.md`
2. `pi-zmux/references/testing/host-flow.md`
3. `pi-zmux/references/testing/prompts.md`

Then execute the flow as written.

Requirements:

- Drive the main chain through one ordinary Pi worker with the settings-managed canonical `pi-zmux` package.
- Keep the worker in a visible stable zmux tab so the human can watch and take over.
- Send the session contract once, then send the checkpoint prompts sequentially and inspect each result before continuing.
- Use the disposable second worker only for the trusted-project and hard-respawn checks described by the flow.
- For lifecycle-only peer checks, launch Pi/Luna-low, Claude/Haiku, Codex/mini-low, and Agy/Flash-low exactly as described; do not silently spend stronger models.
- Cover both shell command lifecycle (`sleep 3`) and all four peer turn lifecycles.
- Own fixture setup, timing, evidence inspection, pass/fail judgment, and teardown.
- Never send answer-key operation names or expected behavior to the worker.
- Stop if unsafe behavior or broken setup invalidates later checks; otherwise continue through the chain.
- Return a concise pass/fail line per checkpoint plus an overall verdict. Do not create run IDs, JSONL results, or transcript directories.

Keep test-owned tabs readable and do not steal focus.
