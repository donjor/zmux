# zmux agent skill testing prompt

Use this file as a copy-paste prompt for a fresh, isolated agent session. It tests the **branch-local zmux skill docs and CLI doctrine** against an isolated `zzmux` profile. It does not require global skill mirrors to be refreshed. It does not test Pi typed-tool internals; use `zmux-agent-pi-extension-testing-prompt.md` for that.

````text
You are testing the zmux agent-facing skill surface in a fresh isolated session.

## Mission

Run full live E2E QA for the zmux skill documentation and isolated `zzmux` agent surface. Write a structured report with evidence to `.dump/test-prompts-report/`; creating that report file is allowed and required. This prompt is the explicit authorization for **one bounded nested peer CLI** inside the isolated `zzmux` profile so the test proves real peer spawning, typing, output observation, lifecycle metadata, worker targeting, and cleanup. Do not edit source files, commit, push, install global live-profile hooks, or touch the live `zmux` profile.

## Repo and safety setup

1. Work from the zmux repo root. If you are not in the repo, ask for the path.
2. Read `docs/dev/test-prompts/README.md` for the activation model. `./dev.sh zzmux` does not refresh global skill mirrors; this prompt tests branch-local docs directly.
3. Use bounded shell commands for source reads, greps, builds, and `zzmux` smoke checks.
4. Use the isolated `zzmux` profile for objective live checks. Do **not** mutate the live `zmux` profile.
5. Do not use raw `tmux` for app-level behavior. Raw `tmux -L zzmux` is allowed only for narrow diagnostics when a documented `zzmux` verb cannot expose the state.
6. Do not run `./dev.sh` without the `zzmux` argument. `./dev.sh zzmux` is allowed; live `./dev.sh` is not.
7. Do not run `zmux refresh`, `zmux terminal refresh`, `zmux init`, live reaping activation, or destructive live-profile commands.
8. This prompt authorizes exactly one bounded real nested peer CLI if `claude`, `codex`, `pi`, or `agy` is available. The peer must receive a marker-only prompt, produce evidence, and be cleaned up. If no peer CLI is available or auth blocks it, report `BLOCKED`; do not downgrade to docs-only pass.
9. Worker checks must prove command completion/interaction, not just detached-session spawn.
10. Clean up any `zzmux` sessions/tabs you create.

## Read first

Read these files before judging behavior:

- `docs/dev/test-prompts/README.md`
- `skills/zmux/SKILL.md`
- `skills/zmux/references/run-observe.md`
- `skills/zmux/references/cli-catalog.md`
- `skills/zmux/references/guard-and-tab-states.md`
- `skills/zmux/references/agent-peer.md`
- `skills/zmux/references/agent-worker.md`
- `docs/dev/agent-grounding.md`
- `docs/domains/pi-zmux-extension.md` only for boundaries between shared skill doctrine and Pi extension behavior

Do not rely on model memory for expected behavior; cite these docs in the report.

## Deterministic gates

Run these if available:

```sh
make test-agent-surfaces
./qa lint
```

If a command is unavailable or fails due environment setup, report `BLOCKED` with stdout/stderr summary. Do not hide it as pass.

## Live CLI inventory gate

After `./dev.sh zzmux`, enumerate the active top-level CLI verbs:

```sh
zzmux --help || zzmux help
```

Compare the visible top-level verbs and command groups against this prompt's coverage checklist and `skills/zmux/references/cli-catalog.md`. If a top-level verb or agent-relevant command group appears in the live CLI but is not covered by the checklist or routed through `cli-catalog.md`, report `FAIL: uncovered CLI surface`. If help output is unavailable, report `BLOCKED` with the exact command output.

## Objective `zzmux` smoke checks

First install the isolated edge binary/profile and record versions:

```sh
./dev.sh zzmux
zzmux version
zmux version || true
zzmux --help || zzmux help
```

There are two different live-check modes. Keep them separate in your report. If this prompt runs from a headless shell instead of an attached `zzmux` client, do not improvise against the live profile: mark attached-only current-session checks `BLOCKED: not inside attached zzmux session`, then continue only with checks that can target an existing isolated `zzmux` workspace/session through documented `-s/--session` surfaces. If no isolated workspace/session is discoverable, report `BLOCKED` with the discovery output.

### A. Human-visible current-session checks

These checks are meant to be visible to a human watching the attached `zzmux`
edge environment. They must run from inside an attached `zzmux`/tmux session. If
the current tmux socket is not the `zzmux` socket, or `zzmux pane current --json`
fails, report this section as `BLOCKED: not inside an attached zzmux session` and
do not replace it with detached worker checks. A raw `tmux display-message` read
is allowed here only as a socket diagnostic; do not use raw tmux for app actions.

Use one run id for all live sections. If you run the shell snippets separately, copy the same `RUN_ID` into each shell; changing it mid-test is a prompt failure. Use unique tab names so cleanup cannot destroy a user's existing `scratch` or `dev` tab:

```sh
RUN_ID="${RUN_ID:-agent-skill-$(date +%s)-$$}"
export RUN_ID
SCRATCH="skill-scratch-$RUN_ID"
DEV="skill-dev-$RUN_ID"
PANE="skill-pane-$RUN_ID"
CURRENT_SOCKET="$(tmux display-message -p '#{socket_path}' 2>/dev/null || true)"

if [ -z "$CURRENT_SOCKET" ] || ! printf '%s\n' "$CURRENT_SOCKET" | grep -q 'zzmux'; then
  printf 'BLOCKED: current tmux socket is not zzmux: %s\n' "${CURRENT_SOCKET:-none}"
elif ! zzmux pane current --json; then
  printf 'BLOCKED: zzmux pane current failed\n'
else
  zzmux where --json || true

  zzmux run 'echo one-shot-ok' -n "$SCRATCH" -T 30
  zzmux watch "$SCRATCH" -l 80
  zzmux tab status "$SCRATCH" --json

  zzmux log start "$SCRATCH"
  zzmux run 'echo logged-after-start' -n "$SCRATCH" -T 30
  zzmux log status || true
  zzmux log tail "$SCRATCH" || true
  zzmux log stop "$SCRATCH" || true

  zzmux run 'while true; do sleep 1; echo ready-service; sleep 60; done' -n "$DEV" -d --scope daemon
  zzmux wait "$DEV" --for output:'ready-service' --json -T 30
  zzmux send "$DEV" C-c
  zzmux wait "$DEV" --for idle:2s --json -T 20 || true

  zzmux tab state ready "$SCRATCH" --msg 'skill smoke ready'
  zzmux tab status "$SCRATCH" --json
  zzmux pane list || true
  zzmux pane list --session || true

  PANE_ID="$(zzmux pane open --no-focus "$PANE" -r 30 -- bash -lc 'echo pane-ready; sleep 60')"
  zzmux pane list --json || true
  # Pane title lookup is current-window scoped; detached agent shells can have
  # their active window moved by earlier run commands. Lifecycle proof therefore
  # uses the concrete pane id returned by pane open.
  zzmux pane resize "$PANE_ID" --size 35%
  zzmux pane close "$PANE_ID"

  zzmux snapshot --no-png || true

  zzmux tab kill "$DEV" || true
  zzmux tab kill "$SCRATCH" || true
fi
```

### B. Real visible peer E2E check

This is the part that prevents a thin docs-only pass. It must launch one real
nested peer in an isolated `zzmux` tab when a peer CLI is available, type a
marker prompt into it, observe the marker in output, write lifecycle metadata,
and clean up. If the peer CLI is missing or cannot authenticate, report
`BLOCKED: real nested peer unavailable`; do not replace this with documentation
inspection.

```sh
RUN_ID="${RUN_ID:-agent-skill-$(date +%s)-$$}"
export RUN_ID
CURRENT_SOCKET="$(tmux display-message -p '#{socket_path}' 2>/dev/null || true)"
PEER="skill-peer-$RUN_ID"
PEER_CMD=""
PEER_READY='Claude Code|Codex|bypass permissions|❯|›'

if [ -z "$CURRENT_SOCKET" ] || ! printf '%s\n' "$CURRENT_SOCKET" | grep -q 'zzmux'; then
  printf 'BLOCKED: current tmux socket is not zzmux: %s\n' "${CURRENT_SOCKET:-none}"
elif command -v claude >/dev/null 2>&1; then
  PEER_CMD='claude --dangerously-skip-permissions'
elif command -v codex >/dev/null 2>&1; then
  PEER_CMD='codex'
elif command -v pi >/dev/null 2>&1; then
  PEER_CMD='pi -c'
elif command -v agy >/dev/null 2>&1; then
  PEER_CMD='agy'
fi

if [ -z "$PEER_CMD" ]; then
  echo 'BLOCKED: no supported peer CLI found for real nested peer test'
else
  zzmux run "$PEER_CMD" -n "$PEER" -d --scope peer
  zzmux watch "$PEER" --until "$PEER_READY" -T 120

  zzmux tab peer start "$PEER" --role "${PEER_CMD%% *}" --topic "zzmux skill e2e $RUN_ID"
  zzmux tab status "$PEER" --json

  zzmux type "$PEER" "Reply with prefix PEER_E2E_ immediately followed by this run id, and no other text. Run id: $RUN_ID"
  if ! zzmux watch "$PEER" --until "PEER_E2E_$RUN_ID" -T 120; then
    if zzmux watch "$PEER" -l 160 | grep -F "PEER_E2E_$RUN_ID"; then
      echo 'marker already present in peer output; treating buffer evidence as proof'
    else
      echo 'FINDING: marker not observed after zmux type; retrying exactly one Enter per submission hygiene'
      zzmux send "$PEER" Enter
      zzmux watch "$PEER" --until "PEER_E2E_$RUN_ID" -T 120
    fi
  fi
  zzmux watch "$PEER" --idle 2 -T 10 || true

  zzmux tab peer ready "$PEER" --source e2e --msg 'nested peer answered'
  zzmux tab status "$PEER" --json
  zzmux tab peer consumed "$PEER" || true
fi
```

### C. Detached worker/session-target E2E check

This is intentionally headless. It verifies the worker primitive without stealing
focus or adding tabs to the current session. It must prove more than spawn: the
test types into the worker, waits for the worker response, exercises the fixed
`workspace/session` targeting surfaces, performs a placement round-trip, and
checks `session kill workspace/session`.

Run it only after the same socket check confirms the current attached session is
on `zzmux`, and `zzmux where --json` shows a non-empty workspace. If the workspace
is unavailable, report this section as `BLOCKED` instead of inventing a new
`--workspace` value. `zzmux session run --workspace <name>` requires that the
workspace already exists.

```sh
RUN_ID="${RUN_ID:-agent-skill-$(date +%s)-$$}"
export RUN_ID
CURRENT_SOCKET="$(tmux display-message -p '#{socket_path}' 2>/dev/null || true)"
WORKER="skill-worker-$RUN_ID"
KILLER="skill-kill-$RUN_ID"
PEER="${PEER:-skill-peer-$RUN_ID}"

if [ -z "$CURRENT_SOCKET" ] || ! printf '%s\n' "$CURRENT_SOCKET" | grep -q 'zzmux'; then
  printf 'BLOCKED: current tmux socket is not zzmux: %s\n' "${CURRENT_SOCKET:-none}"
else
  WORKSPACE="$(zzmux where --json | node -e 'const fs=require("fs"); const data=JSON.parse(fs.readFileSync(0,"utf8")); process.stdout.write(data.workspace || "");')"
  if [ -z "$WORKSPACE" ]; then
    echo "BLOCKED: no current workspace for detached worker check"
  else
    zzmux session run "$WORKER" -n worker -- bash -lc 'sleep 1; echo worker-ready; while IFS= read -r line; do sleep 1; echo "worker-saw:$line"; done'
    zzmux wait worker -s "$WORKSPACE/$WORKER" --for output:'worker-ready' --json -T 30

    zzmux pane list --session --target "$WORKSPACE/$WORKER"
    zzmux tab state ready worker -s "$WORKSPACE/$WORKER" --msg 'state via workspace/session'
    zzmux tab status worker -s "$WORKSPACE/$WORKER" --json

    zzmux type worker -s "$WORKSPACE/$WORKER" "hello-worker-$RUN_ID"
    zzmux wait worker -s "$WORKSPACE/$WORKER" --for output:"worker-saw:hello-worker-$RUN_ID" --json -T 30

    zzmux tab peer ready worker -s "$WORKSPACE/$WORKER" --source e2e --msg 'worker lifecycle via -s'
    zzmux tab status worker -s "$WORKSPACE/$WORKER" --json

    zzmux run 'sleep 1; echo side-ready; sleep 60' -n side -s "$WORKSPACE/$WORKER" -d --scope daemon
    zzmux watch side -s "$WORKSPACE/$WORKER" --until 'side-ready' -T 30
    zzmux tab pane side -s "$WORKSPACE/$WORKER" --into worker --size 30%
    zzmux pane list --joined --target "$WORKSPACE/$WORKER"
    zzmux tab full side -s "$WORKSPACE/$WORKER"

    zzmux session run "$KILLER" -n killtab -- bash -lc 'sleep 1; echo kill-ready; sleep 60'
    zzmux watch killtab -s "$WORKSPACE/$KILLER" --until 'kill-ready' -T 30
    zzmux session kill "$WORKSPACE/$KILLER"
    zzmux tabs "$WORKSPACE/$KILLER" && echo 'FAIL: killed session still addressable' || echo 'PASS: session kill workspace/session removed target'

    zzmux kill "$WORKSPACE/$WORKER" || true
    zzmux tab kill "$PEER" || true
    zzmux ls -s | grep "$RUN_ID" && echo 'FAIL: leftovers remain' || echo 'PASS: no RUN_ID leftovers'
  fi
fi
```

Notes for findings:

- Use `zzmux version` / `zmux version`; `--version` is not the documented form.
- `zzmux tab state` syntax is `zzmux tab state <state> [target] --msg ...`.
- `zzmux pane list` has `--target`, `--session`, `--all`, and `--joined`; it does
  not have `-s`. Use `zzmux pane list --session --target <workspace/session>` for logical session targeting; bare `--target <workspace/session>` is a raw tmux target.
- `zzmux kill workspace/session` is the smart cleanup path for workspace-local
  labels. Raw `zws_<workspace>__<session>` names are a diagnostic fallback, not
  the preferred prompt path.

Adjust command flags only after reading `zzmux help` / command output. If a documented command's syntax changed, report that as a finding with the exact correction.

## Coverage checklist

For each item, report `PASS`, `FAIL`, or `BLOCKED`, with evidence. A documentation-only check can pass if the docs clearly specify expected behavior and no safe live proof is appropriate.

### Routing and safety

- Bounded one-shot reads/tests stay in the normal shell when captured output is enough.
- Reviewable one-shots route to `zmux run -n scratch` or equivalent stable named tab.
- Headed/browser-visible Playwright or Chrome proof routes to one reusable scratch/proof tab, not direct bash and not one tab per lane.
- Persistent servers/watchers/queues route to `zmux run -d` / runtime tab, not `&`, `nohup`, `disown`, or hidden background jobs.
- Interactive/manual/sudo/SSH/REPL work routes to visible zmux tabs; passwords are not automated.
- TUI/terminal visual evidence routes to snapshots/visible tabs and not blind shell output.
- Raw tmux is prohibited for app-level actions; docs provide a zmux mapping.
- Destructive live-profile commands are human-gated.

### Roster and addressing

- Tiny roster is documented: `dev`, `scratch`, `<agent>-peer`, worker sessions, long-lived primary agent tabs.
- Reuse is preferred over suffix tab sprawl, and repeated test/proof batches do not mint per-lane tabs.
- Joined logical panes are treated as roster tabs and targeted by logical tab names.
- Outside-tmux and ambiguous session flows require listing sessions and passing explicit `-s`.
- Session labels, workspace/session labels, and raw `zws_...` fallback are documented.

### Run/observe/lifecycle

- `run`, `watch`, `log`, `send`, and `type` have clear examples and separation of lifecycle vs output.
- `tab status --json` is the lifecycle/command/peer state source of truth.
- `zmux wait --for output:/idle:` is documented as structured output/idle evidence, not process truth; `watch --until`/`--idle` remain human output tools. For fast markers, count already-in-tail output only when a buffer/log read proves the marker, and call it out in the report instead of retrying blindly.
- No custom sentinels, wrapper scripts, or hand-rolled poll loops are recommended.
- Reuse/restart-in-place pattern is documented.

### Tabs, panes, placement, evidence

- Logical tab concepts are documented: full tab, pane placement, hidden dock, stable label.
- `tab state`, lifecycle glyphs, `tab label`, `tab move`, `tab kill`, and placement verbs are covered or routed to CLI catalog.
- `pane open/list/focus/close/resize` is covered or routed to CLI catalog, and open/list/resize/close is live-tested on a throwaway pane when attached to `zzmux`.
- `snapshot` and `terminal current`/capability diagnostics are covered or routed to CLI catalog.
- Cleanup/reaping expectations and `--keep` / `--scope daemon` are clear; prompt-scoped ad-hoc tabs are killed after evidence capture, not left until session closeout.
- Focus-safety is explicit: use no-focus/focus-safe variants by default and ask before focus-moving operations.

### Peer and worker flows

- Peer flow uses a visible `<agent>-peer` tab, prompt-scoped lifecycle, status-first observation, and fresh generation (`turnSeq`, with `turnAt` as supporting evidence) readiness.
- Peer submission hygiene is covered: read-only contract, file pointers, no huge pasted diffs when paths suffice.
- Canonical peer-flow failure protocol is documented: stop on unproven/stale/attention instead of piling prompts.
- Worker flow uses isolated worktrees/sessions, `zmux session run` rather than attaching/spawning blank shells, and explicit lifecycle/cleanup.
- Real peer spawning is required once by this prompt when a supported CLI is available; worker spawning and `workspace/session` targeting are required. Missing CLI/auth/environment is `BLOCKED`, not `PASS`.

### Pi boundary awareness

- Skill docs tell Pi users to prefer typed `zmux_*` tools.
- Skill docs do not duplicate Pi extension internals unnecessarily.
- Shared doctrine and Pi extension docs are linked consistently.

## Final report format

Write the report to `.dump/test-prompts-report/zmux-agent-skill-testing-report-<date-or-run-id>.md` (create the directory if needed), then return a short final message with the report path and verdict. The report file must contain exactly these sections:

1. `Verdict` — one of `PASS`, `PASS WITH FINDINGS`, `FAIL`, or `BLOCKED`.
2. `Environment` — repo path, branch/commit, `zmux version` output if available, `zzmux version` output if available, whether `./dev.sh zzmux` ran, and whether the live checks included human-visible current-session checks, a real peer marker response, and detached worker/session-target E2E.
3. `CLI inventory` — top-level verbs seen in help output and any uncovered command groups.
4. `Commands run` — concise list with pass/fail/block notes.
5. `Coverage matrix` — grouped by the checklist above. Include citations to docs and live evidence when relevant.
6. `Findings` — concrete defects or drift, with file paths/commands. If the real peer marker, worker interaction, pane open/resize/close proof, placement round-trip, or `session kill workspace/session` proof is missing, report it here as `FAIL` or `BLOCKED`. Do not count a marker that appears only in the typed prompt/composer.
7. `Cleanup` — sessions/tabs created and confirmation they were removed, or what remains.
8. `Recommended follow-ups` — prioritized, bounded actions.
````
