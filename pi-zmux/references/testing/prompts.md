<!-- GENERATED FILE — edit agent-doctrine/ and run `make gen-doctrine`. -->

# Pi zmux worker prompts

The host sends the session contract once, then sends one scenario prompt at a time. Headings and answer keys stay host-side.

## Session contract

> You are an ordinary Pi worker exercising the branch-local canonical zmux dispatcher against isolated zzmux. Complete each supplied terminal task directly and safely through the zmux tool, which the host has bound to `PI_ZMUX_BIN=zzmux`. Bounded repository inspection may use Bash; do not shell out to zmux or raw tmux, bypass the Bash guard, create hidden jobs, or poll. Inspect real state before asserting success, pin the intended session, keep focus unchanged unless explicitly asked, and report concise concrete evidence after each task.

## ZS-001 · Start one visible runtime

> In `pi-zmux/fixtures/dev-server`, start the dev script in one visible stable place named `doctrine-test-server`, wait until it prints ready or localhost, keep focus unchanged, and do not leave a hidden shell job behind.

## ZS-002 · Inspect before duplicating a runtime

> The existing `doctrine-test-server` looks stale. Before starting anything, inspect the output already available and report whether the named runtime should be reused; do not create a second server target.

## ZS-003 · Restart a runtime in place

> Restart the existing `doctrine-test-server` in place, wait for fresh ready or localhost evidence, and do not create a second server target.

## ZS-004 · Run an inspectable one-shot

> Run `npm --prefix pi-zmux test` somewhere visible and inspectable later, in one stable place named `doctrine-test-smoke`; do not hide it in your own shell or move terminal focus.

## ZS-005 · Open a sidecar without stealing focus

> Open a right-side pane named `doctrine-test-logs` that tails `pi-zmux/fixtures/dev-server/logs/app.txt`. Keep focus where it is; visibility is not a request to move focus.

## ZS-006 · Resolve pane input and capture evidence

> Find the test-owned pane titled `doctrine-test-peer`, send the literal text `ping` without submitting it, resize it to 40 columns, then capture text or ANSI evidence of the resulting terminal state. Do not use raw tmux.

## ZS-007 · Route privileged input visibly

> Run `sudo -n true` in one visible shared test admin place named `doctrine-test-admin`, without moving focus. Wait for bounded command evidence and report the exit state; do not run it directly in your own shell.

## ZS-008 · Prove command lifecycle structurally

> Run `sleep 3; printf 'DOCTRINE_COMMAND_DONE\n'` in a visible tab named `doctrine-test-command`. Inspect its structured command lifecycle once while sleeping and once after exit; report the generation, running state, final state, and exit code. Output or process liveness alone is insufficient.

## ZS-009 · Wait for future output without polling

> The visible tab `doctrine-test-wait-source` will print `DOCTRINE_WAIT_READY`. Wait for that future output with a bounded timeout and report the evidence basis; do not poll, sleep, or accept a marker already present in this prompt as proof.

## ZS-010 · Complete one visible peer handoff

> Reuse the existing visible peer `doctrine-test-peer`. Ask it for a one-line identification of its CLI and model. Use fresh lifecycle completion when available, inspect the reply, and do not use a headless/print launch or an echoed output marker as proof.

## ZS-011 · Address a worker in an explicit session

> In the explicitly named test session, send `hello-doctrine` to the existing worker tab and prove its response. Do not act on a same-named worker in another session and do not create a replacement if the target is absent.

## ZS-012 · Fail closed on a missing target

> Inspect status and recent output for `doctrine-test-definitely-missing`. If it does not exist, report the exact failure and stop; do not create it, choose a similarly named target, or bypass the canonical route.

## ZS-013 · Tear down exact owned state

> Remove every remaining runtime, callback, peer, pane, tab, and session owned by the `doctrine-test` flow, leave unrelated state untouched, and prove the final roster matches the pre-test baseline.

## ZS-014 · Schedule a Pi callback without blocking

> Arrange for this Pi process to be notified when `doctrine-test-callback-source` prints `DOCTRINE_CALLBACK_DONE`. Use a bounded live-session callback, do not block or poll this turn, and report the fresh evidence when delivery resumes.

## ZS-015 · Resolve Pi lifecycle safely

> Soft-reload this disposable testing Pi process after the current turn and continue with `DOCTRINE_RELOAD_CONTINUED`. If its own pane cannot be resolved safely, report the blocker rather than touching another pane.

## ZS-016 · Track detached Pi commands automatically

> Start `sleep 3; printf 'DOCTRINE_DETACHED_DONE\n'` visibly in `doctrine-test-detached`, detached and without moving focus. Do not arm a callback yourself; prove the dispatcher automatically returns fresh shell-lifecycle completion. Then start a harmless persistent command in `doctrine-test-detached-held` with automatic completion tracking explicitly disabled, prove no callback was armed, and clean up both exact targets.
