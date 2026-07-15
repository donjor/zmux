package tabstate

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/donjor/zmux/internal/tmux"
)

func newTestService(mock *tmux.MockRunner, env map[string]string) *Service {
	s := New(mock, func(key string) string { return env[key] })
	s.now = func() time.Time { return time.Unix(1750000000, 0) }
	return s
}

func callsOf(mock *tmux.MockRunner, method string) []tmux.MockCall {
	var out []tmux.MockCall
	for _, c := range mock.Calls {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

// batchWrites filters the recorded ApplyOptions writes (one record per write)
// by scope and unset flag.
func batchWrites(mock *tmux.MockRunner, scope tmux.OptionScope, unset bool) []tmux.MockCall {
	var out []tmux.MockCall
	for _, c := range callsOf(mock, "ApplyOptions") {
		if c.Args[0] == string(scope) && c.Args[4] == "unset="+map[bool]string{true: "true", false: "false"}[unset] {
			out = append(out, c)
		}
	}
	return out
}

func TestSetWritesPaneCanonicalAndWindowMirror(t *testing.T) {
	mock := tmux.NewMockRunner()
	s := newTestService(mock, nil)
	tgt := Target{PaneID: "%3", Window: "dev:2"}

	if err := s.Set(tgt, StateRunning, "run", ""); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	pane := batchWrites(mock, tmux.ScopePane, false)
	if len(pane) != 3 { // state, source, at — msg empty → unset
		t.Fatalf("want 3 pane writes, got %d: %v", len(pane), pane)
	}
	if pane[0].Args[1] != "%3" || pane[0].Args[2] != OptState || pane[0].Args[3] != "running" {
		t.Fatalf("canonical state write wrong: %v", pane[0].Args)
	}
	win := batchWrites(mock, tmux.ScopeWindow, false)
	if len(win) != 3 {
		t.Fatalf("want 3 mirror writes, got %d", len(win))
	}
	if win[0].Args[1] != "dev:2" || win[0].Args[2] != OptState || win[0].Args[3] != "running" {
		t.Fatalf("mirror state write wrong: %v", win[0].Args)
	}
	if got := batchWrites(mock, tmux.ScopePane, true); len(got) != 1 || got[0].Args[2] != OptMsg {
		t.Fatalf("empty msg should unset %s on pane, got %v", OptMsg, got)
	}
	if got := callsOf(mock, "RefreshStatus"); len(got) != 1 {
		t.Fatalf("Set must refresh status once, got %d", len(got))
	}
}

// TestSetSanitizesTabsFromMirrorValues is the invariant guard for the
// ShowPaneOptions TAB field separator: no free-text mirror value (source, msg)
// may reach a pane option carrying a TAB, or a later ShowPaneOptions batch read
// would split one value into two and misalign every field after it.
func TestSetSanitizesTabsFromMirrorValues(t *testing.T) {
	mock := tmux.NewMockRunner()
	s := newTestService(mock, nil)
	tgt := Target{PaneID: "%3", Window: "dev:2"}

	if err := s.Set(tgt, StateFailed, "run\tinjected", "exit 2\tsmuggled\ttail"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	for _, c := range callsOf(mock, "ApplyOptions") {
		if strings.Contains(c.Args[3], "\t") {
			t.Fatalf("mirror write for %s must not contain a TAB, got %q", c.Args[2], c.Args[3])
		}
	}

	pane := batchWrites(mock, tmux.ScopePane, false)
	got := map[string]string{}
	for _, c := range pane {
		got[c.Args[2]] = c.Args[3]
	}
	if got[OptSource] != "run injected" {
		t.Fatalf("source not collapsed: %q", got[OptSource])
	}
	if got[OptMsg] != "exit 2 smuggled tail" {
		t.Fatalf("msg not collapsed: %q", got[OptMsg])
	}
}

func TestSetSurvivesRefreshStatusError(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.RefreshStatusErr = errors.New("no current client")
	s := newTestService(mock, nil)

	if err := s.Set(Target{PaneID: "%1", Window: "a:0"}, StateDone, "run", ""); err != nil {
		t.Fatalf("Set must treat RefreshStatus as best-effort, got %v", err)
	}
}

func TestClearUnsetsAllOptionsBothScopes(t *testing.T) {
	mock := tmux.NewMockRunner()
	s := newTestService(mock, nil)

	if err := s.Clear(Target{PaneID: "%5", Window: "dev:1"}); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}
	if got := batchWrites(mock, tmux.ScopePane, true); len(got) != 4 {
		t.Fatalf("want 4 pane unsets, got %d", len(got))
	}
	if got := batchWrites(mock, tmux.ScopeWindow, true); len(got) != 4 {
		t.Fatalf("want 4 window unsets, got %d", len(got))
	}
}

func TestClearIfOnlyMatchingState(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.DisplayMessageResult = "attention\n"
	s := newTestService(mock, nil)
	tgt := Target{PaneID: "%2", Window: "dev:0"}

	cleared, err := s.ClearIf(tgt, StateAttention)
	if err != nil || !cleared {
		t.Fatalf("want clear of matching attention, got cleared=%v err=%v", cleared, err)
	}

	mock2 := tmux.NewMockRunner()
	mock2.DisplayMessageResult = "done\n"
	s2 := newTestService(mock2, nil)
	cleared, err = s2.ClearIf(tgt, StateAttention)
	if err != nil || cleared {
		t.Fatalf("done must NOT be cleared by focus rule, got cleared=%v err=%v", cleared, err)
	}
	if got := callsOf(mock2, "UnsetPaneOption"); len(got) != 0 {
		t.Fatalf("no unsets expected when state does not match, got %v", got)
	}
}

func TestSetRunningDropsStatusInterval(t *testing.T) {
	mock := tmux.NewMockRunner()
	s := newTestService(mock, nil)

	if err := s.Set(Target{PaneID: "%1", Window: "a:0"}, StateRunning, "run", ""); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	got := callsOf(mock, "SetOption")
	if len(got) != 1 || got[0].Args[1] != "status-interval" || got[0].Args[2] != intervalActive {
		t.Fatalf("running must drop status-interval to %s, got %v", intervalActive, got)
	}
	if probes := callsOf(mock, "ListPaneOptionValues"); len(probes) != 0 {
		t.Fatalf("running set needs no pane probe, got %v", probes)
	}
}

func TestSetDoneRestoresIntervalWhenNothingRuns(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.PaneOptionValues = map[string][]string{OptState: {"done", "", "attention"}}
	s := newTestService(mock, nil)

	if err := s.Set(Target{PaneID: "%1", Window: "a:0"}, StateDone, "run", ""); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	got := callsOf(mock, "SetOption")
	if len(got) != 1 || got[0].Args[1] != "status-interval" || got[0].Args[2] != intervalIdle {
		t.Fatalf("want idle interval restore, got %v", got)
	}
}

func TestSetDoneKeepsFastIntervalWhileAnotherRuns(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.PaneOptionValues = map[string][]string{OptState: {"done", "running"}}
	s := newTestService(mock, nil)

	if err := s.Set(Target{PaneID: "%1", Window: "a:0"}, StateDone, "run", ""); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if got := callsOf(mock, "SetOption"); len(got) != 0 {
		t.Fatalf("another running pane must keep the fast tick, got %v", got)
	}
}

func TestClearRestoresIntervalWhenNothingRuns(t *testing.T) {
	mock := tmux.NewMockRunner()
	s := newTestService(mock, nil)

	if err := s.Clear(Target{PaneID: "%5", Window: "dev:1"}); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}
	got := callsOf(mock, "SetOption")
	if len(got) != 1 || got[0].Args[2] != intervalIdle {
		t.Fatalf("clear with no running panes must restore idle interval, got %v", got)
	}
}

func TestVisibleWindowMembership(t *testing.T) {
	cases := []struct {
		display string
		want    bool
	}{
		{"1\t1\n", true},  // current window, attached
		{"1\t2\n", true},  // attached count >1
		{"0\t1\n", false}, // background window
		{"1\t0\n", false}, // detached session
	}
	for _, c := range cases {
		mock := tmux.NewMockRunner()
		mock.DisplayMessageResult = c.display
		s := newTestService(mock, nil)
		got, err := s.Visible(Target{PaneID: "%1", Window: "a:0"})
		if err != nil || got != c.want {
			t.Fatalf("Visible(%q) = %v, %v; want %v", c.display, got, err, c.want)
		}
	}
}

func TestSetDoneByVisibility(t *testing.T) {
	// visible → done
	mock := tmux.NewMockRunner()
	mock.DisplayMessageResult = "1\t1\n"
	s := newTestService(mock, nil)
	st, err := s.SetDoneByVisibility(Target{PaneID: "%1", Window: "a:0"}, "claude-stop", "")
	if err != nil || st != StateDone {
		t.Fatalf("visible pane: want done, got %v err=%v", st, err)
	}

	// hidden → attention
	mock2 := tmux.NewMockRunner()
	mock2.DisplayMessageResult = "0\t1\n"
	s2 := newTestService(mock2, nil)
	st, err = s2.SetDoneByVisibility(Target{PaneID: "%1", Window: "a:0"}, "claude-stop", "")
	if err != nil || st != StateAttention {
		t.Fatalf("hidden pane: want attention, got %v err=%v", st, err)
	}

	// indeterminate (display errors) → done, write still happens
	mock3 := tmux.NewMockRunner()
	mock3.Err = errors.New("boom")
	s3 := newTestService(mock3, nil)
	st, err = s3.SetDoneByVisibility(Target{PaneID: "%1", Window: "a:0"}, "claude-stop", "")
	if st != StateDone {
		t.Fatalf("indeterminate visibility must prefer done, got %v", st)
	}
	if err == nil {
		t.Fatalf("Set with mock.Err must propagate the write error")
	}
}
