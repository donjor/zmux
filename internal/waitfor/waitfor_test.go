package waitfor

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
)

func TestParseConditionTurnSet(t *testing.T) {
	cond, err := ParseCondition("turn:ready, failed ,attention")
	if err != nil {
		t.Fatalf("parse set: %v", err)
	}
	if cond.Kind != ConditionTurn || cond.Value != "ready,failed,attention" {
		t.Fatalf("unexpected condition: %+v", cond)
	}
}

func TestParseConditionTurnSingleByteIdentical(t *testing.T) {
	cond, err := ParseCondition("turn:ready")
	if err != nil {
		t.Fatalf("parse single: %v", err)
	}
	if cond.Value != "ready" {
		t.Fatalf("single-state value drifted: %q", cond.Value)
	}
}

func TestParseConditionTurnRejectsBadMember(t *testing.T) {
	_, err := ParseCondition("turn:ready,bogus")
	if err == nil || !strings.Contains(err.Error(), "bogus") {
		t.Fatalf("expected error naming bogus, got: %v", err)
	}
}

func TestTurnWaitSetMetByFailedReportsFired(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.PaneOptions = map[string]string{
		"%1\x00" + tabs.OptTurnState: tabs.TurnFailed,
		"%1\x00" + tabs.OptTurnSeq:   "4",
	}
	cond, err := ParseCondition("turn:ready,failed")
	if err != nil {
		t.Fatal(err)
	}
	out, err := Wait(context.Background(), Request{
		Runner:    mock,
		Target:    "%1",
		PaneID:    "%1",
		Condition: cond,
		Baseline:  &Baseline{TurnSeq: 4, TurnState: tabs.TurnRunning},
		Timeout:   20 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if !out.Met || out.State != tabs.TurnFailed || out.FailureKind != "" {
		t.Fatalf("fresh failed should satisfy ready,failed and report failed: %+v", out)
	}
}

func TestTurnWaitSetRejectsStale(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.PaneOptions = map[string]string{
		"%1\x00" + tabs.OptTurnState: tabs.TurnFailed,
		"%1\x00" + tabs.OptTurnSeq:   "3",
	}
	cond, err := ParseCondition("turn:ready,failed")
	if err != nil {
		t.Fatal(err)
	}
	out, err := Wait(context.Background(), Request{
		Runner:       mock,
		Target:       "%1",
		PaneID:       "%1",
		Condition:    cond,
		Baseline:     &Baseline{TurnSeq: 3, TurnState: tabs.TurnFailed},
		Timeout:      5 * time.Millisecond,
		PollInterval: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if out.Met || out.Fresh || out.FailureKind != "turn_unproven" {
		t.Fatalf("stale set member should be unproven: %+v", out)
	}
}

func TestTurnWaitRejectsStaleReady(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.PaneOptions = map[string]string{
		"%1\x00" + tabs.OptTurnState: tabs.TurnReady,
		"%1\x00" + tabs.OptTurnSeq:   "3",
	}
	cond, err := ParseCondition("turn:ready")
	if err != nil {
		t.Fatal(err)
	}
	out, err := Wait(context.Background(), Request{
		Runner:       mock,
		Target:       "%1",
		PaneID:       "%1",
		Condition:    cond,
		Baseline:     &Baseline{TurnSeq: 3, TurnState: tabs.TurnReady},
		Timeout:      5 * time.Millisecond,
		PollInterval: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if out.Met || out.Fresh || out.FailureKind != "turn_unproven" {
		t.Fatalf("stale ready should be unproven, got %+v", out)
	}
}

func TestTurnWaitAcceptsReadyAfterRunningSameGeneration(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.PaneOptions = map[string]string{
		"%1\x00" + tabs.OptTurnState: tabs.TurnReady,
		"%1\x00" + tabs.OptTurnSeq:   "4",
	}
	cond, _ := ParseCondition("turn:ready")
	out, err := Wait(context.Background(), Request{
		Runner:    mock,
		Target:    "%1",
		PaneID:    "%1",
		Condition: cond,
		Baseline:  &Baseline{TurnSeq: 4, TurnState: tabs.TurnRunning},
		Timeout:   20 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if !out.Met || !out.Fresh || out.Basis != BasisTurnState {
		t.Fatalf("fresh same-generation ready not accepted: %+v", out)
	}
}

func TestCommandWaitAcceptsDoneAfterRunningSameSequence(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.PaneOptions = map[string]string{
		"%1\x00" + tabs.OptCmdState:    tabs.CmdDone,
		"%1\x00" + tabs.OptCmdSeq:      "7",
		"%1\x00" + tabs.OptCmdLastExit: "0",
	}
	cond, _ := ParseCondition("cmd:done")
	out, err := Wait(context.Background(), Request{
		Runner:    mock,
		Target:    "%1",
		PaneID:    "%1",
		Condition: cond,
		Baseline:  &Baseline{CmdSeq: 7, CmdState: tabs.CmdRunning},
		Timeout:   20 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if !out.Met || !out.Fresh || out.Status.LastExit != "0" {
		t.Fatalf("fresh done not accepted: %+v", out)
	}
}

func TestOutputWaitUsesNewOutputBaseline(t *testing.T) {
	mock := tmux.NewMockRunner()
	captures := []string{"old READY\n", "old READY\n", "old READY\nnew READY\n"}
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
	if !out.Met || out.Basis != BasisOutput || out.OutputTail == "old READY\n" {
		t.Fatalf("output baseline failed: %+v", out)
	}
}

func TestOutputWaitReportsAlreadyPresent(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.CapturedPaneContent = "old READY\n"
	cond, _ := ParseCondition("output:READY")
	out, err := Wait(context.Background(), Request{
		Runner:       mock,
		Target:       "%1",
		PaneID:       "%1",
		Condition:    cond,
		Timeout:      5 * time.Millisecond,
		PollInterval: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if out.Met || out.FailureKind != "output_regex_already_present" || !out.AlreadyInTail || out.Fresh {
		t.Fatalf("already-present output not distinguished: %+v", out)
	}
	if !strings.Contains(out.OutputTail, "old READY") {
		t.Fatalf("output tail missing already-present evidence: %+v", out)
	}
}

func TestOutputWaitAcceptsRepeatedMatchingLineAfterBaseline(t *testing.T) {
	mock := tmux.NewMockRunner()
	captures := []string{"READY\n", "READY\n", "READY\nREADY\n"}
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
	if !out.Met || !out.Fresh || out.AlreadyInTail {
		t.Fatalf("repeated fresh matching line not accepted: %+v", out)
	}
}

func TestOutputWaitTreatsScrolledBaselineOccurrenceConservatively(t *testing.T) {
	// L3 (detox 054 audit): the pre-existing READY sits in the baseline window.
	// It then scrolls out as a genuinely fresh READY arrives, but the bounded
	// capture window still shows exactly one READY, so the fresh occurrence's
	// count never exceeds the stale baseline count. Freshness is occurrence-count
	// based within the window, so this indistinguishable case is classified
	// conservatively as pre-existing (not met) — a false-negative timeout is
	// acceptable; a false-positive readiness is not. This pins that boundary.
	mock := tmux.NewMockRunner()
	captures := []string{"READY\n", "READY\n", "READY\n"}
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
		Timeout:      10 * time.Millisecond,
		PollInterval: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if out.Met || out.Fresh {
		t.Fatalf("scrolled-out baseline occurrence must not be classified fresh readiness: %+v", out)
	}
}

func TestIdleWaitReturnsStableTail(t *testing.T) {
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
	if !out.Met || out.Basis != BasisIdle || out.OutputTail != "quiet\n" {
		t.Fatalf("idle wait failed: %+v", out)
	}
}
