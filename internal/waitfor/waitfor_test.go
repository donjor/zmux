package waitfor

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
)

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
