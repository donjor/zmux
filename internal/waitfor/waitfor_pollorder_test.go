package waitfor

import (
	"context"
	"testing"
	"time"

	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
)

// T-103 (055 P-001) — polling-order characterization for the THREE waitfor poll
// loops (waitLifecycle / waitOutput / waitIdle). B-03 flags that the loops share
// a skeleton but differ in success-vs-deadline ordering; any pollUntil
// consolidation in T-204 must preserve exactly the orderings pinned here. All
// tests use an injected MockRunner and sub-millisecond intervals — no real tmux,
// no production-duration sleeps.
//
// Ordering contract, per loop:
//   - waitLifecycle checks success at the TOP of the loop, BEFORE any poll wait:
//     a condition already satisfied at entry returns met with ZERO ticks, and a
//     met condition is reported even if the deadline check would also fire (the
//     success check precedes the deadline check in the loop body).
//   - waitOutput / waitIdle capture a baseline BEFORE the loop but only evaluate
//     success INSIDE the ticker.C case, so they always wait at least one poll
//     interval before the first success return; within a tick, success is
//     checked before the deadline.

func TestLifecycleChecksSuccessBeforeAnyWait(t *testing.T) {
	// Fresh ready at entry with a poll interval far larger than the timeout:
	// only a top-of-loop pre-wait success check can return met here.
	mock := tmux.NewMockRunner()
	mock.PaneOptions = map[string]string{
		"%1\x00" + tabs.OptTurnState: tabs.TurnReady,
		"%1\x00" + tabs.OptTurnSeq:   "2",
	}
	cond, _ := ParseCondition("turn:ready")
	start := time.Now()
	out, err := Wait(context.Background(), Request{
		Runner:       mock,
		Target:       "%1",
		PaneID:       "%1",
		Condition:    cond,
		Baseline:     &Baseline{TurnSeq: 1, TurnState: tabs.TurnRunning},
		Timeout:      time.Hour,
		PollInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if !out.Met || out.Basis != BasisTurnState {
		t.Fatalf("met condition at entry must return without waiting: %+v", out)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("lifecycle waited %v; pre-wait success check not honored", elapsed)
	}
}

func TestLifecycleDeadlineGovernsWhenNeverMet(t *testing.T) {
	// Still-running, stale baseline -> never met; the deadline check (which
	// follows the success check in the loop body) governs the exit.
	mock := tmux.NewMockRunner()
	mock.PaneOptions = map[string]string{
		"%1\x00" + tabs.OptTurnState: tabs.TurnRunning,
		"%1\x00" + tabs.OptTurnSeq:   "1",
	}
	cond, _ := ParseCondition("turn:ready")
	out, err := Wait(context.Background(), Request{
		Runner:       mock,
		Target:       "%1",
		PaneID:       "%1",
		Condition:    cond,
		Baseline:     &Baseline{TurnSeq: 1, TurnState: tabs.TurnRunning},
		Timeout:      10 * time.Millisecond,
		PollInterval: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if out.Met || out.FailureKind != "turn_unproven" {
		t.Fatalf("unmet lifecycle should time out as turn_unproven: %+v", out)
	}
}

func TestLifecycleContextCancellationReportsInterrupted(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.PaneOptions = map[string]string{
		"%1\x00" + tabs.OptTurnState: tabs.TurnRunning,
		"%1\x00" + tabs.OptTurnSeq:   "1",
	}
	cond, _ := ParseCondition("turn:ready")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out, err := Wait(ctx, Request{
		Runner:       mock,
		Target:       "%1",
		PaneID:       "%1",
		Condition:    cond,
		Baseline:     &Baseline{TurnSeq: 1, TurnState: tabs.TurnRunning},
		Timeout:      time.Second,
		PollInterval: time.Millisecond,
	})
	if err == nil {
		t.Fatalf("cancelled wait should return ctx error, got nil (out=%+v)", out)
	}
	if out.FailureKind != "interrupted" {
		t.Fatalf("cancelled lifecycle should report interrupted: %+v", out)
	}
}

func TestOutputSuccessCheckedBeforeDeadline(t *testing.T) {
	// A fresh READY appears; success is evaluated before the deadline check
	// inside the tick, so a fresh match returns met rather than timing out.
	mock := tmux.NewMockRunner()
	captures := []string{"boot\n", "boot\n", "boot\nREADY\n"}
	mock.CapturePaneFunc = func(_ string, _ int) (string, error) {
		if len(captures) == 1 {
			return captures[0], nil
		}
		out := captures[0]
		captures = captures[1:]
		return out, nil
	}
	cond, _ := ParseCondition("output:READY")
	out, err := Wait(context.Background(), Request{
		Runner:       mock,
		Target:       "%1",
		PaneID:       "%1",
		Condition:    cond,
		Timeout:      50 * time.Millisecond,
		PollInterval: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if !out.Met || out.Basis != BasisOutput {
		t.Fatalf("fresh output match should win over deadline: %+v", out)
	}
}

func TestOutputDeadlineWhenNoFreshMatch(t *testing.T) {
	// Constant non-matching output -> never a fresh match; deadline governs.
	mock := tmux.NewMockRunner()
	mock.CapturedPaneContent = "still booting\n"
	cond, _ := ParseCondition("output:READY")
	out, err := Wait(context.Background(), Request{
		Runner:       mock,
		Target:       "%1",
		PaneID:       "%1",
		Condition:    cond,
		Timeout:      10 * time.Millisecond,
		PollInterval: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if out.Met || out.FailureKind != "output_regex_unproven" || out.AlreadyInTail {
		t.Fatalf("no fresh match should time out unproven: %+v", out)
	}
}

func TestOutputCaptureErrorSurfacesAtDeadline(t *testing.T) {
	// Capture errors are swallowed until the deadline, then surfaced as
	// capture_failed with the error appended as a warning.
	mock := tmux.NewMockRunner()
	mock.CapturePaneFunc = func(_ string, _ int) (string, error) {
		return "", context.DeadlineExceeded
	}
	cond, _ := ParseCondition("output:READY")
	out, err := Wait(context.Background(), Request{
		Runner:       mock,
		Target:       "%1",
		PaneID:       "%1",
		Condition:    cond,
		Timeout:      10 * time.Millisecond,
		PollInterval: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if out.Met || out.FailureKind != "capture_failed" {
		t.Fatalf("persistent capture error should time out as capture_failed: %+v", out)
	}
	if len(out.Warnings) == 0 {
		t.Fatalf("capture_failed should carry the capture error as a warning: %+v", out)
	}
}

func TestIdleSuccessBeforeDeadline(t *testing.T) {
	// Stable output reaches the idle threshold before the timeout -> met.
	mock := tmux.NewMockRunner()
	mock.CapturedPaneContent = "quiet\n"
	cond, _ := ParseCondition("idle:1ms")
	out, err := Wait(context.Background(), Request{
		Runner:       mock,
		Target:       "%1",
		PaneID:       "%1",
		Condition:    cond,
		Timeout:      50 * time.Millisecond,
		PollInterval: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if !out.Met || out.Basis != BasisIdle {
		t.Fatalf("stable pane should reach idle before deadline: %+v", out)
	}
}

func TestIdleDeadlineWhenNeverStable(t *testing.T) {
	// Output that changes on every capture never stabilizes -> idle_unproven.
	mock := tmux.NewMockRunner()
	n := 0
	mock.CapturePaneFunc = func(_ string, _ int) (string, error) {
		n++
		return "line-" + time.Duration(n).String() + "\n", nil
	}
	cond, _ := ParseCondition("idle:20ms")
	out, err := Wait(context.Background(), Request{
		Runner:       mock,
		Target:       "%1",
		PaneID:       "%1",
		Condition:    cond,
		Timeout:      10 * time.Millisecond,
		PollInterval: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if out.Met || out.FailureKind != "idle_unproven" {
		t.Fatalf("never-stable pane should time out idle_unproven: %+v", out)
	}
}
