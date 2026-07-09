# Agent-surface test prompts

These prompts are for fresh-session exploratory QA of zmux's agent-facing surfaces.
They pair branch-local source docs with an isolated `zzmux` runtime profile.

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
- **Pi extension prompt:** launch a fresh Pi process with the branch-local
  extension and isolated binary:

  ```sh
  ./dev.sh zzmux
  PI_ZMUX_BIN=zzmux pi -ne -e ./pi-zmux
  ```

  Then paste `zmux-agent-pi-zmux-testing-prompt.md` into that Pi session.
  `-ne` disables globally discovered extensions so the already-installed live
  `zmux/pi-zmux` cannot register duplicate `zmux_*` tools. The explicit
  `-e ./pi-zmux` still loads this branch's extension for that process, and
  `PI_ZMUX_BIN=zzmux` routes typed `zmux_*` tools to the isolated binary/profile.

Current already-running Pi sessions will not gain new tools from `./dev.sh
zzmux`. To test new tools there, the live package/mirror path must be refreshed
and Pi must reload or restart; prefer the fresh `pi -ne -e ./pi-zmux` path
for isolated QA.

## Prompts

- `zmux-agent-skill-testing-prompt.md` — shared skill/CLI doctrine and safe
  `zzmux` smoke checks.
- `zmux-agent-pi-zmux-testing-prompt.md` — active Pi tool inventory,
  guardrails, branch-local extension behavior, and typed-tool smoke checks.
