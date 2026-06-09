package qa

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"time"
	"unicode/utf8"
)

// DefaultTimeout bounds a step command when the step doesn't set its own.
const DefaultTimeout = 60 * time.Second

// EvidenceCap bounds the persisted output tail per step.
const EvidenceCap = 4 * 1024

// ExecResult is the raw outcome of running a step command.
type ExecResult struct {
	Exit     int
	Output   string // combined stdout+stderr
	Duration time.Duration
	TimedOut bool
}

// CmdRunner executes step commands per the exec contract: `sh -c`, cwd =
// repo root, env passthrough, combined output, bounded by timeout. A
// nonzero exit is a result, not an error — err means the command could
// not run at all (no shell, bad dir).
type CmdRunner interface {
	Run(command, dir string, timeout time.Duration) (ExecResult, error)
}

// ShellRunner is the real CmdRunner.
type ShellRunner struct{}

func (ShellRunner) Run(command, dir string, timeout time.Duration) (ExecResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = dir
	// Without a WaitDelay, an orphaned grandchild holding the output pipe
	// keeps CombinedOutput blocked long after the timeout kill.
	cmd.WaitDelay = time.Second
	start := time.Now()
	out, err := cmd.CombinedOutput()

	res := ExecResult{
		Output:   string(out),
		Duration: time.Since(start),
		TimedOut: errors.Is(ctx.Err(), context.DeadlineExceeded),
	}
	if err != nil {
		var exitErr *exec.ExitError
		switch {
		case errors.As(err, &exitErr):
			res.Exit = exitErr.ExitCode() // -1 when killed (timeout)
			return res, nil
		case errors.Is(err, exec.ErrWaitDelay):
			return res, nil // exited fine; orphans held the pipe past the grace
		}
		return res, err
	}
	return res, nil
}

// ExecuteStep runs one step's command and derives the scorecard record
// per the result contract:
//
//	automatic     check match && exit 0 → pass; otherwise → fail
//	human-judged  exit 0 → pending (await verdict); exit ≠ 0 → error
//	timeout / unstartable → error for both — the harness gave up, which
//	is not a verdict on the expectation
//
// By attribution is the caller's (CLI = agent, picker = human).
func ExecuteStep(r CmdRunner, cl *Checklist, s *Step, repoRoot string) StepRecord {
	command, err := cl.Command(s)
	if err != nil {
		return StepRecord{Result: ResultError, Note: err.Error()}
	}

	timeout := DefaultTimeout
	if s.Timeout > 0 {
		timeout = time.Duration(s.Timeout) * time.Second
	}

	res, err := r.Run(command, repoRoot, timeout)
	ev := &Evidence{
		Command:    command,
		Exit:       res.Exit,
		DurationMS: res.Duration.Milliseconds(),
		Output:     tail(res.Output, EvidenceCap),
	}
	switch {
	case res.TimedOut: // before err — a kill-induced wait error is still a timeout
		return StepRecord{Result: ResultError, Note: fmt.Sprintf("timed out after %s", timeout), Evidence: ev}
	case err != nil:
		return StepRecord{Result: ResultError, Note: "command could not run: " + err.Error(), Evidence: ev}
	}

	if s.Automatic() {
		re, reErr := regexp.Compile(s.Check)
		if reErr != nil { // lint catches this up front; guards direct callers
			return StepRecord{Result: ResultError, Note: "invalid check: " + reErr.Error(), Evidence: ev}
		}
		ev.Matched = re.MatchString(res.Output)
		if ev.Matched && res.Exit == 0 {
			return StepRecord{Result: ResultPass, Evidence: ev}
		}
		return StepRecord{Result: ResultFail, Evidence: ev}
	}

	// Human-judged: a clean run awaits a verdict; a broken command is an
	// error — never forced into pass/fail.
	if res.Exit != 0 {
		return StepRecord{Result: ResultError, Note: "command failed — needs a clean run before a verdict", Evidence: ev}
	}
	return StepRecord{Result: ResultPending, Evidence: ev}
}

// tail keeps the last n bytes of s, trimming to a rune boundary and
// marking the cut.
func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := s[len(s)-n:]
	for len(cut) > 0 && !utf8.RuneStart(cut[0]) {
		cut = cut[1:]
	}
	return "…" + cut
}
