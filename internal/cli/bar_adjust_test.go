package cli

import (
	"testing"

	"github.com/donjor/zmux/internal/bar"
	"github.com/donjor/zmux/internal/tmux"
)

// sessionStatusSets collects the per-session "status" value(s) reconcile set
// for each session, keyed by session name.
func sessionStatusSets(calls []tmux.MockCall) map[string][]string {
	out := map[string][]string{}
	for _, c := range calls {
		if c.Method == "SetSessionOption" && len(c.Args) == 3 && c.Args[1] == "status" {
			out[c.Args[0]] = append(out[c.Args[0]], c.Args[2])
		}
	}
	return out
}

// Always-2-line contract (plan 024): every session gets `status 2` under a
// two-line layout regardless of how many sessions a workspace has — no
// count-based collapse, so the bar never reflows.
func TestReconcileBarStatusLines_TwoLineAlwaysTwo(t *testing.T) {
	mock := &tmux.MockRunner{Sessions: []tmux.Session{
		{Name: "solo"}, {Name: "dev"}, {Name: "dev-b"},
	}}

	reconcileBarStatusLines(mock, "two-line", "tabs", "zmux")

	got := sessionStatusSets(mock.Calls)
	for _, name := range []string{"solo", "dev", "dev-b"} {
		vals := got[name]
		if len(vals) != 1 || vals[0] != "2" {
			t.Errorf("session %q: want status=[2], got %v", name, vals)
		}
	}

	// format[0] must be the dynamic top bar, format[1] the logical tabs row
	// (zmux binary present → dynamic row, native list is the fallback).
	wantTop := bar.TopBarFormatCmd("zmux", "tabs")
	if !hasSessionOption(mock.Calls, "solo", "status-format[0]", wantTop) {
		t.Errorf("solo: status-format[0] not set to top bar cmd")
	}
	if !hasSessionOption(mock.Calls, "solo", "status-format[1]", bar.TabsRowStatusFormat("zmux")) {
		t.Errorf("solo: status-format[1] not set to the logical tabs row")
	}
}

// Defensive path: "single" was removed as a user layout (config normalizes it
// to two-line; plan 024), but reconcile must still degrade any non-two-line
// value to one line — clearing stale per-session `status 2` overrides rather
// than leaving a session stuck on a two-line format with no top row.
func TestReconcileBarStatusLines_NonTwoLineClearsStale(t *testing.T) {
	mock := &tmux.MockRunner{Sessions: []tmux.Session{{Name: "dev"}, {Name: "ops"}}}

	reconcileBarStatusLines(mock, "single", "tabs", "zmux")

	got := sessionStatusSets(mock.Calls)
	for _, name := range []string{"dev", "ops"} {
		vals := got[name]
		if len(vals) != 1 || vals[0] != "on" {
			t.Errorf("session %q: want status=[on], got %v", name, vals)
		}
	}
	if !hasSessionOption(mock.Calls, "dev", "status-format[0]", bar.TabsRowStatusFormat("zmux")) {
		t.Errorf("dev: status-format[0] not reset to the one-line tabs row")
	}
}

func hasSessionOption(calls []tmux.MockCall, target, key, value string) bool {
	for _, c := range calls {
		if c.Method == "SetSessionOption" && len(c.Args) == 3 &&
			c.Args[0] == target && c.Args[1] == key && c.Args[2] == value {
			return true
		}
	}
	return false
}
