package tabs

import (
	"testing"

	"github.com/donjor/zmux/internal/tabstate"
)

func TestResolveDisplayStatePrecedence(t *testing.T) {
	cases := []struct {
		name string
		sig  DisplaySignals
		want tabstate.State
	}{
		{"attention beats failed", DisplaySignals{ManualState: "attention", CommandState: CmdFailed, TurnState: TurnRunning}, tabstate.StateAttention},
		{"turn failed beats running command", DisplaySignals{TurnState: TurnFailed, CommandState: CmdRunning}, tabstate.StateFailed},
		{"turn running beats command done", DisplaySignals{TurnState: TurnRunning, CommandState: CmdDone}, tabstate.StateRunning},
		{"command running beats ready", DisplaySignals{CommandState: CmdRunning, TurnState: TurnReady}, tabstate.StateRunning},
		{"ready beats command done", DisplaySignals{TurnState: TurnReady, CommandState: CmdDone}, tabstate.StateReady},
		{"plain command done", DisplaySignals{CommandState: CmdDone}, tabstate.StateDone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveDisplayState(tc.sig)
			if !got.Set || got.State != tc.want {
				t.Fatalf("ResolveDisplayState(%+v) = set=%v state=%q reason=%q, want %q", tc.sig, got.Set, got.State, got.Reason, tc.want)
			}
		})
	}
}

func TestResolveDisplayStateInteractiveVenueSuppressesCommandLiveness(t *testing.T) {
	got := ResolveDisplayState(DisplaySignals{CommandState: CmdRunning, CommandInteractive: true})
	if got.Set {
		t.Fatalf("interactive command running must resolve idle, got %+v", got)
	}

	got = ResolveDisplayState(DisplaySignals{CommandState: CmdRunning, CommandInteractive: true, TurnState: TurnRunning})
	if !got.Set || got.State != tabstate.StateRunning || got.Source != "turn" {
		t.Fatalf("explicit turn must still animate venue, got %+v", got)
	}
}

func TestResolveDisplayStateWaitingAliasesReady(t *testing.T) {
	got := ResolveDisplayState(DisplaySignals{TurnState: TurnWaiting})
	if !got.Set || got.State != tabstate.StateReady {
		t.Fatalf("waiting alias should resolve ready, got %+v", got)
	}
}
