// Package runexec owns the post-delivery mechanics of `zmux run`: writing the
// rare temp command script, blocking on the shell-lifecycle run result, proving
// startup readiness, and following live output. It is deliberately free of
// cobra and app.App — the CLI layer resolves targets, creates/reuses tabs, and
// applies policy, then hands this package a tmux.Runner plus injected writers
// and a cancellable context. That seam makes the loops (interrupt, follow
// growth/reset, dedupe) deterministically testable without a real terminal.
package runexec

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/waitfor"
)

// DefaultStartGrace is the fallback window a blocking run waits for the shell
// lifecycle hook to consume the staged nonce before declaring the pane's shell
// uninstrumented. The CLI passes an explicit StartGrace; this only applies when
// an Executor is built without one.
const DefaultStartGrace = 5 * time.Second

// Executor runs the tail phase of a `zmux run` behind injected collaborators.
// Stdout/Stderr are captured at dispatch time (the CLI passes os.Stdout/
// os.Stderr) so tests can redirect them; Runner is the only tmux boundary.
type Executor struct {
	Runner     tmux.Runner
	Stdout     io.Writer
	Stderr     io.Writer
	StartGrace time.Duration

	// tick overrides the internal poll cadence; zero uses the per-method
	// defaults (300ms result poll, 500ms follow poll). Only tests set it, to
	// exercise the poll loops without real-time waits.
	tick time.Duration
}

func (e Executor) grace() time.Duration {
	if e.StartGrace <= 0 {
		return DefaultStartGrace
	}
	return e.StartGrace
}

func (e Executor) resultTick() time.Duration {
	if e.tick > 0 {
		return e.tick
	}
	return 300 * time.Millisecond
}

func (e Executor) followTick() time.Duration {
	if e.tick > 0 {
		return e.tick
	}
	return 500 * time.Millisecond
}

// WaitResult watches pane output while waiting for the shell lifecycle hook to
// publish @zmux_run_result=<nonce>:<exit>. Completion is metadata, not terminal
// text, so stdout stays free of AGENT_DONE-style sentinels. It returns nil on a
// zero exit, an error carrying the code on a non-zero exit, and distinct
// start/result timeout errors. Cancelling ctx (Ctrl+C at the CLI) returns
// "interrupted".
func (e Executor) WaitResult(ctx context.Context, target, paneID, nonce string, timeout time.Duration, lines int) error {
	deadline := time.Now().Add(timeout)
	startWindow := clampStartDeadline(e.grace(), timeout)
	startDeadline := time.Now().Add(startWindow)
	started := false
	ticker := time.NewTicker(e.resultTick())
	defer ticker.Stop()

	seen := map[string]bool{}
	printNew := func(output string) {
		for _, line := range strings.Split(output, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if !seen[trimmed] {
				seen[trimmed] = true
				fmt.Fprintln(e.Stdout, line)
			}
		}
	}

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("interrupted")

		case <-ticker.C:
			output, err := e.Runner.CapturePane(target, lines)
			if err == nil {
				printNew(output)
			}
			if exitCode, ok := RunResultExit(e.Runner, paneID, nonce); ok {
				_ = e.Runner.UnsetPaneOption(paneID, tabs.OptRunResult)
				if exitCode != 0 {
					return fmt.Errorf("command exited with code %d", exitCode)
				}
				return nil
			}
			if !started && LifecycleStarted(e.Runner, paneID, nonce) {
				started = true
			}
			if !started && time.Now().After(startDeadline) {
				_ = e.Runner.UnsetPaneOption(paneID, tabs.OptNextRunID)
				return fmt.Errorf("timeout after %ds waiting for shell lifecycle to start in the target pane (run `zmux setup doctor`; if stale, run `zmux setup shell` and open a fresh tab, or use --detach/--follow for REPLs/TUIs/non-interactive shells)", int(startWindow.Seconds()))
			}
			if time.Now().After(deadline) {
				if err != nil {
					if output, _ := e.Runner.CapturePane(target, lines); output != "" {
						fmt.Fprint(e.Stdout, output)
					}
				}
				return fmt.Errorf("timeout after %ds waiting for shell lifecycle result (run `zmux setup doctor`; if stale, run `zmux setup shell` for the target shell, or inspect output with `zmux watch`)", int(timeout.Seconds()))
			}
		}
	}
}

// WaitReadiness proves that fresh startup output matching `until` appeared in
// the target within the timeout, ignoring pre-launch content via baseline. It
// is the --until path: the command keeps running while readiness is asserted.
func (e Executor) WaitReadiness(ctx context.Context, target, paneID, until string, timeout time.Duration, lines int, baseline map[string]int) error {
	condition, err := waitfor.ParseCondition("output:" + until)
	if err != nil {
		return err
	}
	outcome, err := waitfor.Wait(ctx, waitfor.Request{
		Runner:         e.Runner,
		Target:         target,
		PaneID:         paneID,
		Lines:          lines,
		Timeout:        timeout,
		Condition:      condition,
		OutputBaseline: baseline,
	})
	if err != nil {
		return err
	}
	if outcome.OutputTail != "" {
		fmt.Fprint(e.Stdout, outcome.OutputTail)
	}
	if !outcome.Met {
		return fmt.Errorf("runtime readiness not met: %s", outcome.FailureKind)
	}
	return nil
}

// Follow tails a tmux pane, printing newly appended lines as they appear until
// ctx is cancelled (Ctrl+C at the CLI). A shorter capture than a prior tick
// (pane cleared/redrawn) resets the high-water mark rather than reprinting.
func (e Executor) Follow(ctx context.Context, target string, lines int) error {
	lastLen := 0
	ticker := time.NewTicker(e.followTick())
	defer ticker.Stop()

	fmt.Fprintf(e.Stderr, "--- following %s (Ctrl+C to stop) ---\n", target)

	for {
		select {
		case <-ctx.Done():
			fmt.Fprintln(e.Stderr, "\n--- stopped following ---")
			return nil
		case <-ticker.C:
			output, err := e.Runner.CapturePane(target, lines)
			if err != nil {
				continue
			}

			captured := strings.Split(strings.TrimRight(output, "\n"), "\n")
			if len(captured) > lastLen {
				for _, line := range captured[lastLen:] {
					fmt.Fprintln(e.Stdout, line)
				}
				lastLen = len(captured)
			} else if len(captured) < lastLen {
				lastLen = len(captured)
			}
		}
	}
}

// clampStartDeadline returns the shorter of the start grace and the overall
// timeout: a short-timeout run must not wait longer for the shell to start than
// it would for the whole result.
func clampStartDeadline(grace, timeout time.Duration) time.Duration {
	if timeout <= 0 || timeout > grace {
		return grace
	}
	return timeout
}

// LifecycleStarted reports whether the shell lifecycle hook has consumed the
// staged nonce (the pane's @zmux_next_run_id no longer equals it), the signal
// that the command actually began running in an instrumented shell.
func LifecycleStarted(r tmux.Runner, paneID, nonce string) bool {
	next, err := r.ShowPaneOption(paneID, tabs.OptNextRunID)
	if err != nil {
		return false
	}
	return strings.TrimSpace(next) != nonce
}

// RunResultExit parses @zmux_run_result for this invocation. It matches only
// when the value is prefixed with the run's nonce, so a stale result from a
// previous command can never satisfy the current wait.
func RunResultExit(r tmux.Runner, paneID, nonce string) (int, bool) {
	value, err := r.ShowPaneOption(paneID, tabs.OptRunResult)
	if err != nil || value == "" {
		return 0, false
	}
	prefix := nonce + ":"
	if !strings.HasPrefix(strings.TrimSpace(value), prefix) {
		return 0, false
	}
	exitCode, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(value, prefix)))
	if err != nil {
		return 0, false
	}
	return exitCode, true
}

// WriteCommandScript writes a command to a temp script file for the rare cases
// where the command cannot be delivered as one prompt line (currently multiline
// or literal-tab input). Returns the script path and a fallback cleanup that
// removes the file after timeout+10s in case the script's own EXIT trap never
// runs.
func WriteCommandScript(command string, timeout time.Duration) (string, func(), error) {
	f, err := os.CreateTemp("", "zmux-cmd-*.sh")
	if err != nil {
		return "", nil, err
	}

	// Echo the command so the terminal shows what's being run, then execute it,
	// preserving the command's exit status even when the script self-deletes.
	// Use printf to avoid echo interpreting escape sequences.
	escaped := strings.ReplaceAll(command, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	script := fmt.Sprintf("#!/usr/bin/env bash\nprintf '\\033[2m$ %%s\\033[0m\\n' %q\n__zmux_cmd_cleanup() { __zmux_status=$?; rm -f %q; exit $__zmux_status; }\ntrap __zmux_cmd_cleanup EXIT\n%s\n", escaped, f.Name(), command)
	if _, err := f.WriteString(script); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", nil, err
	}
	f.Close()

	// The script self-deletes (rm -f at the end), so cleanup is a no-op.
	// But provide a fallback cleanup in case it never runs.
	cleanupDelay := timeout + 10*time.Second
	cleanup := func() {
		go func(delay time.Duration) {
			time.Sleep(delay)
			os.Remove(f.Name())
		}(cleanupDelay)
	}

	return f.Name(), cleanup, nil
}
