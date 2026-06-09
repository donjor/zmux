// Package tabstate stores tab lifecycle states (attention/running/done/
// failed) on tmux pane options — the canonical home, surviving join-pane and
// break-pane — mirrored to window options for bar/window-status rendering
// while the tab is full-window (P1: always; multi-pane aggregation is P3).
package tabstate

import "fmt"

// State is a tab lifecycle state.
type State string

const (
	StateAttention State = "attention" // needs the human (permission prompt, sudo handoff)
	StateRunning   State = "running"   // command/agent turn in flight
	StateDone      State = "done"      // finished cleanly, not yet acknowledged
	StateFailed    State = "failed"    // finished with an error
)

// Option names. Pane-scoped writes are canonical; the same names at window
// scope are the presentation mirror the bar reads.
const (
	OptState  = "@zmux_state"
	OptSource = "@zmux_state_source"
	OptAt     = "@zmux_state_at"
	OptMsg    = "@zmux_state_msg" // display-only: format expansion escapes $ (spike A) — never parse back
)

// All enumerates valid states in aggregation priority order (highest urgency
// first). P3 multi-pane mirrors aggregate by this order; P1 only uses it for
// validation and format generation.
var All = []State{StateAttention, StateFailed, StateRunning, StateDone}

// Parse validates a raw state string.
func Parse(raw string) (State, error) {
	for _, st := range All {
		if string(st) == raw {
			return st, nil
		}
	}
	return "", fmt.Errorf("unknown tab state %q (want attention|running|done|failed)", raw)
}
