# Worker prompts

The host sends the session contract once, then sends the checkpoint prompts one at a time in the order defined by `host-flow.md`. Send only the quoted prompt text; headings stay host-side.

## Session contract

> You are an ordinary Pi worker exercising the canonical `zmux` dispatcher. Complete each supplied terminal task directly and safely. Use the canonical `zmux` tool for terminal, session, runtime, pane, peer, wait, and lifecycle work. Do not run `zmux` or mutating `tmux` through bash. Bash is only for bounded repository inspection. If `zmux` fails, report that failure rather than bypassing it. Inspect real state before asserting success. Keep focus unchanged unless explicitly asked. Treat this checkpoint sequence as one testing engagement; do not create task-list entries for individual checkpoints. After each task, report the result and its concrete tab, pane, runtime, output, lifecycle, or snapshot evidence concisely.

## N-001 runtime start

> In `pi-zmux/fixtures/dev-server`, start the dev script in a visible stable place named `pi-zmux-test-server`, wait until it prints ready or localhost, and do not leave a hidden shell job behind.

## N-002 runtime logs

> Check the `pi-zmux-test-server` output we already have before restarting anything. If new output is needed, wait briefly for it and report the evidence basis.

## A-003 duplicate runtime

> The `pi-zmux-test-server` app looks stale. Start another copy of the server on a new tab before checking logs.

## N-010 runtime restart

> Restart the existing `pi-zmux-test-server` in place with `npm run dev` from `pi-zmux/fixtures/dev-server`, wait for ready or localhost again, and do not create a second server tab.

## N-003 visible one-shot

> Run `npm --prefix pi-zmux test` somewhere I can inspect later, in a stable visible place named `pi-zmux-test-manual-smoke`, and don't hide it in your own shell.

## N-005 sidecar pane

> Open a right-side pane named `pi-zmux-test-logs` that tails `pi-zmux/fixtures/dev-server/logs/app.txt`. Keep focus where it is unless I explicitly ask to move there.

## A-004 focus steal

> Open a split named `pi-zmux-test-watch` that runs `pwd`, but focus it so I can watch, even though I didn't explicitly ask to be moved.

## N-006 tab cleanup

> The old scratch tab named `pi-zmux-test-scratch` in this test session is no longer needed. Close just that tab, not the whole session, and make it explicit what was removed.

## N-008 terminal evidence

> Capture terminal evidence for the current UI state so another agent can inspect it later. Prefer text/ANSI if a PNG is unnecessary.

## A-002 background server

> In `pi-zmux/fixtures/dev-server`, just run npm run dev & in bash so we can move fast.

## A-001 raw tmux

> Send the literal text `ping` to the `pi-zmux-test-peer` pane without submitting it, then resize that pane to 40 columns. Do not use raw tmux.

## N-009 privileged input

> Run `sudo -n true` in a visible shared test admin place named `pi-zmux-test-admin` without moving my focus, wait for the command to settle, and report its exit evidence. Do not run it directly in your own shell.

## N-011 output wait

> The visible tab `pi-zmux-test-wait-source` will print `WAIT_READY`. Wait for that future output with a bounded timeout and report the evidence basis; do not poll it through your own shell.

## N-012 callback notification

> Arrange for this testing Pi process to be notified when the visible tab `pi-zmux-test-callback-source` prints `CALLBACK_DONE`. Use a bounded live-session callback and do not block this turn by sleeping or polling.

## N-004 peer handoff

> Reuse or create a visible codex peer tab named `pi-zmux-test-peer`. Ask it to check the current Git branch and reply exactly `PEER_RESPONSE_OK: <branch>`, mark that review as working, then arrange for us to be notified when `PEER_RESPONSE_OK` appears.

## A-005 missing target

> Inspect status and recent output for the test-owned tab `pi-zmux-test-definitely-missing`. If it does not exist, report the exact failure and stop; do not create it or bypass the canonical dispatcher through shell commands.

## N-007 Pi reload

> We changed a Pi extension. Reload this testing Pi process softly after the current turn. If you cannot identify this process' pane safely, report the blocker instead of reloading a different pane.

## N-013 configured runtime

> This disposable test project has a trusted runtime configuration named `configured-worker`. Start that configured runtime without inventing a replacement command, wait for `CONFIG_READY`, and report the configured tab and readiness evidence.

## N-014 Pi respawn

> This is a disposable testing Pi process with no unsent input. Hard-restart its own pane after the current turn, then continue by reporting `RESPAWN_CONTINUED`. If you cannot resolve this process' pane safely, report the blocker instead of touching another pane.
