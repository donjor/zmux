package tabs

import (
	"fmt"
	"strings"

	"github.com/donjor/zmux/internal/tabstate"
)

// DisplaySignals are the raw lifecycle channels that feed the single human
// facing glyph state. ManualState is for explicit operator/admin state writes;
// command and turn state are structured signals from shell and agent adapters.
type DisplaySignals struct {
	ManualState        string
	ManualSource       string
	CommandState       string
	CommandSource      string
	CommandInteractive bool
	CommandExit        string
	TurnState          string
	TurnSource         string
}

// DisplayResolution is the canonical state chosen from DisplaySignals.
type DisplayResolution struct {
	Set     bool
	State   tabstate.State
	Source  string
	Message string
	Reason  string
}

// ResolveDisplayState applies lifecycle-state-v2 precedence for one pane:
// attention > failed > running > ready > done > idle. Interactive venues do not
// turn command liveness into display running/done; they need an explicit turn
// signal to animate.
func ResolveDisplayState(sig DisplaySignals) DisplayResolution {
	manual := strings.TrimSpace(sig.ManualState)
	manualSource := strings.TrimSpace(sig.ManualSource)
	cmd := strings.TrimSpace(sig.CommandState)
	turn := NormalizeTurnState(sig.TurnState)

	if manual == string(tabstate.StateAttention) || turn == TurnAttention {
		source := sourceOr(manualSource, "turn")
		if turn == TurnAttention && manual != string(tabstate.StateAttention) {
			source = sourceOr(sig.TurnSource, "turn")
		}
		return DisplayResolution{Set: true, State: tabstate.StateAttention, Source: source, Reason: "attention requires human action"}
	}
	if manual == string(tabstate.StateFailed) || turn == TurnFailed || (!sig.CommandInteractive && cmd == CmdFailed) {
		source := sourceOr(manualSource, "command")
		msg := ""
		switch {
		case turn == TurnFailed:
			source = sourceOr(sig.TurnSource, "turn")
		case !sig.CommandInteractive && cmd == CmdFailed:
			source = sourceOr(sig.CommandSource, "shell")
			if sig.CommandExit != "" {
				msg = fmt.Sprintf("exit %s", strings.TrimSpace(sig.CommandExit))
			}
		}
		return DisplayResolution{Set: true, State: tabstate.StateFailed, Source: source, Message: msg, Reason: "failed command or turn"}
	}
	if manual == string(tabstate.StateRunning) || turn == TurnRunning || (!sig.CommandInteractive && cmd == CmdRunning) {
		source := sourceOr(manualSource, "command")
		switch {
		case turn == TurnRunning:
			source = sourceOr(sig.TurnSource, "turn")
		case !sig.CommandInteractive && cmd == CmdRunning:
			source = sourceOr(sig.CommandSource, "shell")
		}
		return DisplayResolution{Set: true, State: tabstate.StateRunning, Source: source, Reason: "active command or turn"}
	}
	if manual == string(tabstate.StateReady) || turn == TurnReady {
		source := sourceOr(manualSource, "turn")
		if turn == TurnReady {
			source = sourceOr(sig.TurnSource, "turn")
		}
		return DisplayResolution{Set: true, State: tabstate.StateReady, Source: source, Reason: "turn ready for host/user"}
	}
	if manual == string(tabstate.StateDone) || (!sig.CommandInteractive && cmd == CmdDone) {
		source := sourceOr(manualSource, sourceOr(sig.CommandSource, "shell"))
		return DisplayResolution{Set: true, State: tabstate.StateDone, Source: source, Reason: "plain command completed"}
	}
	return DisplayResolution{Reason: "idle or interactive venue without active turn"}
}

func sourceOr(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}
