package waitfor

import (
	"testing"

	"github.com/donjor/zmux/internal/tabs"
)

// T-102 (055 P-001) — pure-function characterization of the waitfor freshness
// and lifecycle classifiers (freshTurn/freshCmd/freshnessFor/classifyLifecycle).
// Pins CURRENT behavior before any consolidation (S-006 acceptance): zero/
// malformed/equal/incremented generations, same-gen transitions, stale
// terminals, allow-stale, cmd done/failed/exit match+mismatch, turn
// failed/attention, PeerTurns fallback.

func TestFreshTurnMatrix(t *testing.T) {
	cases := []struct {
		name string
		base Baseline
		st   Status
		want bool
	}{
		{"zero seq is stale", Baseline{TurnSeq: 0}, Status{TurnSeq: "0", TurnState: tabs.TurnReady}, false},
		{"malformed seq is stale", Baseline{TurnSeq: 2}, Status{TurnSeq: "nope", TurnState: tabs.TurnReady}, false},
		{"incremented generation is fresh", Baseline{TurnSeq: 2, TurnState: tabs.TurnReady}, Status{TurnSeq: "3", TurnState: tabs.TurnReady}, true},
		{"same gen same state is stale", Baseline{TurnSeq: 3, TurnState: tabs.TurnReady}, Status{TurnSeq: "3", TurnState: tabs.TurnReady}, false},
		{"same gen transitioned state is fresh", Baseline{TurnSeq: 3, TurnState: tabs.TurnRunning}, Status{TurnSeq: "3", TurnState: tabs.TurnReady}, true},
		{"same gen but empty baseline state is stale", Baseline{TurnSeq: 3, TurnState: ""}, Status{TurnSeq: "3", TurnState: tabs.TurnReady}, false},
		{"peerTurns fallback when turnSeq empty", Baseline{TurnSeq: 1, TurnState: tabs.TurnRunning}, Status{TurnSeq: "", PeerTurns: "2", TurnState: tabs.TurnReady}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := freshTurn(c.base, c.st); got != c.want {
				t.Fatalf("freshTurn = %v, want %v", got, c.want)
			}
		})
	}
}

func TestFreshCmdMatrix(t *testing.T) {
	cases := []struct {
		name string
		base Baseline
		st   Status
		want string // command condition value
		out  bool
	}{
		{"zero seq is stale", Baseline{CmdSeq: 0}, Status{CmdSeq: "0", CmdState: tabs.CmdDone}, tabs.CmdDone, false},
		{"incremented seq is fresh", Baseline{CmdSeq: 4, CmdState: tabs.CmdDone}, Status{CmdSeq: "5", CmdState: tabs.CmdDone}, tabs.CmdDone, true},
		{"same gen transition matching want is fresh", Baseline{CmdSeq: 5, CmdState: tabs.CmdRunning}, Status{CmdSeq: "5", CmdState: tabs.CmdDone}, tabs.CmdDone, true},
		{"same gen transition not matching want is stale", Baseline{CmdSeq: 5, CmdState: tabs.CmdRunning}, Status{CmdSeq: "5", CmdState: tabs.CmdDone}, tabs.CmdRunning, false},
		{"same gen same state is stale", Baseline{CmdSeq: 5, CmdState: tabs.CmdDone}, Status{CmdSeq: "5", CmdState: tabs.CmdDone}, tabs.CmdDone, false},
		{"exit= match on same-gen transition is fresh", Baseline{CmdSeq: 5, CmdState: tabs.CmdRunning}, Status{CmdSeq: "5", CmdState: tabs.CmdDone, LastExit: "0"}, "exit=0", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := freshCmd(c.base, c.st, c.want); got != c.out {
				t.Fatalf("freshCmd = %v, want %v", got, c.out)
			}
		})
	}
}

func TestFreshnessForDispatch(t *testing.T) {
	// Output and idle conditions are always "fresh" (they baseline differently).
	if !freshnessFor(Condition{Kind: ConditionOutput}, Baseline{}, Status{}) {
		t.Fatal("output condition should always report fresh")
	}
	if !freshnessFor(Condition{Kind: ConditionIdle}, Baseline{}, Status{}) {
		t.Fatal("idle condition should always report fresh")
	}
	// Cmd/turn dispatch to their respective freshness helpers.
	turnFresh := freshnessFor(Condition{Kind: ConditionTurn}, Baseline{TurnSeq: 1, TurnState: tabs.TurnRunning}, Status{TurnSeq: "2", TurnState: tabs.TurnReady})
	if !turnFresh {
		t.Fatal("turn dispatch should report fresh on incremented gen")
	}
	cmdStale := freshnessFor(Condition{Kind: ConditionCmd, Value: tabs.CmdDone}, Baseline{CmdSeq: 5, CmdState: tabs.CmdDone}, Status{CmdSeq: "5", CmdState: tabs.CmdDone})
	if cmdStale {
		t.Fatal("cmd dispatch should report stale on same gen same state")
	}
}

func cmdCond(t *testing.T, value string) Condition {
	t.Helper()
	c, err := ParseCondition("cmd:" + value)
	if err != nil {
		t.Fatalf("ParseCondition cmd:%s: %v", value, err)
	}
	return c
}

func turnCond(t *testing.T, value string) Condition {
	t.Helper()
	c, err := ParseCondition("turn:" + value)
	if err != nil {
		t.Fatalf("ParseCondition turn:%s: %v", value, err)
	}
	return c
}

func TestClassifyLifecycleTurn(t *testing.T) {
	base := Baseline{TurnSeq: 1, TurnState: tabs.TurnRunning}

	t.Run("fresh match is met", func(t *testing.T) {
		oc, done := classifyLifecycle(
			Request{Condition: turnCond(t, "ready")}, base,
			Status{TurnSeq: "2", TurnState: tabs.TurnReady})
		if !done || !oc.Met || !oc.Fresh || oc.Basis != BasisTurnState {
			t.Fatalf("fresh ready match: %+v done=%v", oc, done)
		}
	})

	// A same-gen, same-state ready (no transition off the baseline) is stale.
	staleBase := Baseline{TurnSeq: 1, TurnState: tabs.TurnReady}
	staleReady := Status{TurnSeq: "1", TurnState: tabs.TurnReady}

	t.Run("stale match not done", func(t *testing.T) {
		oc, done := classifyLifecycle(
			Request{Condition: turnCond(t, "ready")}, staleBase, staleReady)
		if done || oc.Met {
			t.Fatalf("stale ready should not be done: %+v done=%v", oc, done)
		}
	})

	t.Run("allow-stale accepts stale match", func(t *testing.T) {
		oc, done := classifyLifecycle(
			Request{Condition: turnCond(t, "ready"), AllowStale: true}, staleBase, staleReady)
		if !done || !oc.Met || !oc.Fresh {
			t.Fatalf("allow-stale ready: %+v done=%v", oc, done)
		}
	})

	t.Run("fresh failed while awaiting ready terminates as failure", func(t *testing.T) {
		oc, done := classifyLifecycle(
			Request{Condition: turnCond(t, "ready")}, base,
			Status{TurnSeq: "2", TurnState: tabs.TurnFailed})
		if !done || oc.Met || oc.FailureKind != "turn_failed" {
			t.Fatalf("turn failed short-circuit: %+v done=%v", oc, done)
		}
	})

	t.Run("fresh attention while awaiting ready terminates as failure", func(t *testing.T) {
		oc, done := classifyLifecycle(
			Request{Condition: turnCond(t, "ready")}, base,
			Status{TurnSeq: "2", TurnState: tabs.TurnAttention})
		if !done || oc.Met || oc.FailureKind != "turn_attention" {
			t.Fatalf("turn attention short-circuit: %+v done=%v", oc, done)
		}
	})

	t.Run("awaiting failed matches failed as success", func(t *testing.T) {
		oc, done := classifyLifecycle(
			Request{Condition: turnCond(t, "failed")}, base,
			Status{TurnSeq: "2", TurnState: tabs.TurnFailed})
		if !done || !oc.Met || oc.FailureKind != "" {
			t.Fatalf("awaiting failed should be a met success: %+v done=%v", oc, done)
		}
	})
}

func TestClassifyLifecycleCmd(t *testing.T) {
	base := Baseline{CmdSeq: 4, CmdState: tabs.CmdRunning}

	t.Run("fresh done match is met", func(t *testing.T) {
		oc, done := classifyLifecycle(
			Request{Condition: cmdCond(t, "done")}, base,
			Status{CmdSeq: "5", CmdState: tabs.CmdDone, LastExit: "0"})
		if !done || !oc.Met || oc.Basis != BasisCmdState {
			t.Fatalf("fresh done match: %+v done=%v", oc, done)
		}
	})

	t.Run("exit match reports exit state", func(t *testing.T) {
		oc, done := classifyLifecycle(
			Request{Condition: cmdCond(t, "exit=0")}, base,
			Status{CmdSeq: "5", CmdState: tabs.CmdDone, LastExit: "0"})
		if !done || !oc.Met || oc.State != "exit=0" {
			t.Fatalf("exit=0 match: %+v done=%v", oc, done)
		}
	})

	t.Run("exit mismatch terminates as cmd_exit failure", func(t *testing.T) {
		oc, done := classifyLifecycle(
			Request{Condition: cmdCond(t, "exit=0")}, base,
			Status{CmdSeq: "5", CmdState: tabs.CmdDone, LastExit: "1"})
		if !done || oc.Met || oc.FailureKind != "cmd_exit" || oc.State != "exit=1" {
			t.Fatalf("exit mismatch: %+v done=%v", oc, done)
		}
	})

	t.Run("fresh failed while awaiting done reports cmd_exit with last exit", func(t *testing.T) {
		oc, done := classifyLifecycle(
			Request{Condition: cmdCond(t, "done")}, base,
			Status{CmdSeq: "5", CmdState: tabs.CmdFailed, LastExit: "2"})
		if !done || oc.Met || oc.FailureKind != "cmd_exit" {
			t.Fatalf("failed with exit -> cmd_exit: %+v done=%v", oc, done)
		}
	})

	t.Run("fresh failed without last exit reports cmd_failed", func(t *testing.T) {
		oc, done := classifyLifecycle(
			Request{Condition: cmdCond(t, "done")}, base,
			Status{CmdSeq: "5", CmdState: tabs.CmdFailed})
		if !done || oc.Met || oc.FailureKind != "cmd_failed" {
			t.Fatalf("failed without exit -> cmd_failed: %+v done=%v", oc, done)
		}
	})

	t.Run("stale state not done", func(t *testing.T) {
		oc, done := classifyLifecycle(
			Request{Condition: cmdCond(t, "done")}, base,
			Status{CmdSeq: "4", CmdState: tabs.CmdRunning})
		if done || oc.Met {
			t.Fatalf("still-running should not be done: %+v done=%v", oc, done)
		}
	})
}
